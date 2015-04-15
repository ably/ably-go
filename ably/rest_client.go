package ably

import (
	"bytes"
	_ "crypto/sha512"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
	"github.com/ably/ably-go/ably/proto"
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
	Auth *Auth

	RestEndpoint string
	Protocol     ProtocolType

	HTTPClient *http.Client

	channels map[string]*RestChannel
	chanMtx  sync.Mutex
}

func NewRestClient(params Params) *RestClient {
	params.Prepare()
	client := &RestClient{
		RestEndpoint: params.RestEndpoint,
		HTTPClient:   params.HTTPClient,
		channels:     make(map[string]*RestChannel),
	}

	client.Auth = NewAuth(params, client)
	client.Protocol = params.Protocol

	return client
}

func (c *RestClient) httpclient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *RestClient) Time() (*time.Time, error) {
	times := []int64{}
	_, err := c.Get("/time", &times)
	if err != nil {
		return nil, err
	}
	if len(times) != 1 {
		return nil, fmt.Errorf("Expected 1 timestamp, got %d", len(times))
	}
	t := time.Unix(times[0]/1000, times[0]%1000)
	return &t, nil
}

func (c *RestClient) Channel(name string) *RestChannel {
	c.chanMtx.Lock()
	defer c.chanMtx.Unlock()

	if ch, ok := c.channels[name]; ok {
		return ch
	}

	ch := newRestChannel(name, c)
	c.channels[name] = ch
	return ch
}

// Stats gives the channel's metrics according to the given parameters.
// The returned resource can be inspected for the statistics via the Stats()
// method.
func (c *RestClient) Stats(params *PaginateParams) (*PaginatedResource, error) {
	return newPaginatedResource(statType, "/stats", params, query(c.Get))
}

func (c *RestClient) Get(path string, out interface{}) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.RestEndpoint+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Auth.AppID, c.Auth.AppSecret)
	res, err := c.httpclient().Do(req)

	if err != nil {
		return nil, err
	}

	if !c.ok(res.StatusCode) {
		return res, NewRestHttpError(res, fmt.Sprintf("Unexpected status code %d", res.StatusCode))
	}

	if out != nil {
		defer res.Body.Close()
		return res, json.NewDecoder(res.Body).Decode(out)
	}

	return res, nil
}

func (c *RestClient) Post(path string, in, out interface{}) (*http.Response, error) {
	buf, err := c.marshalMessages(in)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.RestEndpoint+path, bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Auth.AppID, c.Auth.AppSecret)
	res, err := c.httpclient().Do(req)
	if err != nil {
		return nil, err
	}
	if !c.ok(res.StatusCode) {
		return res, NewRestHttpError(res, fmt.Sprintf("Unexpected status code %d", res.StatusCode))
	}
	if out != nil && c.ok(res.StatusCode) {
		defer res.Body.Close()
		return res, json.NewDecoder(res.Body).Decode(out)
	}
	return res, nil
}

func (c *RestClient) ok(status int) bool {
	return status == http.StatusOK || status == http.StatusCreated
}

func (c *RestClient) marshalMessages(in interface{}) ([]byte, error) {
	switch c.Protocol {
	case ProtocolJSON:
		return json.Marshal(in)
	case ProtocolMsgPack:
		return msgpack.Marshal(in)
	default:
		// TODO log fallback to default encoding
		return json.Marshal(in)
	}
}
