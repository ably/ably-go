package ably

import (
	"bytes"
	"context"
	_ "crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ugorji/go/codec"
)

var (
	msgType     = reflect.TypeOf((*[]*Message)(nil)).Elem()
	presMsgType = reflect.TypeOf((*[]*PresenceMessage)(nil)).Elem()
	arrayTyp    = reflect.TypeOf((*[]interface{})(nil)).Elem()
)

func query(fn func(context.Context, string, interface{}) (*http.Response, error)) queryFunc {
	return func(ctx context.Context, path string) (*http.Response, error) {
		return fn(ctx, path, nil)
	}
}

// raw "decodes" both json and message pack keeping the original bytes.
// This is used to delay the actual decoding until later.
type raw []byte

func (r *raw) UnmarshalJSON(data []byte) error {
	*r = data
	return nil
}

func (r *raw) CodecEncodeSelf(*codec.Encoder) {
	panic("Raw cannot be used as encoder")
}

func (r *raw) CodecDecodeSelf(decoder *codec.Decoder) {
	var raw codec.Raw
	decoder.MustDecode(&raw)
	*r = []byte(raw)
}

// RESTChannels provides an API for managing collection of RESTChannel.
// This is safe for concurrent use.
type RESTChannels struct {
	chans  map[string]*RESTChannel
	mu     sync.RWMutex
	client *REST
}

// Iterate returns a list of created channels.
//
// It is safe to call Iterate from multiple goroutines, however there's no guarantee
// the returned list would not list a channel that was already released from
// different goroutine.
func (c *RESTChannels) Iterate() []*RESTChannel { // RSN2, RTS2
	c.mu.Lock()
	chans := make([]*RESTChannel, 0, len(c.chans))
	for _, restChannel := range c.chans {
		chans = append(chans, restChannel)
	}
	c.mu.Unlock()
	return chans
}

// Exists returns true if the channel by the given name exists.
func (c *RESTChannels) Exists(name string) bool { // RSN2, RTS2
	c.mu.RLock()
	_, ok := c.chans[name]
	c.mu.RUnlock()
	return ok
}

// Get returns an existing channel or creates a new one if it doesn't exist.
//
// You can optionally pass ChannelOptions, if the channel exists it will
// updated with the options and when it doesn't a new channel will be created
// with the given options.
func (c *RESTChannels) Get(name string, options ...ChannelOption) *RESTChannel {
	var o channelOptions
	for _, set := range options {
		set(&o)
	}
	return c.get(name, (*protoChannelOptions)(&o))
}

func (c *RESTChannels) get(name string, opts *protoChannelOptions) *RESTChannel {
	c.mu.RLock()
	v, ok := c.chans[name]
	c.mu.RUnlock()
	if ok {
		if opts != nil {
			v.options = opts
		}
		return v
	}
	v = newRESTChannel(name, c.client)
	v.options = opts
	c.mu.Lock()
	c.chans[name] = v
	c.mu.Unlock()
	return v
}

// Release deletes the channel from the chans.
func (c *RESTChannels) Release(name string) {
	c.mu.Lock()
	delete(c.chans, name)
	c.mu.Unlock()
}

// REST is rest client that offers a simple stateless API to interact directly with Ably's REST API.
type REST struct {
	// Auth is a  [ably.Auth] object (RSC5).
	Auth *Auth

	//Channels is a [ably.RESTChannels] object (RSN1).
	Channels *RESTChannels

	opts               *clientOptions
	hostCache          *hostCache
	activeRealtimeHost string // RTN17e
	log                logger
}

// NewREST construct a RestClient object using an [ably.ClientOption] object to configure
// the client connection to Ably (RSC1).
func NewREST(options ...ClientOption) (*REST, error) {
	c := &REST{
		opts: applyOptionsWithDefaults(options...),
	}
	if err := c.opts.validate(); err != nil {
		return nil, err
	}
	c.log = logger{l: c.opts.LogHandler}
	auth, err := newAuth(c)
	if err != nil {
		return nil, err
	}
	c.Auth = auth
	c.Channels = &RESTChannels{
		chans:  make(map[string]*RESTChannel),
		client: c,
	}
	c.hostCache = &hostCache{
		duration: c.opts.fallbackRetryTimeout(),
	}
	return c, nil
}

// Time retrieves the time from the Ably service as milliseconds since the Unix epoch.
// Clients that do not have access to a sufficiently well maintained time source and wish to issue Ably
// multiple [ably.TokenRequest] with a more accurate timestamp should use the ClientOptions.UseQueryTime
// property instead of this method (RSC16).
func (c *REST) Time(ctx context.Context) (time.Time, error) {
	var times []int64
	r := &request{
		Method: "GET",
		Path:   "/time",
		Out:    &times,
		NoAuth: true,
	}
	_, err := c.do(ctx, r)
	if err != nil {
		return time.Time{}, err
	}
	if len(times) != 1 {
		return time.Time{}, newErrorf(ErrInternalError, "expected 1 timestamp, got %d", len(times))
	}
	return time.Unix(times[0]/1000, times[0]%1000), nil
}

// Stats queries the REST/stats API and retrieves your application's usage statistics. Returns a
// [ably.PaginatedResult] object, containing an array of [Stats]{@link Stats} objects (RSC6a).
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (c *REST) Stats(o ...StatsOption) StatsRequest {
	params := (&statsOptions{}).apply(o...)
	return StatsRequest{r: c.newPaginatedRequest("/stats", "", params)}
}

func (c *REST) setActiveRealtimeHost(realtimeHost string) {
	c.activeRealtimeHost = realtimeHost
}

// A StatsOption configures a call to REST.Stats or Realtime.Stats.
type StatsOption func(*statsOptions)

// StatsWithStart sets the time from which stats are retrieved, specified as milliseconds since the Unix epoch (RSC6b1).
func StatsWithStart(t time.Time) StatsOption {
	return func(o *statsOptions) {
		o.params.Set("start", strconv.FormatInt(unixMilli(t), 10))
	}
}

// StatsWithEnd sets the time until stats are retrieved, specified as milliseconds since the Unix epoch (RSC6b1).
func StatsWithEnd(t time.Time) StatsOption {
	return func(o *statsOptions) {
		o.params.Set("end", strconv.FormatInt(unixMilli(t), 10))
	}
}

// StatsWithLimit sets an upper limit on the number of stats returned.
// The default is 100, and the maximum is 1000 (RSC6b3).
func StatsWithLimit(limit int) StatsOption {
	return func(o *statsOptions) {
		o.params.Set("limit", strconv.Itoa(limit))
	}
}

// StatsWithDirection sets the order for which stats are returned in. Valid values are backwards which
// orders stats from most recent to oldest, or forwards which orders stats from oldest to most recent.
// The default is backwards (RSC6b2).
func StatsWithDirection(d Direction) StatsOption {
	return func(o *statsOptions) {
		o.params.Set("direction", string(d))
	}
}

type PeriodUnit string

const (
	PeriodMinute PeriodUnit = "minute"
	PeriodHour   PeriodUnit = "hour"
	PeriodDay    PeriodUnit = "day"
	PeriodMonth  PeriodUnit = "month"
)

// StatsWithUnit sets minute, hour, day or month. Based on the unit selected, the given start or end times are
// rounded down to the start of the relevant interval depending on the unit granularity of the query (RSC6b4).
func StatsWithUnit(d PeriodUnit) StatsOption {
	return func(o *statsOptions) {
		o.params.Set("unit", string(d))
	}
}

type statsOptions struct {
	params url.Values
}

func (o *statsOptions) apply(opts ...StatsOption) url.Values {
	o.params = make(url.Values)
	for _, opt := range opts {
		opt(o)
	}
	return o.params
}

// StatsRequest represents a request prepared by the REST.Stats or
// Realtime.Stats method, ready to be performed by its Pages or Items methods.
type StatsRequest struct {
	r paginatedRequest
}

// Pages returns an iterator for whole pages of Stats.
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (r StatsRequest) Pages(ctx context.Context) (*StatsPaginatedResult, error) {
	var res StatsPaginatedResult
	return &res, res.load(ctx, r.r)
}

// A StatsPaginatedResult is an iterator for the result of a Stats request.
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
type StatsPaginatedResult struct {
	PaginatedResult
	items []*Stats
}

// Next retrieves the next page of results.
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (p *StatsPaginatedResult) Next(ctx context.Context) bool {
	p.items = nil // avoid mutating already returned items
	return p.next(ctx, &p.items)
}

// IsLast returns true if the page is last page.
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (p *StatsPaginatedResult) IsLast(ctx context.Context) bool {
	return !p.HasNext(ctx)
}

// HasNext returns true is there are more pages available.
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (p *StatsPaginatedResult) HasNext(ctx context.Context) bool {
	return p.nextLink != ""
}

// Items returns the current page of results.
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (p *StatsPaginatedResult) Items() []*Stats {
	return p.items
}

// Items returns a convenience iterator for single Stats, over an underlying
// paginated iterator.
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (r StatsRequest) Items(ctx context.Context) (*StatsPaginatedItems, error) {
	var res StatsPaginatedItems
	var err error
	res.next, err = res.loadItems(ctx, r.r, func() (interface{}, func() int) {
		res.items = nil // avoid mutating already returned items
		return &res.items, func() int { return len(res.items) }
	})
	return &res, err
}

type StatsPaginatedItems struct {
	PaginatedResult
	items []*Stats
	item  *Stats
	next  func(context.Context) (int, bool)
}

// Next retrieves the next result.
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (p *StatsPaginatedItems) Next(ctx context.Context) bool {
	i, ok := p.next(ctx)
	if !ok {
		return false
	}
	p.item = p.items[i]
	return true
}

// Item returns the current result.
//
// See package-level documentation => [ably] Pagination for handling stats pagination.
func (p *StatsPaginatedItems) Item() *Stats {
	return p.item
}

// request contains fields necessary to compose http request that will be sent ably endpoints.
type request struct {
	Method string
	Path   string
	In     interface{} // value to be encoded and sent with request body
	Out    interface{} // value to store decoded response body

	// NoAuth when set true, makes the request not being authenticated.
	NoAuth bool

	// NoRenew when set true, token is not refreshed when request fails with token expired response
	NoRenew bool
	header  http.Header
}

// Request makes a REST request with given http method (GET, POST) and url path. This is provided as a convenience for
// developers who wish to use REST API functionality that is either not documented or is not yet included in the
// public API, without having to directly handle features such as authentication, paging, fallback hosts, MsgPack
// and JSON support (RSC19).
func (c *REST) Request(method string, path string, o ...RequestOption) RESTRequest {
	method = strings.ToUpper(method)
	var opts requestOptions
	opts.apply(o...)
	return RESTRequest{r: paginatedRequest{
		path:   path,
		params: opts.params,
		query: func(ctx context.Context, path string) (*http.Response, error) {
			switch method {
			case "GET", "POST", "PUT", "PATCH", "DELETE": // spec RSC19a
			default:
				return nil, fmt.Errorf("invalid HTTP method: %q", method)
			}
			var lastResponse *http.Response

			req := &request{
				Method: method,
				Path:   path,
				In:     opts.body,
				header: opts.headers,
			}
			resp, err := c.doWithHandle(ctx, req, func(resp *http.Response, out interface{}) (*http.Response, error) {
				// Save the resp but return an error on bad status to trigger fallback
				lastResponse = resp
				return c.handleResponse(resp, nil)
			})

			// RSC19e
			// Only return an error if there was an actual network failiure
			if err != nil && lastResponse != nil {
				return lastResponse, nil
			}
			return resp, err
		},
	}}
}

type requestOptions struct {
	params  url.Values
	headers http.Header
	body    interface{}
}

// RequestOption configures a call to REST.Request.
type RequestOption func(*requestOptions)

// RequestWithParams sets the parameters to include in the URL query of the request.
// The parameters depend on the endpoint being queried.
// See the REST API reference for the available parameters of each endpoint.
func RequestWithParams(params url.Values) RequestOption {
	return func(o *requestOptions) {
		o.params = params
	}
}

// RequestWithHeaders sets additional HTTP headers to include in the request.
func RequestWithHeaders(headers http.Header) RequestOption {
	return func(o *requestOptions) {
		o.headers = headers
	}
}

// RequestWithBody sets the JSON body of the request.
func RequestWithBody(body interface{}) RequestOption {
	return func(o *requestOptions) {
		o.body = body
	}
}

func (o *requestOptions) apply(opts ...RequestOption) {
	o.params = make(url.Values)
	for _, opt := range opts {
		opt(o)
	}
}

// RESTRequest represents a request prepared by the REST.Request method, ready
// to be performed by its Pages or Items methods.
type RESTRequest struct {
	// r is an [ably.HTTPPaginatedResponse] object returned by the HTTP request, containing an empty or
	// JSON-encodable object.
	r paginatedRequest
}

// Pages returns an iterator for whole pages of results.
//
// See package-level documentation => [ably] Pagination for more details.
func (r RESTRequest) Pages(ctx context.Context) (*HTTPPaginatedResponse, error) {
	var res HTTPPaginatedResponse
	return &res, res.load(ctx, r.r)
}

// HTTPPaginatedResponse is a superset of [ably.PaginatedResult] which represents a page of results plus metadata
// indicating the relative queries available to it. HttpPaginatedResponse additionally carries information
// about the response to an HTTP request.
// A HTTPPaginatedResponse is an iterator for the response of a REST request.
//
// See package-level documentation => [ably] Pagination for more details.
type HTTPPaginatedResponse struct {
	PaginatedResult
	items raw
}

// StatusCode is the HTTP status code of the response (HP4).
func (r *HTTPPaginatedResponse) StatusCode() int {
	return r.res.StatusCode
}

// Success is whether statusCode indicates success. This is equivalent to 200 <= statusCode < 300 (HP5).
func (r *HTTPPaginatedResponse) Success() bool {
	return 200 <= r.res.StatusCode && r.res.StatusCode < 300
}

// ErrorCode is the error code if the X-Ably-Errorcode HTTP header is sent in the response (HP6).
func (r *HTTPPaginatedResponse) ErrorCode() ErrorCode {
	codeStr := r.res.Header.Get(ablyErrorCodeHeader)
	if codeStr == "" {
		return ErrNotSet
	}
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		return ErrNotSet
	}
	return ErrorCode(code)
}

// ErrorMessage is the error message if the X-Ably-Errormessage HTTP header is sent in the response (HP7).
func (r *HTTPPaginatedResponse) ErrorMessage() string {
	return r.res.Header.Get(ablyErrorMessageHeader)
}

// Headers are the headers of the response (HP8).
func (r *HTTPPaginatedResponse) Headers() http.Header {
	return r.res.Header
}

// Next retrieves the next page of results.
//
// See package-level documentation => [ably] Pagination for more details.
func (p *HTTPPaginatedResponse) Next(ctx context.Context) bool {
	p.items = nil
	return p.next(ctx, &p.items)
}

// IsLast returns true if the page is last page.
//
// See package-level documentation => [ably] Pagination for more details.
func (p *HTTPPaginatedResponse) IsLast(ctx context.Context) bool {
	return !p.HasNext(ctx)
}

// HasNext returns true is there are more pages available.
//
// See package-level documentation => [ably] Pagination for more details.
func (p *HTTPPaginatedResponse) HasNext(ctx context.Context) bool {
	return p.nextLink != ""
}

// Items contains a page of results; for example, an array of [ably.Message] or [ably.PresenceMessage]
// objects for a channel history request (HP3)
//
// See package-level documentation => [ably] Pagination for more details.
func (p *HTTPPaginatedResponse) Items(dst interface{}) error {
	typ, _, err := mime.ParseMediaType(p.PaginatedResult.res.Header.Get("Content-Type"))
	if err != nil {
		return err
	}

	/// Turn non arrays into arrays so it fits the output kind
	if typ == "application/json" {
		token, _ := json.NewDecoder(bytes.NewReader(p.items)).Token()
		if token != json.Delim('[') {
			p.items = append(raw{'['}, p.items...)
			p.items = append(p.items, ']')
		}
	} else if typ == "application/x-msgpack" {
		// 0x9, 0xdc, 0xdd are message pack's array types
		// append array if the start is not already an array type
		if (p.items[0]&0xf0 != 0x90) && p.items[0] != 0xdc && p.items[0] != 0xdd {
			p.items = append(raw{0x91}, p.items...)
		}
	}
	return decode(typ, bytes.NewReader(p.items), dst)
}

// Items returns a convenience iterator for single items, over an underlying
// paginated iterator.
//
// See package-level documentation => [ably] Pagination for more details.
func (r RESTRequest) Items(ctx context.Context) (*RESTPaginatedItems, error) {
	var res RESTPaginatedItems
	var err error
	res.next, err = res.loadItems(ctx, r.r, func() (interface{}, func() int) {
		res.items = nil
		return &res.items, func() int { return len(res.items) }
	})
	return &res, err
}

type RESTPaginatedItems struct {
	PaginatedResult
	items []raw
	item  raw
	next  func(context.Context) (int, bool)
}

// Next retrieves the next result.
//
// See the "Paginated results" section in the package-level documentation.
func (p *RESTPaginatedItems) Next(ctx context.Context) bool {
	i, ok := p.next(ctx)
	if !ok {
		return false
	}
	p.item = p.items[i]
	return true
}

// IsLast returns true if the page is last page.
//
// See package-level documentation => [ably] Pagination for more details.
func (p *RESTPaginatedItems) IsLast(ctx context.Context) bool {
	return !p.HasNext(ctx)
}

// HasNext returns true is there are more pages available.
//
// See package-level documentation => [ably] Pagination for more details.
func (p *RESTPaginatedItems) HasNext(ctx context.Context) bool {
	return p.nextLink != ""
}

// Item unmarshal the current result into the provided variable.
//
// See the "Paginated results" section in the package-level documentation.
func (p *RESTPaginatedItems) Item(dst interface{}) error {
	typ, _, err := mime.ParseMediaType(p.PaginatedResult.res.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	return decode(typ, bytes.NewReader(p.item), dst)
}

func (c *REST) get(ctx context.Context, path string, out interface{}) (*http.Response, error) {
	r := &request{
		Method: "GET",
		Path:   path,
		Out:    out,
	}
	return c.do(ctx, r)
}

func (c *REST) post(ctx context.Context, path string, in, out interface{}) (*http.Response, error) {
	r := &request{
		Method: "POST",
		Path:   path,
		In:     in,
		Out:    out,
	}
	return c.do(ctx, r)
}

func (c *REST) do(ctx context.Context, r *request) (*http.Response, error) {
	return c.doWithHandle(ctx, r, c.handleResponse)
}

func (c *REST) doWithHandle(ctx context.Context, r *request, handle func(*http.Response, interface{}) (*http.Response, error)) (*http.Response, error) {
	req, err := c.newHTTPRequest(ctx, r)
	if err != nil {
		return nil, err
	}
	if h := c.hostCache.get(); h != "" {
		req.URL.Host = h // RSC15f
		c.log.Verbosef("RestClient: setting cached URL.Host=%q", h)
	} else if !empty(c.activeRealtimeHost) { // RTN17e
		req.URL.Host = c.activeRealtimeHost
		c.log.Verbosef("RestClient: setting activeRealtimeHost URL.Host=%q", c.activeRealtimeHost)
	}

	if c.opts.Trace != nil {
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), c.opts.Trace))
		c.log.Verbose("RestClient: enabling httptrace")
	}
	resp, err := c.opts.httpclient().Do(req)
	serverResp := resp
	if err == nil {
		resp, err = handle(resp, r.Out)
	} else {
		c.log.Error("RestClient: failed sending a request ", err)
	}
	if err != nil {
		c.log.Error("RestClient: error handling response: ", err)
		if canFallBack(err, serverResp) {
			fallbacks, _ := c.opts.getFallbackHosts()
			c.log.Infof("RestClient: trying to fallback with hosts=%v", fallbacks)
			if len(fallbacks) > 0 {
				left := fallbacks
				iteration := 0
				maxLimit := c.opts.HTTPMaxRetryCount
				if maxLimit == 0 {
					maxLimit = defaultOptions.HTTPMaxRetryCount
				}
				c.log.Infof("RestClient: maximum fallback retry limit=%d", maxLimit)

				for {
					if len(left) == 0 {
						c.log.Errorf("RestClient: exhausted fallback hosts", err)
						return nil, err
					}
					var h string
					if len(left) == 1 {
						h = left[0]
					} else {
						h = left[rand.Intn(len(left)-1)]
					}
					var n []string
					for _, v := range left {
						if v != h {
							n = append(n, v)
						}
					}
					left = n
					req, err := c.newHTTPRequest(ctx, r)
					if err != nil {
						return nil, err
					}
					c.log.Infof("RestClient:  chose fallback host=%q ", h)
					req.URL.Host = h
					req.Host = ""
					req.Header.Set(hostHeader, h)
					resp, err := c.opts.httpclient().Do(req)
					serverResp := resp
					if err == nil {
						resp, err = handle(resp, r.Out)
					} else {
						c.log.Error("RestClient: failed sending a request to a fallback host", err)
					}
					if err != nil {
						c.log.Error("RestClient: error handling response: ", err)
						if iteration == maxLimit-1 {
							return nil, err
						}
						if canFallBack(err, serverResp) {
							iteration++
							continue
						}
						return nil, err
					}
					c.hostCache.put(h)
					return resp, nil
				}
			}
			return nil, err
		}
		if e, ok := err.(*ErrorInfo); ok {
			if e.Code == ErrTokenErrorUnspecified {
				if r.NoRenew || !c.Auth.isTokenRenewable() {
					return nil, err
				}
				if _, err := c.Auth.reauthorize(ctx); err != nil {
					return nil, err
				}
				r.NoRenew = true
				return c.do(ctx, r)
			}
		}
		return nil, err
	}
	return resp, nil
}

func canFallBack(err error, res *http.Response) bool {
	return isStatusCodeBetween500_504(res) || // RSC15l3
		isCloudFrontError(res) || //RSC15l4
		isTimeoutOrDnsErr(err) //RSC15l1, RSC15l2
}

// RSC15l3
func isStatusCodeBetween500_504(res *http.Response) bool {
	return res != nil &&
		res.StatusCode >= http.StatusInternalServerError &&
		res.StatusCode <= http.StatusGatewayTimeout
}

// RSC15l4
func isCloudFrontError(res *http.Response) bool {
	return res != nil &&
		strings.EqualFold(res.Header.Get("Server"), "CloudFront") &&
		res.StatusCode >= http.StatusBadRequest
}

// newHTTPRequest creates a new http.Request that can be sent to ably endpoints.
// This makes sure necessary headers are set.
func (c *REST) newHTTPRequest(ctx context.Context, r *request) (*http.Request, error) {
	var body io.Reader
	var protocol = c.opts.protocol()
	if r.In != nil {
		p, err := encode(protocol, r.In)
		if err != nil {
			return nil, newError(ErrProtocolError, err)
		}
		body = bytes.NewReader(p)
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, c.opts.restURL()+r.Path, body)
	if err != nil {
		return nil, newError(ErrInternalError, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", protocol) //spec RSC19c
	}
	if r.header != nil {
		copyHeader(req.Header, r.header)
	}
	req.Header.Set("Accept", protocol)                                  // RSC19c
	req.Header.Set(ablyProtocolVersionHeader, ablyProtocolVersion)      // RSC7a
	req.Header.Set(ablyAgentHeader, ablyAgentIdentifier(c.opts.Agents)) // RSC7d
	if c.opts.ClientID != "" && c.Auth.method == authBasic {
		// References RSA7e2
		h := base64.StdEncoding.EncodeToString([]byte(c.opts.ClientID))
		req.Header.Set(ablyClientIDHeader, h)
	}
	if !r.NoAuth {
		//spec RSC19b
		if err := c.Auth.authReq(req); err != nil {
			return nil, err
		}
	}
	return req, nil
}

func (c *REST) handleResponse(resp *http.Response, out interface{}) (*http.Response, error) {
	c.log.Info("RestClient:checking valid http response")
	if err := checkValidHTTPResponse(resp); err != nil {
		c.log.Error("RestClient: failed to check valid http response ", err)
		return nil, err
	}
	if out == nil {
		return resp, nil
	}
	c.log.Info("RestClient: decoding response")
	if err := decodeResp(resp, out); err != nil {
		c.log.Error("RestClient: failed to decode response ", err)
		return nil, err
	}
	return resp, nil
}

func encode(typ string, in interface{}) ([]byte, error) {
	switch typ {
	case "application/json":
		return json.Marshal(in)
	case "application/x-msgpack":
		return ablyutil.MarshalMsgpack(in)
	case "text/plain":
		return []byte(fmt.Sprintf("%v", in)), nil
	default:
		return nil, newErrorf(40000, "encoding error: unrecognized Content-Type: %q", typ)
	}
}

func decode(typ string, r io.Reader, out interface{}) error {
	switch typ {
	case "application/json":
		return json.NewDecoder(r).Decode(out)
	case "application/x-msgpack":
		b, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		return ablyutil.UnmarshalMsgpack(b, out)
	case "text/plain":
		p, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		_, err = fmt.Sscanf(string(p), "%v", out)
		return err
	default:
		return newErrorf(40000, "decoding error: unrecognized Content-Type: %q", typ)
	}
}

func decodeResp(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	typ, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	b, _ := io.ReadAll(resp.Body)

	return decode(typ, bytes.NewReader(b), out)
}

// hostCache caches a successful fallback host for 10 minutes.
// Only used by REST client while making requests RSC15f
type hostCache struct {
	duration time.Duration

	sync.RWMutex
	deadline time.Time
	host     string
}

func (c *hostCache) put(host string) {
	c.Lock()
	defer c.Unlock()
	c.host = host
	c.deadline = time.Now().Add(c.duration)
}

func (c *hostCache) get() string {
	c.RLock()
	defer c.RUnlock()
	if ablyutil.Empty(c.host) || time.Until(c.deadline) <= 0 {
		return ""
	}
	return c.host
}
