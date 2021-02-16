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
	"math/rand"
	"mime"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

var (
	msgType     = reflect.TypeOf((*[]*proto.Message)(nil)).Elem()
	presMsgType = reflect.TypeOf((*[]*proto.PresenceMessage)(nil)).Elem()
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
	cache  map[string]*RESTChannel
	mu     sync.RWMutex
	client *REST
}

// Range iterates over the channels calling fn on every iteration. If fn returns
// false then the iteration is stopped.
//
// This uses locking to take a snapshot of the underlying RESTChannel map before
// iteration to avoid any deadlock, meaning any modification (like creating new
// RESTChannel, or removing one) that occurs during iteration will not have any
// effect to the values passed to the fn.
func (c *RESTChannels) Range(fn func(name string, channel *RESTChannel) bool) {
	clone := make(map[string]*RESTChannel)
	c.mu.RLock()
	for k, v := range c.cache {
		clone[k] = v
	}
	c.mu.RUnlock()
	for k, v := range clone {
		if !fn(k, v) {
			break
		}
	}

}

// Exists returns true if the channel by the given name exists.
func (c *RESTChannels) Exists(name string) bool {
	c.mu.RLock()
	_, ok := c.cache[name]
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
	return c.get(name, (*proto.ChannelOptions)(&o))
}

func (c *RESTChannels) get(name string, opts *proto.ChannelOptions) *RESTChannel {
	c.mu.RLock()
	v, ok := c.cache[name]
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
	c.cache[name] = v
	c.mu.Unlock()
	return v
}

// Release deletes the channel from the cache.
func (c *RESTChannels) Release(name string) {
	c.mu.Lock()
	delete(c.cache, name)
	c.mu.Unlock()
}

type REST struct {
	Auth                *Auth
	Channels            *RESTChannels
	opts                *clientOptions
	successFallbackHost *fallbackCache
}

// NewREST constructs a new REST.
func NewREST(options ...ClientOption) (*REST, error) {
	c := &REST{
		opts: applyOptionsWithDefaults(options...),
	}
	auth, err := newAuth(c)
	if err != nil {
		return nil, err
	}
	c.Auth = auth
	c.Channels = &RESTChannels{
		cache:  make(map[string]*RESTChannel),
		client: c,
	}
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

type StatsRequest struct {
	r paginatedRequestNew
}

func (r StatsRequest) Pages(ctx context.Context) (*StatsPaginatedResult, error) {
	var res StatsPaginatedResult
	return &res, res.load(ctx, r.r)
}

type StatsPaginatedResult struct {
	PaginatedResultNew
	items []*Stats
}

func (p *StatsPaginatedResult) Next(ctx context.Context) bool {
	return p.next(ctx, &p.items)
}

func (p *StatsPaginatedResult) Items() []*Stats {
	return p.items
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

// Request sends http request to ably.
// spec RSC19
func (c *REST) Request(ctx context.Context, method string, path string, params *PaginateParams, body interface{}, headers http.Header) (*HTTPPaginatedResponse, error) {
	method = strings.ToUpper(method)
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE": // spec RSC19a
		return newHTTPPaginatedResult(ctx, path, params, func(ctx context.Context, p string) (*http.Response, error) {
			req := &request{
				Method: method,
				Path:   p,
				In:     body,
				header: headers,
			}
			return c.doWithHandle(ctx, req, func(resp *http.Response, out interface{}) (*http.Response, error) {
				return resp, nil
			})
		}, c.logger())
	default:
		return nil, newErrorFromProto(&proto.ErrorInfo{
			Message:    fmt.Sprintf("%s method is not supported", method),
			Code:       int(ErrMethodNotAllowed),
			StatusCode: http.StatusMethodNotAllowed,
		})
	}
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

// fallbackCache this caches a successful fallback host for 10 minutes.
type fallbackCache struct {
	running  bool
	host     string
	duration time.Duration
	cancel   func()
	mu       sync.RWMutex
}

func (f *fallbackCache) get() string {
	if f.isRunning() {
		f.mu.RLock()
		h := f.host
		f.mu.RUnlock()
		return h
	}
	return ""
}

func (f *fallbackCache) isRunning() bool {
	f.mu.RLock()
	v := f.running
	f.mu.RUnlock()
	return v
}

func (f *fallbackCache) run(host string) {
	f.mu.Lock()
	now := time.Now()
	duration := defaultOptions.FallbackRetryTimeout // spec RSC15f
	if f.duration != 0 {
		duration = f.duration
	}
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(duration))
	f.running = true
	f.host = host
	f.cancel = cancel
	f.mu.Unlock()
	<-ctx.Done()
	f.mu.Lock()
	f.running = false
	f.mu.Unlock()
}

func (f *fallbackCache) stop() {
	f.cancel()
	// we make sure we have stopped
	for {
		if !f.isRunning() {
			return
		}
	}
}

func (f *fallbackCache) put(host string) {
	if f.get() != host {
		if f.isRunning() {
			f.stop()
		}
		go f.run(host)
	}
}

func (c *REST) doWithHandle(ctx context.Context, r *request, handle func(*http.Response, interface{}) (*http.Response, error)) (*http.Response, error) {
	log := c.opts.Logger.sugar()
	if c.successFallbackHost == nil {
		c.successFallbackHost = &fallbackCache{
			duration: c.opts.fallbackRetryTimeout(),
		}
		log.Verbosef("RestClient: setup fallback duration to %v", c.successFallbackHost.duration)
	}
	req, err := c.newHTTPRequest(ctx, r)
	if err != nil {
		return nil, err
	}
	if h := c.successFallbackHost.get(); h != "" {
		req.URL.Host = h // RSC15f
		log.Verbosef("RestClient: setting URL.Host=%q", h)
	}
	if c.opts.Trace != nil {
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), c.opts.Trace))
		log.Verbose("RestClient: enabling httptrace")
	}
	if log.Is(LogVerbose) {
		b, err := httputil.DumpRequest(req, false)
		if err != nil {
			log.Error("RestClient: error trying to dump request: ", err)
		} else {
			log.Verbose("RestClient: ", string(b))
		}
	}
	resp, err := c.opts.httpclient().Do(req)
	if err != nil {
		log.Error("RestClient: failed sending a request ", err)
		return nil, newError(ErrInternalError, err)
	}
	if log.Is(LogVerbose) {
		typ, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		// dumping msgpack body isn't that helpbul when debugging
		b, err := httputil.DumpResponse(resp, typ != "application/x-msgpack")
		if err != nil {
			log.Error("RestClient: error trying to dump response: ", err)
		} else {
			log.Verbose("RestClient: ", string(b))
		}
	}
	resp, err = handle(resp, r.Out)
	if err != nil {
		log.Error("RestClient: error handling response: ", err)
		if e, ok := err.(*ErrorInfo); ok {
			if canFallBack(e.StatusCode) &&
				(strings.HasPrefix(req.URL.Host, defaultOptions.RESTHost) ||
					c.opts.FallbackHosts != nil) {
				fallback := defaultFallbackHosts()
				if c.opts.FallbackHosts != nil {
					fallback = c.opts.FallbackHosts
				}
				log.Info("RestClient: trying to fallback with hosts=%v", fallback)
				if len(fallback) > 0 {
					left := fallback
					iteration := 0
					maxLimit := c.opts.HTTPMaxRetryCount
					if maxLimit == 0 {
						maxLimit = defaultOptions.HTTPMaxRetryCount
					}
					log.Infof("RestClient: maximum fallback retry limit=%d", maxLimit)

					for {
						if len(left) == 0 {
							log.Errorf("RestClient: exhausted fallback hosts", err)
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
						log.Infof("RestClient:  chose fallback host=%q ", h)
						req.URL.Host = h
						req.Host = ""
						req.Header.Set(proto.HostHeader, h)
						if log.Is(LogVerbose) {
							b, err := httputil.DumpRequest(req, true)
							if err != nil {
								log.Error("RestClient: error trying to dump retry request with fallback host: ", err)
							} else {
								log.Verbose("RestClient: ", string(b))
							}
						}
						resp, err := c.opts.httpclient().Do(req)
						if err != nil {
							log.Error("RestClient: failed sending a request to a fallback host", err)
							return nil, newError(ErrInternalError, err)
						}
						resp, err = handle(resp, r.Out)
						if err != nil {
							log.Error("RestClient: error handling response: ", err)
							if iteration == maxLimit-1 {
								return nil, err
							}
							if ev, ok := err.(*ErrorInfo); ok {
								if canFallBack(ev.StatusCode) {
									iteration++
									continue
								}
							}
							return nil, err
						}
						if log.Is(LogVerbose) {
							typ, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
							// dumping msgpack body isn't that helpbul when debugging
							b, err := httputil.DumpResponse(resp, typ != "application/x-msgpack")
							if err != nil {
								log.Error("RestClient: error trying to dump retry response: ", err)
							} else {
								log.Verbose("RestClient:: ", string(b))
							}
						}
						c.successFallbackHost.put(h)
						return resp, nil
					}
				}
				return nil, err
			}
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

func canFallBack(code int) bool {
	return http.StatusInternalServerError <= code &&
		code <= http.StatusGatewayTimeout
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
	req.Header.Set("Accept", protocol) //spec RSC19c
	req.Header.Set(proto.AblyVersionHeader, proto.AblyVersion)
	req.Header.Set(proto.AblyLibHeader, proto.LibraryString)
	if c.opts.ClientID != "" && c.Auth.method == authBasic {
		// References RSA7e2
		h := base64.StdEncoding.EncodeToString([]byte(c.opts.ClientID))
		req.Header.Set(proto.AblyClientIDHeader, h)
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
	log := c.opts.Logger.sugar()
	log.Info("RestClient:checking valid http response")
	if err := checkValidHTTPResponse(resp); err != nil {
		log.Error("RestClient: failed to check valid http response ", err)
		return nil, err
	}
	if out == nil {
		return resp, nil
	}
	log.Info("RestClient: decoding response")
	if err := decodeResp(resp, out); err != nil {
		log.Error("RestClient: failed to decode response ", err)
		return nil, err
	}
	return resp, nil
}

func (c *REST) logger() *LoggerOptions {
	return &c.opts.Logger
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
	return decode(typ, resp.Body, out)
}
