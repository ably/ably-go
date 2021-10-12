package ably

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/ugorji/go/codec"
)

type RESTPresence struct {
	client  *REST
	channel *RESTChannel
}

func (c *RESTPresence) Get(o ...GetPresenceOption) PresenceRequest {
	params := (&getPresenceOptions{}).apply(o...)
	return PresenceRequest{
		r:       c.client.newPaginatedRequest("/channels/"+c.channel.Name+"/presence", params),
		channel: c.channel,
	}
}

// A GetPresenceOption configures a call to RESTPresence.Get or RealtimePresence.Get.
type GetPresenceOption func(*getPresenceOptions)

func GetPresenceWithLimit(limit int) GetPresenceOption {
	return func(o *getPresenceOptions) {
		o.params.Set("limit", strconv.Itoa(limit))
	}
}

func GetPresenceWithClientID(clientID string) GetPresenceOption {
	return func(o *getPresenceOptions) {
		o.params.Set("clientId", clientID)
	}
}

func GetPresenceWithConnectionID(connectionID string) GetPresenceOption {
	return func(o *getPresenceOptions) {
		o.params.Set("connectionId", connectionID)
	}
}

type getPresenceOptions struct {
	params url.Values
}

func (o *getPresenceOptions) apply(opts ...GetPresenceOption) url.Values {
	o.params = make(url.Values)
	for _, opt := range opts {
		opt(o)
	}
	return o.params
}

func (p *RESTPresence) log() logger {
	return p.client.log
}

// History gives the channel's presence history.
func (c *RESTPresence) History(o ...PresenceHistoryOption) PresenceRequest {
	params := (&presenceHistoryOptions{}).apply(o...)
	return PresenceRequest{
		r:       c.client.newPaginatedRequest("/channels/"+c.channel.Name+"/presence/history", params),
		channel: c.channel,
	}
}

// A PresenceHistoryOption configures a call to RESTChannel.History or RealtimeChannel.History.
type PresenceHistoryOption func(*presenceHistoryOptions)

func PresenceHistoryWithStart(t time.Time) PresenceHistoryOption {
	return func(o *presenceHistoryOptions) {
		o.params.Set("start", strconv.FormatInt(unixMilli(t), 10))
	}
}

func PresenceHistoryWithEnd(t time.Time) PresenceHistoryOption {
	return func(o *presenceHistoryOptions) {
		o.params.Set("end", strconv.FormatInt(unixMilli(t), 10))
	}
}

func PresenceHistoryWithLimit(limit int) PresenceHistoryOption {
	return func(o *presenceHistoryOptions) {
		o.params.Set("limit", strconv.Itoa(limit))
	}
}

func PresenceHistoryWithDirection(d Direction) PresenceHistoryOption {
	return func(o *presenceHistoryOptions) {
		o.params.Set("direction", string(d))
	}
}

type presenceHistoryOptions struct {
	params url.Values
}

func (o *presenceHistoryOptions) apply(opts ...PresenceHistoryOption) url.Values {
	o.params = make(url.Values)
	for _, opt := range opts {
		opt(o)
	}
	return o.params
}

// PresenceRequest represents a request prepared by the RESTPresence.History or
// RealtimePresence.History method, ready to be performed by its Pages or Items methods.
type PresenceRequest struct {
	r       paginatedRequest
	channel *RESTChannel
}

// Pages returns an iterator for whole pages of presence messages.
//
// See "Paginated results" section in the package-level documentation.
func (r PresenceRequest) Pages(ctx context.Context) (*PresencePaginatedResult, error) {
	res := PresencePaginatedResult{decoder: r.channel.fullPresenceDecoder}
	return &res, res.load(ctx, r.r)
}

// A PresencePaginatedResult is an iterator for the result of a PresenceHistory request.
//
// See "Paginated results" section in the package-level documentation.
type PresencePaginatedResult struct {
	PaginatedResult
	items   []*PresenceMessage
	decoder func(*[]*PresenceMessage) interface{}
}

// Next retrieves the next page of results.
//
// See the "Paginated results" section in the package-level documentation.
func (p *PresencePaginatedResult) Next(ctx context.Context) bool {
	p.items = nil // avoid mutating already returned items
	return p.next(ctx, p.decoder(&p.items))
}

// Items returns the current page of results.
//
// See the "Paginated results" section in the package-level documentation.
func (p *PresencePaginatedResult) Items() []*PresenceMessage {
	return p.items
}

// Items returns a convenience iterator for single PresenceHistory, over an underlying
// paginated iterator.
//
// See "Paginated results" section in the package-level documentation.
func (r PresenceRequest) Items(ctx context.Context) (*PresencePaginatedItems, error) {
	var res PresencePaginatedItems
	var err error
	res.next, err = res.loadItems(ctx, r.r, func() (interface{}, func() int) {
		return r.channel.fullPresenceDecoder(&res.items), func() int {
			return len(res.items)
		}
	})
	return &res, err
}

// fullPresenceDecoder wraps a destination slice of messages in a decoder value
// that decodes both the message itself from the transport-level encoding and
// the data field within from its message-specific encoding.
func (c *RESTChannel) fullPresenceDecoder(dst *[]*PresenceMessage) interface{} {
	return &fullPresenceDecoder{dst: dst, c: c}
}

type fullPresenceDecoder struct {
	dst *[]*PresenceMessage
	c   *RESTChannel
}

func (t *fullPresenceDecoder) UnmarshalJSON(b []byte) error {
	err := json.Unmarshal(b, &t.dst)
	if err != nil {
		return err
	}
	t.decodeMessagesData()
	return nil
}

func (t *fullPresenceDecoder) CodecEncodeSelf(*codec.Encoder) {
	panic("presenceDecoderForChannel cannot be used as encoder")
}

func (t *fullPresenceDecoder) CodecDecodeSelf(decoder *codec.Decoder) {
	decoder.MustDecode(&t.dst)
	t.decodeMessagesData()
}

var _ interface {
	json.Unmarshaler
	codec.Selfer
} = (*fullPresenceDecoder)(nil)

func (t *fullPresenceDecoder) decodeMessagesData() {
	cipher, _ := t.c.options.GetCipher()
	for _, m := range *t.dst {
		var err error
		m.Message, err = m.Message.withDecodedData(cipher)
		if err != nil {
			// RSL6b
			t.c.log().Errorf("Couldn't fully decode presence message data from channel %q: %w", t.c.Name, err)
		}
	}
}

type PresencePaginatedItems struct {
	PaginatedResult
	items []*PresenceMessage
	item  *PresenceMessage
	next  func(context.Context) (int, bool)
}

// Next retrieves the next result.
//
// See the "Paginated results" section in the package-level documentation.
func (p *PresencePaginatedItems) Next(ctx context.Context) bool {
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
func (p *PresencePaginatedItems) Item() *PresenceMessage {
	return p.item
}
