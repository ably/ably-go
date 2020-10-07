package ably

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ably/ably-go/ably/internal/ablyutil"

	"github.com/ably/ably-go/ably/proto"
)

// based on HttpUtils::encodeURIComponent from ably-java library
var encodeURIComponent = strings.NewReplacer(
	" ", "%20",
	"!", "%21",
	"'", "%27",
	"(", "%28",
	")", "%29",
	"+", "%2B",
	":", "%3A",
	"~", "%7E",
	"/", "%2F",
	"?", "%3F",
	"#", "%23",
)

type RestChannel struct {
	Name     string
	Presence *RestPresence

	client  *REST
	baseURL string
	options *proto.ChannelOptions
}

func newRestChannel(name string, client *REST) *RestChannel {
	c := &RestChannel{
		Name:    name,
		client:  client,
		baseURL: "/channels/" + encodeURIComponent.Replace(name),
	}
	c.Presence = &RestPresence{
		client:  client,
		channel: c,
	}
	return c
}

// Publish publishes a message on the channel.
func (c *RestChannel) Publish(ctx context.Context, name string, data interface{}) error {
	return c.PublishAll(ctx, []Message{
		{Name: name, Data: data},
	}, nil)
}

// Message is what Ably channels send and receive.
type Message = proto.Message

// PublishAll publishes multiple messages in a batch.
//
// The params, if any, will be set as additional query parameters in the
// resulting HTTP request to the REST API.
func (c *RestChannel) PublishAll(ctx context.Context, messages []Message, params map[string]string) error {
	// TODO: Use context
	msgPtrs := make([]*proto.Message, 0, len(messages))
	for _, m := range messages {
		msgPtrs = append(msgPtrs, (*proto.Message)(&m))
	}
	if c.options != nil {
		for _, v := range messages {
			v.ChannelOptions = c.options
		}
	}
	useIdempotent := c.client.opts.idempotentRestPublishing()
	if useIdempotent {
		switch len(messages) {
		case 1:
			// spec RSL1k2 we preserve the id if we have one message and it contains the
			// id.
			if messages[0].ID == "" {
				base, err := ablyutil.BaseID()
				if err != nil {
					return err
				}
				messages[0].ID = fmt.Sprintf("%s:%d", base, 0)
			}
		default:
			empty := true
			for _, v := range messages {
				if v.ID != "" {
					empty = false
				}
			}
			if empty { // spec RSL1k3,RSL1k1
				base, err := ablyutil.BaseID()
				if err != nil {
					return err
				}
				for k, v := range messages {
					v.ID = fmt.Sprintf("%s:%d", base, k)
				}
			}
		}
	}
	var query string
	if len(params) > 0 {
		queryParams := url.Values{}
		for k, v := range params {
			queryParams.Set(k, v)
		}
		query = "?" + queryParams.Encode()
	}
	res, err := c.client.post(c.baseURL+"/messages"+query, messages, nil)
	if err != nil {
		return err
	}
	return res.Body.Close()
}

// History gives the channel's message history according to the given parameters.
// The returned result can be inspected for the messages via the Messages()
// method.
func (c *RestChannel) History(params *PaginateParams) (*PaginatedResult, error) {
	path := c.baseURL + "/history"
	rst, err := newPaginatedResult(c.options, paginatedRequest{typ: msgType, path: path, params: params, query: query(c.client.get), logger: c.logger(), respCheck: checkValidHTTPResponse})
	if err != nil {
		return nil, err
	}
	return rst, nil
}

func (c *RestChannel) logger() *LoggerOptions {
	return c.client.logger()
}
