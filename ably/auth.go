package ably

import (
	"fmt"
	"strconv"
	"time"

	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"github.com/ably/ably-go/Godeps/_workspace/src/github.com/flynn/flynn/pkg/random"
)

type Capability map[string][]string

func (c Capability) MarshalJSON() ([]byte, error) {
	p, err := json.Marshal((map[string][]string)(c))
	if err != nil {
		return nil, err
	}
	return []byte(strconv.Quote(string(p))), nil
}

func (c *Capability) UnmarshalJSON(p []byte) error {
	s, err := strconv.Unquote(string(p))
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(s), (*map[string][]string)(c))
}

func (c Capability) String() string {
	p, err := json.Marshal((map[string][]string)(c))
	if err != nil {
		panic(err)
	}
	return string(p)
}

type Token struct {
	ID         string     `json:"id"`
	Key        string     `json:"key"`
	Capability Capability `json:"capability"`
}

type tokenResponse struct {
	AccessToken *Token `json:"access_token"`
}

type TokenRequest struct {
	ID         string     `json:"id"`
	TTL        int        `json:"ttl"`
	Capability Capability `json:"capability"`
	ClientID   string     `json:"client_id"`
	Timestamp  int64      `json:"timestamp"`
	Nonce      string     `json:"nonce"`
	Mac        string     `json:"mac"`
}

func (t *TokenRequest) Sign(secret string) {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintln(mac, t.ID)
	fmt.Fprintln(mac, t.TTL)
	fmt.Fprintln(mac, t.Capability.String())
	fmt.Fprintln(mac, t.ClientID)
	fmt.Fprintln(mac, t.Timestamp)
	fmt.Fprintln(mac, t.Nonce)
	t.Mac = base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type Auth struct {
	Params
	client *RestClient
}

func NewAuth(params Params, client *RestClient) *Auth {
	return &Auth{
		Params: params,
		client: client,
	}
}

func (a *Auth) CreateTokenRequest(ttl int, capability Capability) *TokenRequest {
	req := &TokenRequest{
		ID:         a.AppID,
		TTL:        ttl,
		Capability: capability,
		ClientID:   a.ClientID,
		Timestamp:  time.Now().Unix(),
		Nonce:      random.String(32),
	}

	req.Sign(a.AppSecret)

	return req
}

func (a *Auth) RequestToken(ttl int, capability Capability) (*Token, error) {
	req := a.CreateTokenRequest(ttl, capability)

	res := &tokenResponse{}
	_, err := a.client.Post("/keys/"+a.AppID+"/requestToken", req, res)
	if err != nil {
		return nil, err
	}
	return res.AccessToken, nil
}
