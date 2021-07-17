package ably

import (
	"bytes"
	"context"
	_ "crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

// RESTChannels provides an API for managing collection of RESTChannel. This is
// safe for concurrent use.
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

type REST struct {
	Auth     *Auth
	Channels *RESTChannels
	opts     *clientOptions
	hosts    *restHosts
	log      logger
}

// NewREST constructs a new REST.
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
	c.hosts = newRestHosts(c.opts)
	return c, nil
}

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

// Stats retrieves statistics about the Ably app's activity.
func (c *REST) Stats(o ...StatsOption) StatsRequest {
	params := (&statsOptions{}).apply(o...)
	return StatsRequest{r: c.newPaginatedRequest("/stats", params)}
}

// A StatsOption configures a call to REST.Stats or Realtime.Stats.
type StatsOption func(*statsOptions)

func StatsWithStart(t time.Time) StatsOption {
	return func(o *statsOptions) {
		o.params.Set("start", strconv.FormatInt(unixMilli(t), 10))
	}
}

func StatsWithEnd(t time.Time) StatsOption {
	return func(o *statsOptions) {
		o.params.Set("end", strconv.FormatInt(unixMilli(t), 10))
	}
}

func StatsWithLimit(limit int) StatsOption {
	return func(o *statsOptions) {
		o.params.Set("limit", strconv.Itoa(limit))
	}
}

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
// See "Paginated results" section in the package-level documentation.
func (r StatsRequest) Pages(ctx context.Context) (*StatsPaginatedResult, error) {
	var res StatsPaginatedResult
	return &res, res.load(ctx, r.r)
}

// A StatsPaginatedResult is an iterator for the result of a Stats request.
//
// See "Paginated results" section in the package-level documentation.
type StatsPaginatedResult struct {
	PaginatedResult
	items []*Stats
}

// Next retrieves the next page of results.
//
// See the "Paginated results" section in the package-level documentation.
func (p *StatsPaginatedResult) Next(ctx context.Context) bool {
	p.items = nil // avoid mutating already returned items
	return p.next(ctx, &p.items)
}

// Items returns the current page of results.
//
// See the "Paginated results" section in the package-level documentation.
func (p *StatsPaginatedResult) Items() []*Stats {
	return p.items
}

// Items returns a convenience iterator for single Stats, over an underlying
// paginated iterator.
//
// See "Paginated results" section in the package-level documentation.
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
// See the "Paginated results" section in the package-level documentation.
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
// See the "Paginated results" section in the package-level documentation.
func (p *StatsPaginatedItems) Item() *Stats {
	return p.item
}

// request this contains fields necessary to compose http request that will be
// sent ably endpoints.
type request struct {
	Method string
	Path   string
	In     interface{} // value to be encoded and sent with request body
	Out    interface{} // value to store decoded response body

	// NoAuth when set to true, makes the request not being authenticated.
	NoAuth bool

	// when true token is not refreshed when request fails with token expired response
	NoRenew bool
	header  http.Header
}

// Request prepares an arbitrary request to the REST API.
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

			req := &request{
				Method: method,
				Path:   path,
				In:     opts.body,
				header: opts.headers,
			}
			return c.doWithHandle(ctx, req, func(resp *http.Response, out interface{}) (*http.Response, error) {
				return resp, nil
			})
		},
	}}
}

type requestOptions struct {
	params  url.Values
	headers http.Header
	body    interface{}
}

// A RequestOption configures a call to REST.Request.
type RequestOption func(*requestOptions)

func RequestWithParams(params url.Values) RequestOption {
	return func(o *requestOptions) {
		o.params = params
	}
}

func RequestWithHeaders(headers http.Header) RequestOption {
	return func(o *requestOptions) {
		o.headers = headers
	}
}

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
	r paginatedRequest
}

// Pages returns an iterator for whole pages of results.
//
// See "Paginated results" section in the package-level documentation.
func (r RESTRequest) Pages(ctx context.Context) (*HTTPPaginatedResponse, error) {
	var res HTTPPaginatedResponse
	return &res, res.load(ctx, r.r)
}

// A HTTPPaginatedResponse is an iterator for the response of a REST request.
//
// See "Paginated results" section in the package-level documentation.
type HTTPPaginatedResponse struct {
	PaginatedResult
	items jsonRawArray
}

func (r *HTTPPaginatedResponse) StatusCode() int {
	return r.res.StatusCode
}

func (r *HTTPPaginatedResponse) Success() bool {
	return 200 <= r.res.StatusCode && r.res.StatusCode < 300
}

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

func (r *HTTPPaginatedResponse) ErrorMessage() string {
	return r.res.Header.Get(ablyErrorMessageHeader)
}

func (r *HTTPPaginatedResponse) Headers() http.Header {
	return r.res.Header
}

// Next retrieves the next page of results.
//
// See the "Paginated results" section in the package-level documentation.
func (p *HTTPPaginatedResponse) Next(ctx context.Context) bool {
	p.items = nil
	return p.next(ctx, &p.items)
}

// Items unmarshals the current page of results as JSON into the provided
// variable.
//
// See the "Paginated results" section in the package-level documentation.
func (p *HTTPPaginatedResponse) Items(dst interface{}) error {
	return json.Unmarshal(p.items, dst)
}

// Items returns a convenience iterator for single items, over an underlying
// paginated iterator.
//
// For each item,
//
// See "Paginated results" section in the package-level documentation.
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
	items []json.RawMessage
	item  json.RawMessage
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

// Item unmarshal the current result as JSON into the provided variable.
//
// See the "Paginated results" section in the package-level documentation.
func (p *RESTPaginatedItems) Item(dst interface{}) error {
	return json.Unmarshal(p.item, dst)
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
	defer c.hosts.resetVisitedFallbackHosts()

	maxHTTPRequestLimit := c.opts.HTTPMaxRetryCount
	if maxHTTPRequestLimit == 0 {
		maxHTTPRequestLimit = defaultOptions.HTTPMaxRetryCount
	}

	host := c.hosts.getPreferredHost()
	iteration := 0

	for {
		req, err := c.newHTTPRequest(ctx, r, host)
		if err != nil {
			return nil, err
		}
		if host != c.hosts.getPrimaryHost() { // set hostheader for fallback host
			req.Host = req.URL.Host // RSC15j set host header, https://github.com/golang/go/issues/7682, since req.Host overrides req.URL.Host, use the same value
			req.Header.Set(hostHeader, host)
		}
		if c.opts.Trace != nil {
			req = req.WithContext(httptrace.WithClientTrace(req.Context(), c.opts.Trace))
			c.log.Verbose("RestClient: enabling httptrace")
		}
		resp, err := c.opts.httpclient().Do(req)
		isTimeoutOrDNSErr := isTimeoutOrDnsErr(err)
		if err != nil && !isTimeoutOrDNSErr { //RSC15d, RTN17d
			c.log.Error("RestClient: failed sending a request ", err)
			return nil, newError(ErrInternalError, err)
		}
		if err == nil {
			resp, err = handle(resp, r.Out)
		}
		if err != nil {
			c.log.Error("RestClient: error handling response: ", err)
			errorInfo, isErrorInfo := err.(*ErrorInfo)
			isServerError := isErrorInfo && errorInfo.StatusCode >= http.StatusInternalServerError && errorInfo.StatusCode <= http.StatusGatewayTimeout

			if (isTimeoutOrDNSErr || isServerError) && c.hosts.fallbackHostsRemaining() > 0 && iteration < maxHTTPRequestLimit {
				host = c.hosts.nextFallbackHost()
				c.log.Infof("RestClient: trying out fallback with host=%s", host)
				iteration++
			} else if isErrorInfo && errorInfo.Code == ErrTokenErrorUnspecified {
				if r.NoRenew || !c.Auth.isTokenRenewable() {
					return nil, err
				}
				if _, err := c.Auth.reauthorize(ctx); err != nil {
					return nil, err
				}
				r.NoRenew = true
				return c.do(ctx, r)
			} else {
				return nil, err
			}
		} else { //success
			c.hosts.cacheHost(host)
			return resp, nil
		}
	}
}

// newHTTPRequest creates a new http.Request that can be sent to ably endpoints.
// This makes sure necessary headers are set.
func (c *REST) newHTTPRequest(ctx context.Context, r *request, host string) (*http.Request, error) {
	var body io.Reader
	var protocol = c.opts.protocol()
	if r.In != nil {
		p, err := encode(protocol, r.In)
		if err != nil {
			return nil, newError(ErrProtocolError, err)
		}
		body = bytes.NewReader(p)
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, c.opts.restURL(host)+r.Path, body)
	if err != nil {
		return nil, newError(ErrInternalError, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", protocol) //spec RSC19c
	}
	if r.header != nil {
		copyHeader(req.Header, r.header)
	}
	req.Header.Set("Accept", protocol) //spec RSC19c
	req.Header.Set(ablyVersionHeader, ablyVersion)
	req.Header.Set(ablyLibHeader, libraryString)
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
		b, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}
		return ablyutil.UnmarshalMsgpack(b, out)
	case "text/plain":
		p, err := ioutil.ReadAll(r)
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
	b, _ := ioutil.ReadAll(resp.Body)

	return decode(typ, bytes.NewReader(b), out)
}

// jsonRawArray is a json.RawMessage that, if it's not an array already, wrap
// itself in a JSON array when marshaled into.
type jsonRawArray json.RawMessage

func (m *jsonRawArray) UnmarshalJSON(data []byte) error {
	err := (*json.RawMessage)(m).UnmarshalJSON(data)
	if err != nil {
		return err
	}
	token, _ := json.NewDecoder(bytes.NewReader(*m)).Token()
	if token != json.Delim('[') {
		*m = append(
			jsonRawArray("["),
			append(
				*m,
				']',
			)...,
		)
	}
	return nil
}
