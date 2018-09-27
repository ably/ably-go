package ably

import (
	"strings"

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

	client  *RestClient
	baseURL string
	options *proto.ChannelOptions
}

func newRestChannel(name string, client *RestClient) *RestChannel {
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

func (c *RestChannel) Publish(name string, data interface{}) error {
	encoding := proto.ValueEncoding(c.client.opts.protocol(), data)
	messages := []*proto.Message{
		{Name: name, Data: data, Encoding: encoding},
	}
	return c.PublishAll(messages)
}

// PublishAll sends multiple messages in the same http call.
// This is the more efficient way of transmitting a batch of messages
// using the Rest API.
func (c *RestChannel) PublishAll(messages []*proto.Message) error {
	for _, v := range messages {
		e := v.Encoding
		if c.options != nil {
			a, err := c.options.GetCipher()
			if err != nil {
				return err
			}
			if e != "" {
				e += "/"
			}
			e += a.GetAlgorithm() + "/" + proto.Base64
		}
		err := v.EncodeData(e, c.options)
		if err != nil {
			return err
		}
		if v.Data != nil {
			// Before the data field is marshalled, we need to make sure it is encoded in
			// the right format before going over the wire.
			encoding := proto.ValueEncoding(c.client.opts.protocol(), v.Data)
			if encoding != "" && encoding != e {
				stash := v.Encoding
				// We don't pass options here because encryption must have already taken
				// place by now.
				err = v.EncodeData(encoding, nil)
				if err != nil {
					return err
				}
				v.Encoding = stash + "/" + encoding
			}
		}

	}
	res, err := c.client.post(c.baseURL+"/messages", messages, nil)
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
	rst, err := newPaginatedResult(msgType, path, params, query(c.client.get), c.logger())
	if err != nil {
		return nil, err
	}
	if c.options != nil {
		if v, ok := rst.typItems.([]*proto.Message); ok {
			for _, msg := range v {
				err := msg.DecodeData(c.options)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return rst, nil
}

func (c *RestChannel) logger() *LoggerOptions {
	return c.client.logger()
}
