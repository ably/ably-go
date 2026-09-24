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

// TokenParams contains token params to be sent to ably to get auth token
type TokenParams struct {
	// TTL is a requested time to live for the token in milliseconds. If the token request
	// is successful, the TTL of the returned token will be less than or equal
	// to this value depending on application settings and the attributes
	// of the issuing key.
	// The default is 60 minutes (RSA9e, TK2a).
	TTL int64 `json:"ttl,omitempty" codec:"ttl,omitempty"`

	// Capability represents encoded channel access rights associated with this Ably Token.
	// The capabilities value is a JSON-encoded representation of the resource paths and associated operations.
	// Read more about capabilities in the [capabilities docs].
	// default '{"*":["*"]}' (RSA9f, TK2b)
	//
	// [capabilities docs]: https://ably.com/docs/core-features/authentication/#capabilities-explained
	Capability string `json:"capability,omitempty" codec:"capability,omitempty"`

	// ClientID is used for identifying this client when publishing messages or for presence purposes.
	// The clientId can be any non-empty string, except it cannot contain a *. This option is primarily intended
	// to be used in situations where the library is instantiated with a key.
	// Note that a clientId may also be implicit in a token used to instantiate the library.
	// An error is raised if a clientId specified here conflicts with the clientId implicit in the token.
	// Find out more about [identified clients] (TK2c).
	//
	// [identified clients]: https://ably.com/docs/core-features/authentication#identified-clients
	ClientID string `json:"clientId,omitempty" codec:"clientId,omitempty"`

	// Timestamp of the token request as milliseconds since the Unix epoch.
	// Timestamps, in conjunction with the nonce, are used to prevent requests from being replayed.
	// timestamp is a "one-time" value, and is valid in a request, but is not validly a member of
	// any default token params such as ClientOptions.defaultTokenParams (RSA9d, Tk2d).
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

// TokenRequest contains tokenparams with extra details, sent to ably for getting auth token
type TokenRequest struct {
	TokenParams `codec:",inline"`

	// KeyName is the name of the key against which this request is made.
	// The key name is public, whereas the key secret is private (TE2).
	KeyName string `json:"keyName,omitempty" codec:"keyName,omitempty"`

	// Nonce is a cryptographically secure random string of at least 16 characters,
	// used to ensure the TokenRequest cannot be reused (TE2).
	Nonce string `json:"nonce,omitempty" codec:"nonce,omitempty"`

	// MAC is the Message Authentication Code for this request.
	MAC string `json:"mac,omitempty" codec:"mac,omitempty"`
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

// TokenDetails contains an Ably Token and its associated metadata.
type TokenDetails struct {

	// Token is the ably Token itself (TD2).
	// A typical Ably Token string appears with the form xVLyHw.A-pwh7wicf3afTfgiw4k2Ku33kcnSA7z6y8FjuYpe3QaNRTEo4.
	Token string `json:"token,omitempty" codec:"token,omitempty"`

	// KeyName is a string part of ABLY_KEY before :
	KeyName string `json:"keyName,omitempty" codec:"keyName,omitempty"`

	// Expires is the timestamp at which this token expires as milliseconds since the Unix epoch (TD3).
	Expires int64 `json:"expires,omitempty" codec:"expires,omitempty"`

	// ClientID, if any, bound to this Ably Token. If a client ID is included, then the Ably Token authenticates
	// its bearer as that client ID, and the Ably Token may only be used to perform operations on behalf
	// of that client ID. The client is then considered to be an identified client (TD6).
	ClientID string `json:"clientId,omitempty" codec:"clientId,omitempty"`

	// Issued is the timestamp at which this token was issued as milliseconds since the Unix epoch.
	Issued int64 `json:"issued,omitempty" codec:"issued,omitempty"`

	// Capability is the capabilities associated with this Ably Token.
	// The capabilities value is a JSON-encoded representation of the resource paths and associated operations.
	// Read more about capabilities in the [capabilities docs] (TD5).
	//
	// [capabilities docs]: https://ably.com/docs/core-features/authentication/#capabilities-explained
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
