package rest

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	_ "crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ably/ably-go"

	"github.com/flynn/flynn/pkg/random"
)

func NewClient(params ably.Params) *Client {
	return &Client{
		HttpClient: http.DefaultClient,
		Params:     params,
		channels:   make(map[string]*Channel),
	}
}

type Client struct {
	ably.Params

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

	ch := &Channel{Name: name, client: c}
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

type Token struct {
	ID         string           `json:"id"`
	Key        string           `json:"key"`
	Capability *ably.Capability `json:"capability"`
}

type tokenRequest struct {
	ID         string `json:"id"`
	TTL        int    `json:"ttl"`
	Capability string `json:"capability"`
	ClientID   string `json:"client_id"`
	Timestamp  int64  `json:"timestamp"`
	Nonce      string `json:"nonce"`
	Mac        string `json:"mac"`
}

func (t *tokenRequest) Sign(secret string) {
	params := []string{
		t.ID,
		strconv.Itoa(t.TTL),
		t.Capability,
		t.ClientID,
		strconv.Itoa(int(t.Timestamp)),
		t.Nonce,
	}
	s := strings.Join(params, "\n") + "\n"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(s))

	t.Mac = base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type tokenResponse struct {
	AccessToken *Token `json:"access_token"`
}

func (c *Client) RequestToken(ttl int, cap *ably.Capability) (*Token, error) {
	req := &tokenRequest{
		ID:         c.AppID,
		TTL:        ttl,
		Capability: cap.String(),
		ClientID:   c.ClientID,
		Timestamp:  time.Now().Unix(),
		Nonce:      random.String(32),
	}
	req.Sign(c.AppSecret)

	res := &tokenResponse{}
	_, err := c.Post("/keys/"+c.AppID+"/requestToken", req, res)
	if err != nil {
		return nil, err
	}
	return res.AccessToken, nil
}

func (c *Client) Get(path string, data interface{}) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.RestEndpoint+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.AppID, c.AppSecret)
	res, err := c.HttpClient.Do(req)
	if !c.ok(res.StatusCode) {
		return res, &RestHttpError{
			Msg:      fmt.Sprintf("Unexpected status code %d", res.StatusCode),
			Response: res,
		}
	}
	defer res.Body.Close()
	return res, json.NewDecoder(res.Body).Decode(data)
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
	req.SetBasicAuth(c.AppID, c.AppSecret)
	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if !c.ok(res.StatusCode) {
		return res, &RestHttpError{
			Msg:      fmt.Sprintf("Unexpected status code %d", res.StatusCode),
			Response: res,
		}
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
