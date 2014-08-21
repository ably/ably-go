package ably

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Params struct {
	Endpoint  string
	AppID     string
	AppSecret string
}

func RestClient(params Params) *Client {
	return &Client{
		Params:   params,
		channels: make(map[string]*Channel),
	}
}

type Client struct {
	Params

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

	ch := &Channel{Name: name, client: c}
	c.channels[name] = ch
	return ch
}

func (c *Client) Get(path string, data interface{}) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.Endpoint+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.AppID, c.AppSecret)
	res, err := http.DefaultClient.Do(req)
	defer res.Body.Close()
	if !c.ok(res.StatusCode) {
		return res, fmt.Errorf("Unexpected status code %d", res.StatusCode)
	}
	return res, json.NewDecoder(res.Body).Decode(data)
}

func (c *Client) Post(path string, in, out interface{}) (*http.Response, error) {
	buf, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.Endpoint+path, bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.AppID, c.AppSecret)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if out != nil && c.ok(res.StatusCode) {
		defer res.Body.Close()
		return res, json.NewDecoder(res.Body).Decode(out)
	}
	if !c.ok(res.StatusCode) {
		return res, fmt.Errorf("Unexpected status code %d", res.StatusCode)
	}
	return res, nil
}

func (c *Client) Delete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.Endpoint+path, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.AppID, c.AppSecret)
	res, err := http.DefaultClient.Do(req)
	defer res.Body.Close()
	if !c.ok(res.StatusCode) {
		return res, fmt.Errorf("Unexpected status code %d", res.StatusCode)
	}
	return res, nil
}

func (c *Client) ok(status int) bool {
	return status == http.StatusOK || status == http.StatusCreated
}
