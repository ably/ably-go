package ably

import (
	"bytes"
	_ "crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

var (
	msgType     = reflect.TypeOf((*[]*proto.Message)(nil)).Elem()
	statType    = reflect.TypeOf((*[]*proto.Stats)(nil)).Elem()
	presMsgType = reflect.TypeOf((*[]*proto.PresenceMessage)(nil)).Elem()
)

// constants for rsc7
const (
	AblyVersionHeader = "X-Ably-Version"
	AblyLibHeader     = "X-Ably-Lib"
	LibraryVersion    = "1.0"
	LibraryName       = "ably-go"
	LibraryString     = LibraryName + "-" + LibraryVersion
	AblyVersion       = "1.0"
)

const HostHeader = "Host"

func query(fn func(string, interface{}) (*http.Response, error)) QueryFunc {
	return func(path string) (*http.Response, error) {
		return fn(path, nil)
	}
}

type RestClient struct {
	Auth *Auth

	chansMtx sync.Mutex
	chans    map[string]*RestChannel
	opts     ClientOptions
}

func NewRestClient(opts *ClientOptions) (*RestClient, error) {
	if opts == nil {
		panic("called NewRealtimeClient with nil ClientOptions")
	}
	c := &RestClient{
		chans: make(map[string]*RestChannel),
		opts:  *opts,
	}
	auth, err := newAuth(c)
	if err != nil {
		return nil, err
	}
	c.Auth = auth
	return c, nil
}

func (c *RestClient) Time() (time.Time, error) {
	var times []int64
	r := &Request{
		Method: "GET",
		Path:   "/time",
		Out:    &times,
		NoAuth: true,
	}
	_, err := c.do(r)
	if err != nil {
		return time.Time{}, err
	}
	if len(times) != 1 {
		return time.Time{}, newErrorf(ErrInternalError, "expected 1 timestamp, got %d", len(times))
	}
	return time.Unix(times[0]/1000, times[0]%1000), nil
}

func (c *RestClient) Channel(name string) *RestChannel {
	c.chansMtx.Lock()
	defer c.chansMtx.Unlock()
	if ch, ok := c.chans[name]; ok {
		return ch
	}
	ch := newRestChannel(name, c)
	c.chans[name] = ch
	return ch
}

// Stats gives the channel's metrics according to the given parameters.
// The returned result can be inspected for the statistics via the Stats()
// method.
func (c *RestClient) Stats(params *PaginateParams) (*PaginatedResult, error) {
	return newPaginatedResult(statType, "/stats", params, query(c.get), c.logger())
}

// Request this contains fields necessary to compose http request that will be
// sent ably endpoints.
type Request struct {
	Method string
	Path   string
	In     interface{} // value to be encoded and sent with request body
	Out    interface{} // value to store decoded response body

	// NoAuth when set to true, makes the request not being authenticated.
	NoAuth bool

	// when true token is not refreshed when request fails with token expired response
	NoRenew bool
}

func (c *RestClient) get(path string, out interface{}) (*http.Response, error) {
	r := &Request{
		Method: "GET",
		Path:   path,
		Out:    out,
	}
	return c.do(r)
}

func (c *RestClient) post(path string, in, out interface{}) (*http.Response, error) {
	r := &Request{
		Method: "POST",
		Path:   path,
		In:     in,
		Out:    out,
	}
	return c.do(r)
}

func (c *RestClient) do(r *Request) (*http.Response, error) {
	req, err := c.NewHTTPRequest(r)
	if err != nil {
		return nil, err
	}
	resp, err := c.opts.httpclient().Do(req)
	if err != nil {
		return nil, newError(ErrInternalError, err)
	}
	resp, err = c.handleResponse(resp, r.Out)
	if err != nil {
		if e, ok := err.(*Error); ok {
			if canFallBack(e.StatusCode) &&
				(c.opts.FallbackHostsUseDefault ||
					strings.HasSuffix(req.URL.Host, defaultOptions.RestHost)) {
				fallback := defaultOptions.FallbackHosts
				if c.opts.FallbackHosts != nil {
					fallback = c.opts.FallbackHosts
				}
				if len(fallback) > 0 {
					left := fallback
					iteration := 0
					maxLimit := c.opts.HTTPMaxRetryCount
					if maxLimit == 0 {
						maxLimit = defaultOptions.HTTPMaxRetryCount
					}
					for {
						if len(left) == 0 {
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
						req, err := c.NewHTTPRequest(r)
						if err != nil {
							return nil, err
						}
						req.URL.Host = h
						req.Header.Set(HostHeader, h)
						resp, err := c.opts.httpclient().Do(req)
						if err != nil {
							return nil, newError(ErrInternalError, err)
						}
						resp, err = c.handleResponse(resp, r.Out)
						if err != nil {
							if iteration == maxLimit-1 {
								return nil, err
							}
							if ev, ok := err.(*Error); ok {
								if canFallBack(ev.StatusCode) {
									iteration++
									continue
								}
							}
							return nil, err
						}
						return resp, nil
					}
				}
				return nil, err
			}
			if e.Code == ErrTokenErrorUnspecified {
				if r.NoRenew || !c.Auth.isTokenRenewable() {
					return nil, err
				}
				if _, err := c.Auth.reauthorise(); err != nil {
					return nil, err
				}
				r.NoRenew = true
				return c.do(r)
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

// NewHTTPRequest creates a new http.Request that can be sent to ably endpoints.
// This makes sure necessary headers are set.
func (c *RestClient) NewHTTPRequest(r *Request) (*http.Request, error) {
	var body io.Reader
	var proto = c.opts.protocol()
	if r.In != nil {
		p, err := encode(proto, r.In)
		if err != nil {
			return nil, newError(ErrCodeProtocol, err)
		}
		body = bytes.NewReader(p)
	}
	req, err := http.NewRequest(r.Method, c.opts.restURL()+r.Path, body)
	if err != nil {
		return nil, newError(ErrInternalError, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", proto)
	}
	req.Header.Set("Accept", proto)
	req.Header.Set(AblyVersionHeader, AblyVersion)
	req.Header.Set(AblyLibHeader, LibraryString)
	if !r.NoAuth {
		if err := c.Auth.authReq(req); err != nil {
			return nil, err
		}
	}
	return req, nil
}

func (c *RestClient) handleResponse(resp *http.Response, out interface{}) (*http.Response, error) {
	if err := checkValidHTTPResponse(resp); err != nil {
		return nil, err
	}
	if out == nil {
		return resp, nil
	}
	if err := decodeResp(resp, out); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *RestClient) logger() *LoggerOptions {
	return &c.opts.Logger
}

func encode(typ string, in interface{}) ([]byte, error) {
	switch typ {
	case "application/json":
		return json.Marshal(in)
	case "application/x-msgpack":
		return ablyutil.Marshal(in)
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
		return ablyutil.Unmarshal(b, out)
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
