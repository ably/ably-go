package ably

import (
	"bytes"
	_ "crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
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
		return nil, errors.New("invalid key format")
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
		return time.Time{}, fmt.Errorf("expected 1 timestamp, got %d", len(times))
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
	return newPaginatedResult(statType, "/stats", params, query(c.Get))
}

func (c *RestClient) Get(path string, out interface{}) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.Auth.options.restURL()+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Auth.keyName, c.Auth.keySecret)
	res, err := c.Auth.options.httpclient().Do(req)
	if err != nil {
		return nil, err
	}
	if !c.ok(res.StatusCode) {
		return res, NewRestHttpError(res, fmt.Sprintf("unexpected status code %d", res.StatusCode))
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
	req, err := http.NewRequest("POST", c.Auth.options.restURL()+path, bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Auth.keyName, c.Auth.keySecret)
	res, err := c.Auth.options.httpclient().Do(req)
	if err != nil {
		return nil, err
	}
	if !c.ok(res.StatusCode) {
		return res, NewRestHttpError(res, fmt.Sprintf("unexpected status code %d", res.StatusCode))
	}
	if out != nil {
		defer res.Body.Close()
		return res, json.NewDecoder(res.Body).Decode(out)
	}
	return res, nil
}

func (c *RestClient) ok(status int) bool {
	return status == http.StatusOK || status == http.StatusCreated
}

func (c *RestClient) marshalMessages(in interface{}) ([]byte, error) {
	switch proto := c.Auth.options.protocol(); proto {
	case ProtocolJSON:
		return json.Marshal(in)
	case ProtocolMsgPack:
		return msgpack.Marshal(in)
	default:
		return nil, errors.New(`invalid protocol: "` + proto + `"`)
	}
}
