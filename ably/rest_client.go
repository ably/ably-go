package ably

import (
	"bytes"
	_ "crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/ably/ably-go/ably/proto"

	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
)

var (
	msgType     = reflect.TypeOf((*[]*proto.Message)(nil)).Elem()
	statType    = reflect.TypeOf((*[]*proto.Stats)(nil)).Elem()
	presMsgType = reflect.TypeOf((*[]*proto.PresenceMessage)(nil)).Elem()
)

func query(fn func(string, interface{}) (*http.Response, error)) QueryFunc {
	return func(path string) (*http.Response, error) {
		return fn(path, nil)
	}
}

type RestClient struct {
	Auth *Auth

	chansMtx sync.Mutex
	chans    map[string]*RestChannel
	options  ClientOptions
}

func NewRestClient(opts *ClientOptions) (*RestClient, error) {
	c := &RestClient{
		chans:   make(map[string]*RestChannel),
		options: *opts,
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
	r := &request{
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
		return time.Time{}, newErrorf(50000, "expected 1 timestamp, got %d", len(times))
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
	return newPaginatedResult(statType, "/stats", params, query(c.get))
}

type request struct {
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
	r := &request{
		Method: "GET",
		Path:   path,
		Out:    out,
	}
	return c.do(r)
}

func (c *RestClient) post(path string, in, out interface{}) (*http.Response, error) {
	r := &request{
		Method: "POST",
		Path:   path,
		In:     in,
		Out:    out,
	}
	return c.do(r)
}

func (c *RestClient) do(r *request) (*http.Response, error) {
	req, err := c.newHTTPRequest(r)
	if err != nil {
		return nil, err
	}
	resp, err := c.options.httpclient().Do(req)
	if err != nil {
		return nil, newError(50000, err)
	}
	resp, err = c.handleResponse(resp, r.Out)
	switch {
	case err == nil:
		return resp, nil
	case code(err) == 40140:
		c.Auth.setToken(nil) // clear expired token
		if r.NoRenew || !c.Auth.isTokenRenewable() {
			return nil, err
		}
		if _, err := c.Auth.reauthorise(true); err != nil {
			return nil, err
		}
		r.NoRenew = true
		return c.do(r)
	default:
		return nil, err
	}
}

func (c *RestClient) newHTTPRequest(r *request) (*http.Request, error) {
	var body io.Reader
	var proto = c.options.protocol()
	if r.In != nil {
		p, err := encode(proto, r.In)
		if err != nil {
			return nil, newError(ErrCodeProtocol, err)
		}
		body = bytes.NewReader(p)
	}
	req, err := http.NewRequest(r.Method, c.options.restURL()+r.Path, body)
	if err != nil {
		return nil, newError(50000, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", proto)
	}
	req.Header.Set("Accept", proto)
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

func encode(typ string, in interface{}) ([]byte, error) {
	switch typ {
	case "application/json":
		return json.Marshal(in)
	case "application/x-msgpack":
		return msgpack.Marshal(in)
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
		return msgpack.NewDecoder(r).Decode(out)
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
