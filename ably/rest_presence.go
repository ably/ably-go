package ably

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/ably/ably-go/ably/proto"
)

type RESTPresence struct {
	client  *REST
	channel *RESTChannel
}

// Get gives the channel's presence messages according to the given parameters.
// The returned result can be inspected for the presence messages via
// the PresenceMessages() method.
func (p *RESTPresence) Get(ctx context.Context, params *PaginateParams) (*PaginatedResult, error) {
	path := p.channel.baseURL + "/presence"
	return newPaginatedResult(ctx, nil, paginatedRequest{typ: presMsgType, path: path, params: params, query: query(p.client.get), logger: p.logger(), respCheck: checkValidHTTPResponse})
}

func (p *RESTPresence) logger() *LoggerOptions {
	return p.client.logger()
}

// History gives the channel's presence history.
func (c *RESTPresence) History(o ...PresenceHistoryOption) PresenceHistoryRequest {
	params := (&presenceHistoryOptions{}).apply(o...)
	return PresenceHistoryRequest{
		r:              c.client.newPaginatedRequest("/channels/"+c.channel.Name+"/presence/history", params),
		channelOptions: c.channel.options,
	}
}

// A HistoryOption configures a call to RESTChannel.History or RealtimeChannel.History.
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

// PresenceHistoryRequest represents a request prepared by the RESTPresence.History or
// RealtimePresence.History method, ready to be performed by its Pages or Items methods.
type PresenceHistoryRequest struct {
	r              paginatedRequestNew
	channelOptions *proto.ChannelOptions
}

// Pages returns an iterator for whole pages of presence messages.
//
// See "Paginated results" section in the package-level documentation.
func (r PresenceHistoryRequest) Pages(ctx context.Context) (*PresencePaginatedResult, error) {
	var res PresencePaginatedResult
	return &res, res.load(ctx, r.r)
}

// A PresencePaginatedResult is an iterator for the result of a PresenceHistory request.
//
// See "Paginated results" section in the package-level documentation.
type PresencePaginatedResult struct {
	PaginatedResultNew
	items []*PresenceMessage
}

// Next retrieves the next page of results.
//
// See the "Paginated results" section in the package-level documentation.
func (p *PresencePaginatedResult) Next(ctx context.Context) bool {
	p.items = nil // avoid mutating already returned items
	return p.next(ctx, &p.items)
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
func (r PresenceHistoryRequest) Items(ctx context.Context) (*PresencePaginatedItems, error) {
	var res PresencePaginatedItems
	var err error
	res.next, err = res.loadItems(ctx, r.r, func() (interface{}, func() int) {
		res.items = nil // avoid mutating already returned Items
		var dst interface{} = &res.items
		if r.channelOptions != nil {
			// TODO
		}
		return dst, func() int {
			return len(res.items)
		}
	})
	return &res, err
}

type PresencePaginatedItems struct {
	PaginatedResultNew
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
