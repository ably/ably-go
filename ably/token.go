package ably

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// TokenParams
type TokenParams struct {
	// TTL is a requested time to live for the token. If the token request
	// is successful, the TTL of the returned token will be less than or equal
	// to this value depending on application settings and the attributes
	// of the issuing key.
	TTL int64 `json:"ttl,omitempty" codec:"ttl,omitempty"`

	// Capability represents encoded access rights of the token.
	Capability string `json:"capability,omitempty" codec:"capability,omitempty"`

	// ClientID represents a client, whom the token is generated for.
	ClientID string `json:"clientId,omitempty" codec:"clientId,omitempty"`

	// Timestamp of the token request. It's used, in conjunction with the nonce,
	// are used to prevent token requests from being replayed.
	Timestamp int64 `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
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
	if params.Capability != "" {
		q.Set("capability", params.Capability)
	}
	if params.ClientID != "" {
		q.Set("clientId", params.ClientID)
	}
	if params.Timestamp != 0 {
		q.Set("timestamp", strconv.FormatInt(params.Timestamp, 10))
	}
	return q
}

// TokenRequest
type TokenRequest struct {
	TokenParams `codec:",inline"`

	KeyName string `json:"keyName,omitempty" codec:"keyName,omitempty"`
	Nonce   string `json:"nonce,omitempty" codec:"nonce,omitempty"` // should be at least 16 characters long
	MAC     string `json:"mac,omitempty" codec:"mac,omitempty"`     // message authentication code for the request
}

func (TokenRequest) IsTokener() {}
func (TokenRequest) isTokener() {}

func (req *TokenRequest) sign(secret []byte) {
	mac := hmac.New(sha256.New, secret)
	fmt.Fprintln(mac, req.KeyName)
	fmt.Fprintln(mac, req.TTL)
	fmt.Fprintln(mac, req.Capability)
	fmt.Fprintln(mac, req.ClientID)
	fmt.Fprintln(mac, req.Timestamp)
	fmt.Fprintln(mac, req.Nonce)
	req.MAC = base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

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

	// Capability
	Capability string `json:"capability,omitempty" codec:"capability,omitempty"`
}

func (TokenDetails) IsTokener() {}
func (TokenDetails) isTokener() {}

func (tok *TokenDetails) expired(now time.Time) bool {
	return tok.Expires != 0 && tok.Expires <= unixMilli(now)
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
