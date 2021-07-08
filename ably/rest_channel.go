package ably

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ugorji/go/codec"
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

// RESTChannel is the interface for REST API operations on a channel.
type RESTChannel struct {
	Name     string
	Presence *RESTPresence

	client  *REST
	baseURL string
	options *protoChannelOptions
}

func newRESTChannel(name string, client *REST) *RESTChannel {
	c := &RESTChannel{
		Name:    name,
		client:  client,
		baseURL: "/channels/" + encodeURIComponent.Replace(name),
	}
	c.Presence = &RESTPresence{
		client:  client,
		channel: c,
	}
	return c
}

// Publish publishes a message on the channel.
func (c *RESTChannel) Publish(ctx context.Context, name string, data interface{}) error {
	return c.PublishMultiple(ctx, []*Message{
		{Name: name, Data: data},
	})
}

// PublishMultiple publishes multiple messages in a batch.
func (c *RESTChannel) PublishMultiple(ctx context.Context, messages []*Message) error {
	return c.PublishMultipleWithOptions(ctx, messages)
}

// PublishMultipleOption is an optional parameter for
// RESTChannel.PublishMultipleWithOptions.
type PublishMultipleOption func(*publishMultipleOptions)

type publishMultipleOptions struct {
	params map[string]string
}

// Params adds query parameters to the resulting HTTP request to the REST API.
func PublishMultipleWithParams(params map[string]string) PublishMultipleOption {
	return func(options *publishMultipleOptions) {
		options.params = params
	}
}

// PublishMultipleWithOptions is PublishMultiple with optional parameters.
func (c *RESTChannel) PublishMultipleWithOptions(ctx context.Context, messages []*Message, options ...PublishMultipleOption) error {
	var publishOpts publishMultipleOptions
	for _, o := range options {
		o(&publishOpts)
	}
	for i, m := range messages {
		cipher, _ := c.options.GetCipher()
		var err error
		*m, err = (*m).withEncodedData(cipher)
		if err != nil {
			return fmt.Errorf("encoding data for message #%d: %w", i, err)
		}
	}
	useIdempotent := c.client.opts.idempotentRESTPublishing()
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
	if params := publishOpts.params; len(params) > 0 {
		queryParams := url.Values{}
		for k, v := range params {
			queryParams.Set(k, v)
		}
		query = "?" + queryParams.Encode()
	}
	res, err := c.client.post(ctx, c.baseURL+"/messages"+query, messages, nil)
	if err != nil {
		return err
	}
	return res.Body.Close()
}

// History gives the channel's message history.
func (c *RESTChannel) History(o ...HistoryOption) HistoryRequest {
	params := (&historyOptions{}).apply(o...)
	return HistoryRequest{
		r:       c.client.newPaginatedRequest("/channels/"+c.Name+"/history", params),
		channel: c,
	}
}

// A HistoryOption configures a call to RESTChannel.History or RealtimeChannel.History.
type HistoryOption func(*historyOptions)

func HistoryWithStart(t time.Time) HistoryOption {
	return func(o *historyOptions) {
		o.params.Set("start", strconv.FormatInt(unixMilli(t), 10))
	}
}

func HistoryWithEnd(t time.Time) HistoryOption {
	return func(o *historyOptions) {
		o.params.Set("end", strconv.FormatInt(unixMilli(t), 10))
	}
}

func HistoryWithLimit(limit int) HistoryOption {
	return func(o *historyOptions) {
		o.params.Set("limit", strconv.Itoa(limit))
	}
}

func HistoryWithDirection(d Direction) HistoryOption {
	return func(o *historyOptions) {
		o.params.Set("direction", string(d))
	}
}

type historyOptions struct {
	params url.Values
}

func (o *historyOptions) apply(opts ...HistoryOption) url.Values {
	o.params = make(url.Values)
	for _, opt := range opts {
		opt(o)
	}
	return o.params
}

// HistoryRequest represents a request prepared by the RESTChannel.History or
// RealtimeChannel.History method, ready to be performed by its Pages or Items methods.
type HistoryRequest struct {
	r       paginatedRequest
	channel *RESTChannel
}

// Pages returns an iterator for whole pages of History.
//
// See "Paginated results" section in the package-level documentation.
func (r HistoryRequest) Pages(ctx context.Context) (*MessagesPaginatedResult, error) {
	var res MessagesPaginatedResult
	return &res, res.load(ctx, r.r)
}

// A MessagesPaginatedResult is an iterator for the result of a History request.
//
// See "Paginated results" section in the package-level documentation.
type MessagesPaginatedResult struct {
	PaginatedResult
	items []*Message
}

// Next retrieves the next page of results.
//
// See the "Paginated results" section in the package-level documentation.
func (p *MessagesPaginatedResult) Next(ctx context.Context) bool {
	p.items = nil // avoid mutating already returned items
	return p.next(ctx, &p.items)
}

// Items returns the current page of results.
//
// See the "Paginated results" section in the package-level documentation.
func (p *MessagesPaginatedResult) Items() []*Message {
	return p.items
}

// Items returns a convenience iterator for single History, over an underlying
// paginated iterator.
//
// See "Paginated results" section in the package-level documentation.
func (r HistoryRequest) Items(ctx context.Context) (*MessagesPaginatedItems, error) {
	var res MessagesPaginatedItems
	var err error
	res.next, err = res.loadItems(ctx, r.r, func() (interface{}, func() int) {
		res.items = nil // avoid mutating already returned Items
		return r.channel.fullMessagesDecoder(&res.items), func() int {
			return len(res.items)
		}
	})
	return &res, err
}

// fullMessagesDecoder wraps a destination slice of messages in a decoder value
// that decodes both the message itself from the transport-level encoding and
// the data field within from its message-specific encoding.
func (c *RESTChannel) fullMessagesDecoder(dst *[]*Message) interface{} {
	return &fullMessagesDecoder{dst: dst, c: c}
}

type fullMessagesDecoder struct {
	dst *[]*Message
	c   *RESTChannel
}

func (t *fullMessagesDecoder) UnmarshalJSON(b []byte) error {
	err := json.Unmarshal(b, &t.dst)
	if err != nil {
		return err
	}
	t.decodeMessagesData()
	return nil
}

func (t *fullMessagesDecoder) CodecEncodeSelf(*codec.Encoder) {
	panic("messagesDecoderForChannel cannot be used as encoder")
}

func (t *fullMessagesDecoder) CodecDecodeSelf(decoder *codec.Decoder) {
	decoder.MustDecode(&t.dst)
	t.decodeMessagesData()
}

var _ interface {
	json.Unmarshaler
	codec.Selfer
} = (*fullMessagesDecoder)(nil)

func (t *fullMessagesDecoder) decodeMessagesData() {
	cipher, _ := t.c.options.GetCipher()
	for _, m := range *t.dst {
		var err error
		*m, err = m.withDecodedData(cipher)
		if err != nil {
			// RSL6b
			t.c.log().Errorf("Couldn't fully decode message data from channel %q: %w", t.c.Name, err)
		}
	}
}

type MessagesPaginatedItems struct {
	PaginatedResult
	items []*Message
	item  *Message
	next  func(context.Context) (int, bool)
}

// Next retrieves the next result.
//
// See the "Paginated results" section in the package-level documentation.
func (p *MessagesPaginatedItems) Next(ctx context.Context) bool {
	i, ok := p.next(ctx)
	if !ok {
		return false
	}
	p.item = p.items[i]
	return true
}

// Item returns the current result.
//
// See the "Paginated results" section in the package-level documentation.
func (p *MessagesPaginatedItems) Item() *Message {
	return p.item
}

func (c *RESTChannel) log() logger {
	return c.client.log
}
