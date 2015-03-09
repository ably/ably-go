package rest

import (
	"bytes"
	_ "crypto/sha512"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ably/ably-go/config"
)

func NewClient(params config.Params) *Client {
	client := &Client{
		RestEndpoint: params.RestEndpoint,
		HttpClient:   http.DefaultClient,
		channels:     make(map[string]*Channel),
	}

	client.Auth = NewAuth(params, client)

	return client
}

type Client struct {
	Auth *Auth

	RestEndpoint string

	HttpClient *http.Client

	channels map[string]*Channel
	chanMtx  sync.Mutex
}

func (c *Client) Time() (*time.Time, error) {
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

func (c *Client) Channel(name string) *Channel {
	c.chanMtx.Lock()
	defer c.chanMtx.Unlock()

	if ch, ok := c.channels[name]; ok {
		return ch
	}

	ch := newChannel(name, c)
	c.channels[name] = ch
	return ch
}

type Stat struct {
	All           map[string]map[string]int
	Inbound       map[string]map[string]map[string]int
	Outbound      map[string]map[string]map[string]int
	Persisted     map[string]map[string]int
	Connections   map[string]map[string]int
	Channels      map[string]float32
	ApiRequests   map[string]int
	TokenRequests map[string]int
	Count         int
	IntervalId    string
}

func (c *Client) Stats(since time.Time) ([]*Stat, error) {
	stats := []*Stat{}
	_, err := c.Get(fmt.Sprintf("/stats?start=%d", since.Unix()), &stats)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (c *Client) Get(path string, out interface{}) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.RestEndpoint+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.Auth.AppID, c.Auth.AppSecret)
	res, err := c.HttpClient.Do(req)

	if err != nil {
		return nil, err
	}

	if !c.ok(res.StatusCode) {
		return res, NewRestHttpError(res, fmt.Sprintf("Unexpected status code %d", res.StatusCode))
	}

	defer res.Body.Close()
	return res, json.NewDecoder(res.Body).Decode(out)
}

func (c *Client) Post(path string, in, out interface{}) (*http.Response, error) {
	buf, err := json.Marshal(in)
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
	res, err := c.HttpClient.Do(req)
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

func (c *Client) ok(status int) bool {
	return status == http.StatusOK || status == http.StatusCreated
}

func (c *Client) buildPaginatedPath(path string, params *config.PaginateParams) (string, error) {
	if params == nil {
		return path, nil
	}

	values, err := params.Values()
	if err != nil {
		return "", err
	}

	queryString := values.Encode()
	if len(queryString) > 0 {
		path = path + "?" + queryString
	}

	return path, nil
}
