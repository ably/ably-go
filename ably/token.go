package ably

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Capability
type Capability map[string][]string

// ParseCapability
func ParseCapability(capability string) (c Capability, err error) {
	c = make(Capability)
	err = json.Unmarshal([]byte(capability), &c)
	return
}

// Encode
func (c Capability) Encode() string {
	if len(c) == 0 {
		return ""
	}
	p, err := json.Marshal((map[string][]string)(c))
	if err != nil {
		panic(err)
	}
	return string(p)
}

type TokenParamsV12 = TokenParams

// TokenParams
type TokenParams struct {
	// TTL is a requested time to live for the token. If the token request
	// is successful, the TTL of the returned token will be less than or equal
	// to this value depending on application settings and the attributes
	// of the issuing key.
	TTL int64 `json:"ttl,omitempty" codec:"ttl,omitempty"`

	// RawCapability represents encoded access rights of the token.
	RawCapability string `json:"capability,omitempty" codec:"capability,omitempty"`

	// ClientID represents a client, whom the token is generated for.
	ClientID string `json:"clientId,omitempty" codec:"clientId,omitempty"`

	// Timestamp of the token request. It's used, in conjunction with the nonce,
	// are used to prevent token requests from being replayed.
	Timestamp int64 `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
}

// Capability
func (params *TokenParams) Capability() Capability {
	c, _ := ParseCapability(params.RawCapability)
	return c
}

// Query encodes the params to query params value. If a field of params is
// a zero-value, it's omitted. If params is zero-value, nil is returned.
func (params *TokenParams) Query() url.Values {
	q := make(url.Values)
	if params == nil {
		return q
	}
	if params.TTL != 0 {
		q.Set("ttl", strconv.FormatInt(params.TTL, 10))
	}
	if params.RawCapability != "" {
		q.Set("capability", params.RawCapability)
	}
	if params.ClientID != "" {
		q.Set("clientId", params.ClientID)
	}
	if params.Timestamp != 0 {
		q.Set("timestamp", strconv.FormatInt(params.Timestamp, 10))
	}
	return q
}

type TokenRequestV12 = TokenRequest

// TokenRequest
type TokenRequest struct {
	TokenParams `codec:",inline"`

	KeyName string `json:"keyName,omitempty" codec:"keyName,omitempty"`
	Nonce   string `json:"nonce,omitempty" codec:"nonce,omitempty"` // should be at least 16 characters long
	Mac     string `json:"mac,omitempty" codec:"mac,omitempty"`     // message authentication code for the request
}

func (TokenRequestV12) IsTokenLikeV12() {}
func (TokenRequestV12) isTokenLikeV12() {}

func (req *TokenRequest) sign(secret []byte) {
	mac := hmac.New(sha256.New, secret)
	fmt.Fprintln(mac, req.KeyName)
	fmt.Fprintln(mac, req.TTL)
	fmt.Fprintln(mac, req.RawCapability)
	fmt.Fprintln(mac, req.ClientID)
	fmt.Fprintln(mac, req.Timestamp)
	fmt.Fprintln(mac, req.Nonce)
	req.Mac = base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type TokenDetailsV12 = TokenDetails

// TokenDetails
type TokenDetails struct {
	// Token
	Token string `json:"token,omitempty" codec:"token,omitempty"`

	// KeyName
	KeyName string `json:"keyName,omitempty" codec:"keyName,omitempty"`

	// Expires
	Expires int64 `json:"expires,omitempty" codec:"expires,omitempty"`

	// ClientID
	ClientID string `json:"clientId,omitempty" codec:"clientId,omitempty"`

	// Issued
	Issued int64 `json:"issued,omitempty" codec:"issued,omitempty"`

	// RawCapability
	RawCapability string `json:"capability,omitempty" codec:"capability,omitempty"`
}

func (TokenDetailsV12) IsTokenLikeV12() {}
func (TokenDetailsV12) isTokenLikeV12() {}

// Capability
func (tok *TokenDetails) Capability() Capability {
	c, _ := ParseCapability(tok.RawCapability)
	return c
}

// Expired
func (tok *TokenDetails) Expired() bool {
	return tok.Expires != 0 && tok.Expires <= TimeNow()
}

func (tok *TokenDetails) IssueTime() time.Time {
	return time.Unix(tok.Issued/1000, tok.Issued%1000*int64(time.Millisecond))
}

func (tok *TokenDetails) ExpireTime() time.Time {
	return time.Unix(tok.Expires/1000, tok.Expires%1000*int64(time.Millisecond))
}

func newTokenDetails(token string) *TokenDetails {
	return &TokenDetails{
		Token: token,
	}
}
