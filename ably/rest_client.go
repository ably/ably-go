package ably

import (
	"bytes"
	_ "crypto/sha512"
	"encoding/json"
	"errors"
	"io"
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
	Auth     *Auth
	Protocol string
	Host     string

	chansMtx sync.Mutex
	chans    map[string]*RestChannel
}

func NewRestClient(options *ClientOptions) (*RestClient, error) {
	keyName, keySecret := options.KeyName(), options.KeySecret()
	if keyName == "" || keySecret == "" {
		return nil, newErrorf(40005, "invalid key format")
	}
	c := &RestClient{
		Protocol: options.protocol(),
		Host:     options.restURL(),
		chans:    make(map[string]*RestChannel),
	}
	c.Auth = &Auth{
		options:   *options,
		client:    c,
		keyName:   keyName,
		keySecret: keySecret,
	}
	return c, nil
}

func (c *RestClient) Time() (time.Time, error) {
	times := []int64{}
	_, err := c.Get("/time", &times)
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

func (c *RestClient) handleResp(v interface{}, resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, newError(50000, err)
	}
	if err = checkValidHTTPResponse(resp); err != nil {
		return nil, err
	}
	if v == nil {
		return resp, nil
	}
	defer resp.Body.Close()
	proto := c.Auth.options.protocol()
	if typ := strip(resp.Header.Get("Content-Type"), ';'); typ != "" && typ != protoMIME[proto] {
		return nil, newError(40000, errors.New("unrecognized Content-Type: "+typ))
	}
	switch proto {
	case ProtocolJSON:
		err = json.NewDecoder(resp.Body).Decode(v)
	case ProtocolMsgPack:
		err = msgpack.NewDecoder(resp.Body).Decode(v)
	}
	if err != nil {
		return nil, newError(ErrCodeProtocol, err)
	}
	return resp, nil
}

// Stats gives the channel's metrics according to the given parameters.
// The returned result can be inspected for the statistics via the Stats()
// method.
func (c *RestClient) Stats(params *PaginateParams) (*PaginatedResult, error) {
	return newPaginatedResult(statType, "/stats", params, query(c.Get))
}

func (c *RestClient) Get(path string, out interface{}) (*http.Response, error) {
	req, err := c.newRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Auth.options.httpclient().Do(req)
	return c.handleResp(out, resp, err)
}

func (c *RestClient) Post(path string, in, out interface{}) (*http.Response, error) {
	req, err := c.newRequest("POST", path, in)
	if err != nil {
		return nil, err
	}
	resp, err := c.Auth.options.httpclient().Do(req)
	return c.handleResp(out, resp, err)
}

var protoMIME = map[string]string{
	ProtocolJSON:    "application/json",
	ProtocolMsgPack: "application/x-msgpack",
}

func (c *RestClient) newRequest(method, path string, in interface{}) (*http.Request, error) {
	var body io.Reader
	var proto = c.Auth.options.protocol()
	if in != nil {
		p, err := c.marshalMessages(proto, in)
		if err != nil {
			return nil, newError(ErrCodeProtocol, err)
		}
		body = bytes.NewReader(p)
	}
	req, err := http.NewRequest(method, c.Auth.options.restURL()+path, body)
	if err != nil {
		return nil, newError(50000, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", protoMIME[proto])
	}
	req.Header.Set("Accept", protoMIME[proto])
	req.SetBasicAuth(c.Auth.keyName, c.Auth.keySecret)
	return req, nil
}

func (c *RestClient) marshalMessages(proto string, in interface{}) ([]byte, error) {
	switch proto {
	case ProtocolJSON:
		return json.Marshal(in)
	case ProtocolMsgPack:
		return msgpack.Marshal(in)
	default:
		return nil, newErrorf(ErrCodeProtocol, "invalid protocol: %q", proto)
	}
}
