package ably

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/ugorji/go/codec"
)

// RESTPresence enables the retrieval of the current and historic presence set for a channel.
type RESTPresence struct {
	client  *REST
	channel *RESTChannel
}

// Get retrieves the current members present on the channel and the metadata for each member, such as
// their [ably.PresenceAction] and ID. Returns a [ably.PaginatedResult] object,
// containing an array of [ably.PresenceMessage]objects (RSPa).
//
// See package-level documentation => [ably] Pagination for more details.
func (c *RESTPresence) Get(o ...GetPresenceOption) PresenceRequest {
	params := (&getPresenceOptions{}).apply(o...)
	return PresenceRequest{
		r:       c.client.newPaginatedRequest("/channels/"+c.channel.Name+"/presence", "/channels/"+c.channel.pathName()+"/presence", params),
		channel: c.channel,
	}
}

// GetPresenceOption configures a call to RESTPresence.Get or RealtimePresence.Get.
type GetPresenceOption func(*getPresenceOptions)

// GetPresenceWithLimit sets an upper limit on the number of messages returned.
// The default is 100, and the maximum is 1000 (RSP3a).
func GetPresenceWithLimit(limit int) GetPresenceOption {
	return func(o *getPresenceOptions) {
		o.params.Set("limit", strconv.Itoa(limit))
	}
}

// GetPresenceWithClientID filters the list of returned presence members by a specific client using its ID (RSP3a2).
func GetPresenceWithClientID(clientID string) GetPresenceOption {
	return func(o *getPresenceOptions) {
		o.params.Set("clientId", clientID)
	}
}

// GetPresenceWithConnectionID filters the list of returned presence members by a specific connection using its ID.
// RSP3a3
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

// History retrieves a [ably.PaginatedResult] object, containing an array of historical [ably.PresenceMessage]
// objects for the channel. If the channel is configured to persist messages, then presence messages can be retrieved
// from history for up to 72 hours in the past. If not, presence messages can only be retrieved from history for up
// to two minutes in the past (RSP4a).
//
// See package-level documentation => [ably] Pagination for details about history pagination.
func (c *RESTPresence) History(o ...PresenceHistoryOption) PresenceRequest {
	params := (&presenceHistoryOptions{}).apply(o...)
	return PresenceRequest{
		r:       c.client.newPaginatedRequest("/channels/"+c.channel.Name+"/presence/history", "/channels/"+c.channel.pathName()+"/presence/history", params),
		channel: c.channel,
	}
}

// PresenceHistoryOption configures a call to RESTChannel.History or RealtimeChannel.History.
type PresenceHistoryOption func(*presenceHistoryOptions)

// PresenceHistoryWithStart sets the time from which messages are retrieved, specified as milliseconds
// since the Unix epoch (RSP4b1).
func PresenceHistoryWithStart(t time.Time) PresenceHistoryOption {
	return func(o *presenceHistoryOptions) {
		o.params.Set("start", strconv.FormatInt(unixMilli(t), 10))
	}
}

// PresenceHistoryWithEnd sets the time until messages are retrieved, specified as milliseconds since the Unix epoch.
// (RSP4b1)
func PresenceHistoryWithEnd(t time.Time) PresenceHistoryOption {
	return func(o *presenceHistoryOptions) {
		o.params.Set("end", strconv.FormatInt(unixMilli(t), 10))
	}
}

// PresenceHistoryWithLimit sets an upper limit on the number of messages returned.
// The default is 100, and the maximum is 1000 (RSP4b3).
func PresenceHistoryWithLimit(limit int) PresenceHistoryOption {
	return func(o *presenceHistoryOptions) {
		o.params.Set("limit", strconv.Itoa(limit))
	}
}

// PresenceHistoryWithDirection sets the order for which messages are returned in.
// Valid values are backwards which orders messages from most recent to oldest,
// or forwards which orders messages from oldest to most recent.
// The default is backwards (RSP4b2).
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
// See package-level documentation => [ably] Pagination for more details.
func (r PresenceRequest) Pages(ctx context.Context) (*PresencePaginatedResult, error) {
	res := PresencePaginatedResult{decoder: r.channel.fullPresenceDecoder}
	return &res, res.load(ctx, r.r)
}

// A PresencePaginatedResult is an iterator for the result of a PresenceHistory request.
//
// See package-level documentation => [ably] Pagination for more details.
type PresencePaginatedResult struct {
	PaginatedResult
	items   []*PresenceMessage
	decoder func(*[]*PresenceMessage) interface{}
}

// Next retrieves the next page of results.
//
// See package-level documentation => [ably] Pagination for more details.
func (p *PresencePaginatedResult) Next(ctx context.Context) bool {
	p.items = nil // avoid mutating already returned items
	return p.next(ctx, p.decoder(&p.items))
}

// IsLast returns true if the page is last page.
//
// See package-level documentation => [ably] Pagination for more details.
func (p *PresencePaginatedResult) IsLast(ctx context.Context) bool {
	return !p.HasNext(ctx)
}

func (p *PresencePaginatedResult) HasNext(ctx context.Context) bool {
	return p.nextLink != ""
}

// Items returns the current page of results.
//
// See package-level documentation => [ably] Pagination for more details.
func (p *PresencePaginatedResult) Items() []*PresenceMessage {
	return p.items
}

// Items returns a convenience iterator for single PresenceHistory, over an underlying
// paginated iterator.
//
// See package-level documentation => [ably] Pagination for more details.
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
// See package-level documentation => [ably] Pagination for more details.
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
// See package-level documentation => [ably] Pagination for more details.
func (p *PresencePaginatedItems) Item() *PresenceMessage {
	return p.item
}
