package ably

import (
	"fmt"
	"strconv"

	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"github.com/ably/ably-go/Godeps/_workspace/src/github.com/flynn/flynn/pkg/random"
	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
)

type Capability map[string][]string

// Ensure Capability implements JSON/msgpack {,un}marshallers.
var (
	_ json.Marshaler      = (*Capability)(nil)
	_ json.Unmarshaler    = (*Capability)(nil)
	_ msgpack.Marshaler   = (*Capability)(nil)
	_ msgpack.Unmarshaler = (*Capability)(nil)
)

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

func (c Capability) MarshalMsgpack() ([]byte, error) {
	return msgpack.Marshal(c.String())
}

func (c *Capability) UnmarshalMsgpack(p []byte) error {
	var s string
	if err := msgpack.Unmarshal(p, &s); err != nil {
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
	Token      string     `json:"token" msgpack:"token"`
	KeyName    string     `json:"keyName" msgpack:"keyName"`
	Expires    int64      `json:"expires" msgpack:"expires"`
	Issued     int64      `json:"issued" msgpack:"issued"`
	Capability Capability `json:"capability" msgpack:"capability"`
}

var _ msgpack.Unmarshaler = (*Token)(nil)

func (tok *Token) UnmarshalMsgpack(p []byte) error {
	// This method is workaround for msgpack decoder, which tries to decode fields
	// using its original type and (finding field byte boundary) and then passes
	// the bytes to custom unmarshaller.
	//
	// Decoding *Token directly fails with:
	//
	//   msgpack: invalid code ab decoding map length
	//
	v := struct {
		Token      string `msgpack:"token"`
		KeyName    string `msgpack:"keyName"`
		Expires    int64  `msgpack:"expires"`
		Issued     int64  `msgpack:"issued"`
		Capability string `msgpack:"capability"`
	}{}
	if err := msgpack.Unmarshal(p, &v); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(v.Capability), (*map[string][]string)(&tok.Capability)); err != nil {
		return err
	}
	tok.Token = v.Token
	tok.KeyName = v.KeyName
	tok.Expires = v.Expires
	tok.Issued = v.Issued
	return nil
}

type TokenRequest struct {
	KeyName    string     `json:"keyName" msgpack:"keyName"`
	TTL        int        `json:"ttl" msgpack:"ttl"`
	Capability Capability `json:"capability" msgpack:"capability"`
	ClientID   string     `json:"clientId" msgpack:"clientId"`
	Timestamp  int64      `json:"timestamp" msgpack:"timestamp"`
	Nonce      string     `json:"nonce" msgpack:"nonce"`
	Mac        string     `json:"mac" msgpack:"mac"`
}

var _ msgpack.Unmarshaler = (*TokenRequest)(nil)

func (req *TokenRequest) UnmarshalMsgpack(p []byte) error {
	// This method is workaround for msgpack decoder, which tries to decode fields
	// using its original type and (finding field byte boundary) and then passes
	// the bytes to custom unmarshaller.
	//
	// Decoding *TokenRequest directly fails with:
	//
	//   msgpack: invalid code ab decoding map length
	//
	v := struct {
		KeyName    string `msgpack:"keyName"`
		TTL        int    `msgpack:"ttl"`
		Capability string `msgpack:"capability"`
		ClientID   string `msgpack:"clientId"`
		Timestamp  int64  `msgpack:"timestamp"`
		Nonce      string `msgpack:"nonce"`
		Mac        string `msgpack:"mac"`
	}{}
	if err := msgpack.Unmarshal(p, &v); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(v.Capability), (*map[string][]string)(&req.Capability)); err != nil {
		return err
	}
	req.KeyName = v.KeyName
	req.TTL = v.TTL
	req.ClientID = v.ClientID
	req.Timestamp = v.Timestamp
	req.Nonce = v.Nonce
	req.Mac = v.Mac
	return nil
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
	options   ClientOptions
	client    *RestClient
	keyName   string
	keySecret string
}

func (a *Auth) CreateTokenRequest() *TokenRequest {
	return &TokenRequest{
		KeyName:  a.keyName,
		ClientID: a.options.ClientID,
	}
}

func (a *Auth) RequestToken(req *TokenRequest) (*Token, error) {
	if req == nil {
		req = a.CreateTokenRequest()
	}
	req.sign([]byte(a.keySecret))
	token := &Token{}
	_, err := a.client.Post("/keys/"+req.KeyName+"/requestToken", req, token)
	if err != nil {
		return nil, err
	}
	return token, nil
}
