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

// **LEGACY**
// TokenParams
type TokenParams struct {
	// **LEGACY**
	// TTL is a requested time to live for the token. If the token request
	// is successful, the TTL of the returned token will be less than or equal
	// to this value depending on application settings and the attributes
	// of the issuing key.
	// **CANONICAL**
	// Default 60 min
	// Requested time to live for the token in milliseconds. The default is 60 minutes.
	// RSA9e, TK2a
	TTL int64 `json:"ttl,omitempty" codec:"ttl,omitempty"`

	// **LEGACY**
	// Capability represents encoded access rights of the token.
	// **CANONICAL**
	// default '{"*":["*"]}'
	// The capabilities associated with this Ably Token. The capabilities value is a JSON-encoded representation of the resource paths and associated operations. Read more about capabilities in the capabilities docs.
	// RSA9f, TK2b
	Capability string `json:"capability,omitempty" codec:"capability,omitempty"`

	// **LEGACY**
	// ClientID represents a client, whom the token is generated for.
	// **CANONICAL**
	// A client ID, used for identifying this client when publishing messages or for presence purposes. The clientId can be any non-empty string, except it cannot contain a *. This option is primarily intended to be used in situations where the library is instantiated with a key. Note that a clientId may also be implicit in a token used to instantiate the library. An error is raised if a clientId specified here conflicts with the clientId implicit in the token. Find out more about identified clients.
	// TK2c
	ClientID string `json:"clientId,omitempty" codec:"clientId,omitempty"`

	// **LEGACY**
	// Timestamp of the token request. It's used, in conjunction with the nonce,
	// are used to prevent token requests from being replayed.
	// **CANONICAL**
	// The timestamp of this request as milliseconds since the Unix epoch. Timestamps, in conjunction with the nonce, are used to prevent requests from being replayed. timestamp is a "one-time" value, and is valid in a request, but is not validly a member of any default token params such as ClientOptions.defaultTokenParams.
	// RSA9d, Tk2d
	Timestamp int64 `json:"timestamp,omitempty" codec:"timestamp,omitempty"`
}

// **LEGACY**
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

// **LEGACY**
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

// **LEGACY**
// TokenDetails
// **CANONICAL**
// Contains an Ably Token and its associated metadata.
type TokenDetails struct {
	// **LEGACY**
	// Token
	// **CANONICAL**
	// The Ably Token itself. A typical Ably Token string appears with the form xVLyHw.A-pwh7wicf3afTfgiw4k2Ku33kcnSA7z6y8FjuYpe3QaNRTEo4.
	// TD2
	Token string `json:"token,omitempty" codec:"token,omitempty"`

	// **LEGACY**
	// KeyName
	KeyName string `json:"keyName,omitempty" codec:"keyName,omitempty"`

	// **LEGACY**
	// Expires
	// **CANONICAL**
	// The timestamp at which this token expires as milliseconds since the Unix epoch.
	// TD3
	Expires int64 `json:"expires,omitempty" codec:"expires,omitempty"`

	// **LEGACY**
	// ClientID
	// **CANONICAL**
	// The client ID, if any, bound to this Ably Token. If a client ID is included, then the Ably Token authenticates its bearer as that client ID, and the Ably Token may only be used to perform operations on behalf of that client ID. The client is then considered to be an identified client.
	// TD6
	ClientID string `json:"clientId,omitempty" codec:"clientId,omitempty"`

	// **LEGACY**
	// Issued
	// **CANONICAL**
	// The timestamp at which this token was issued as milliseconds since the Unix epoch.
	Issued int64 `json:"issued,omitempty" codec:"issued,omitempty"`

	// **LEGACY**
	// Capability
	// **CANONICAL**
	// The capabilities associated with this Ably Token. The capabilities value is a JSON-encoded representation of the resource paths and associated operations. Read more about capabilities in the capabilities docs.
	// TD5
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
