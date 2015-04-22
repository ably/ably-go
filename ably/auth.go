package ably

import (
	"fmt"
	"strconv"

	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"github.com/ably/ably-go/Godeps/_workspace/src/github.com/flynn/flynn/pkg/random"
)

// TODO(rjeczalik): add support for :query_time

type Capability map[string][]string

func (c Capability) MarshalJSON() ([]byte, error) {
	if len(c) == 0 {
		return []byte(`""`), nil
	}
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
	Token      string     `json:"token"`
	KeyName    string     `json:"keyName"`
	Expires    int64      `json:"expires"`
	Issued     int64      `json:"issued"`
	Capability Capability `json:"capability"`
}

type TokenRequest struct {
	KeyName    string     `json:"keyName"`
	TTL        int        `json:"ttl"`
	Capability Capability `json:"capability"`
	ClientID   string     `json:"client_id"`
	Timestamp  int64      `json:"timestamp"`
	Nonce      string     `json:"nonce"`
	Mac        string     `json:"mac"`
}

func (req *TokenRequest) sign(secret []byte) {
	// Set defaults.
	if req.Timestamp == 0 {
		req.Timestamp = TimestampNow()
	}
	if req.Nonce == "" {
		req.Nonce = random.String(32)
	}
	if req.Capability == nil {
		req.Capability = Capability{"*": {"*"}}
	}
	if req.TTL == 0 {
		req.TTL = 60 * 60 * 1000
	}

	// Sign.
	mac := hmac.New(sha256.New, secret)
	fmt.Fprintln(mac, req.KeyName)
	fmt.Fprintln(mac, req.TTL)
	fmt.Fprintln(mac, req.Capability.String())
	fmt.Fprintln(mac, req.ClientID)
	fmt.Fprintln(mac, req.Timestamp)
	fmt.Fprintln(mac, req.Nonce)
	req.Mac = base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type Auth struct {
	Options *ClientOptions
	client  *RestClient
}

func NewAuth(options *ClientOptions, client *RestClient) *Auth {
	return &Auth{
		Options: options,
		client:  client,
	}
}

func (a *Auth) CreateTokenRequest() *TokenRequest {
	return &TokenRequest{
		KeyName:  a.Options.Token,
		ClientID: a.Options.ClientID,
	}
}

func (a *Auth) RequestToken(req *TokenRequest) (*Token, error) {
	if req == nil {
		req = a.CreateTokenRequest()
	}
	req.sign([]byte(a.Options.Secret))
	resp := &Token{}
	_, err := a.client.Post("/keys/"+req.KeyName+"/requestToken", req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
