package ably

import (
	"bytes"
	_ "crypto/sha512"
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/ably/ably-go/ably/proto"

	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
)

var (
	msgType     = reflect.TypeOf((*[]*proto.Message)(nil)).Elem()
	statType    = reflect.TypeOf((*[]*proto.Stat)(nil)).Elem()
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
	if err = checkError(resp); err != nil {
		return nil, err
	}
	if v == nil {
		return resp, nil
	}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(v); err != nil {
		return nil, newError(50000, err)
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
	req, err := http.NewRequest("GET", c.Auth.options.restURL()+path, nil)
	if err != nil {
		return nil, newError(50000, err)
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Auth.keyName, c.Auth.keySecret)
	resp, err := c.Auth.options.httpclient().Do(req)
	return c.handleResp(out, resp, err)
}

func (c *RestClient) Post(path string, in, out interface{}) (*http.Response, error) {
	p, err := c.marshalMessages(in)
	if err != nil {
		return nil, newError(ErrCodeProtocol, err)
	}
	req, err := http.NewRequest("POST", c.Auth.options.restURL()+path, bytes.NewReader(p))
	if err != nil {
		return nil, newError(50000, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Auth.keyName, c.Auth.keySecret)
	resp, err := c.Auth.options.httpclient().Do(req)
	return c.handleResp(out, resp, err)
}

func (c *RestClient) marshalMessages(in interface{}) ([]byte, error) {
	switch proto := c.Auth.options.protocol(); proto {
	case ProtocolJSON:
		return json.Marshal(in)
	case ProtocolMsgPack:
		return msgpack.Marshal(in)
	default:
		return nil, newErrorf(ErrCodeProtocol, "invalid protocol: %q", proto)
	}
}
