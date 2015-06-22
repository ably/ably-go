package ably

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"encoding/base64"

	"github.com/ably/ably-go/Godeps/_workspace/src/github.com/flynn/flynn/pkg/random"
)

var (
	errMissingKey          = errors.New("missing key")
	errInvalidKey          = errors.New("invalid key")
	errMissingTokenOpts    = errors.New("missing options for token authentication")
	errMismatchedKeys      = errors.New("mismatched keys")
	errUnsupportedType     = errors.New("unsupported Content-Type header in response from AuthURL")
	errMissingType         = errors.New("missing Content-Type header in response from AuthURL")
	errInvalidCallbackType = errors.New("invalid value type returned from AuthCallback")
)

// addParams copies each params from rhs to lhs and returns lhs.
//
// If param from rhs exists in lhs, it's omitted.
func addParams(lhs, rhs url.Values) url.Values {
	for key := range rhs {
		if lhs.Get(key) != "" {
			continue
		}
		lhs.Set(key, rhs.Get(key))
	}
	return lhs
}

// addHeaders copies each header from rhs to lhs and returns lhs.
//
// If header from rhs exists in lhs, it's omitted.
func addHeaders(lhs, rhs http.Header) http.Header {
	for key := range rhs {
		if lhs.Get(key) != "" {
			continue
		}
		lhs.Set(key, rhs.Get(key))
	}
	return lhs
}

// Auth
type Auth struct {
	Method AuthMethod

	client *RestClient
}

func newAuth(client *RestClient) (*Auth, error) {
	a := &Auth{client: client}
	method, err := detectAuthMethod(a.opts())
	if err != nil {
		return nil, err
	}
	a.Method = method
	return a, nil
}

// CreateTokenRequest
func (a *Auth) CreateTokenRequest(opts *AuthOptions, params *TokenParams) (*TokenRequest, error) {
	opts = a.mergeOpts(opts)
	keyName, keySecret := opts.KeyName(), opts.KeySecret()
	req := &TokenRequest{}
	if params != nil {
		req.TokenParams = *params
	}
	if req.KeyName == "" {
		req.KeyName = keyName
	}
	if err := a.setDefaults(opts, req); err != nil {
		return nil, err
	}
	// Validate arguments.
	switch {
	case opts.Key == "":
		return nil, newError(40101, errMissingKey)
	case keyName == "" || keySecret == "":
		return nil, newError(40102, errInvalidKey)
	case req.KeyName != keyName:
		return nil, newError(40102, errMismatchedKeys)
	}
	req.sign([]byte(keySecret))
	return req, nil
}

// RequestToken
func (a *Auth) RequestToken(opts *AuthOptions, params *TokenParams) (*TokenDetails, error) {
	opts = a.mergeOpts(opts)
	var tokReq *TokenRequest
	switch {
	case opts.AuthCallback != nil:
		v, err := opts.AuthCallback(params)
		if err != nil {
			return nil, newError(40170, err)
		}
		switch v := v.(type) {
		case *TokenRequest:
			tokReq = v
		case *TokenDetails:
			return v, nil
		case string:
			return newTokenDetails(v), nil
		default:
			return nil, newError(40170, errInvalidCallbackType)
		}
	case opts.AuthURL != "":
		return a.requestAuthURL(opts, params)
	default:
		req, err := a.CreateTokenRequest(opts, params)
		if err != nil {
			return nil, err
		}
		tokReq = req
	}
	token := &TokenDetails{}
	r := &request{
		Method: "POST",
		Path:   "/keys/" + tokReq.KeyName + "/requestToken",
		In:     tokReq,
		Out:    token,
		NoAuth: true,
	}
	if _, err := a.client.do(r); err != nil {
		return nil, err
	}
	return token, nil
}

// Authorise
func (a *Auth) Authorise(opts *AuthOptions, params *TokenParams, force bool) (*TokenDetails, error) {
	if tok := a.token(); tok != nil && !tok.Expired() && !force {
		return tok, nil
	}
	a.setToken(nil) // unset token when it's expired or when force is true
	tok, err := a.RequestToken(opts, params)
	if err != nil {
		return nil, err
	}
	a.setToken(tok)
	a.Method = AuthToken
	return tok, nil
}

func (a *Auth) mergeOpts(opts *AuthOptions) *AuthOptions {
	if opts == nil {
		opts = &a.opts().AuthOptions
	} else {
		opts.merge(&a.opts().AuthOptions, false)
	}
	return opts
}

func detectAuthMethod(opts *ClientOptions) (AuthMethod, error) {
	useTokenAuth := opts.UseTokenAuth || opts.ClientID != ""
	isKeyValid := opts.KeyName() != "" && opts.KeySecret() != ""
	isAuthExternal := opts.externalTokenAuthSupported()
	switch {
	case !isAuthExternal && !useTokenAuth:
		if !isKeyValid {
			return 0, newError(40005, errInvalidKey)
		}
		if opts.NoTLS {
			return 0, newError(40103, nil)
		}
		return AuthBasic, nil
	case isAuthExternal || isKeyValid:
		return AuthToken, nil
	default:
		return 0, newError(40102, errMissingTokenOpts)
	}
}

func (a *Auth) setDefaults(opts *AuthOptions, req *TokenRequest) error {
	if req.Nonce == "" {
		req.Nonce = random.String(32)
	}
	if req.RawCapability == "" {
		req.RawCapability = (Capability{"*": {"*"}}).Encode()
	}
	if req.TTL == 0 {
		req.TTL = 60 * 60 * 1000
	}
	if req.ClientID == "" {
		req.ClientID = a.opts().ClientID
	}
	if req.Timestamp == 0 {
		if opts.UseQueryTime {
			t, err := a.client.Time()
			if err != nil {
				return newError(40100, err)
			}
			req.Timestamp = Timestamp(t)
		} else {
			req.Timestamp = TimestampNow()
		}
	}
	return nil
}

func (a *Auth) requestAuthURL(opts *AuthOptions, params *TokenParams) (*TokenDetails, error) {
	req, err := http.NewRequest(opts.authMethod(), opts.AuthURL, nil)
	if err != nil {
		return nil, newError(40000, err)
	}
	req.URL.RawQuery = addParams(params.Query(), opts.AuthParams).Encode()
	req.Header = addHeaders(req.Header, opts.AuthHeaders)
	resp, err := a.opts().httpclient().Do(req)
	if err != nil {
		return nil, newError(40000, err)
	}
	if err = checkValidHTTPResponse(resp); err != nil {
		return nil, newError(40000, err)
	}
	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	if i := strings.IndexRune(contentType, ';'); i != -1 {
		contentType = contentType[:i]
	}
	switch contentType {
	case "text/plain":
		token, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, newError(40000, err)
		}
		return newTokenDetails(string(token)), nil
	case ProtocolJSON, ProtocolMsgPack:
		var token TokenDetails
		if err := decode(contentType, resp.Body, &token); err != nil {
			return nil, newError(40000, err)
		}
		return &token, nil
	case "":
		return nil, newError(40000, errMissingType)
	default:
		return nil, newError(40000, errUnsupportedType)
	}
}

func (a *Auth) isTokenRenewable() bool {
	return (a.opts().Key != "" && a.opts().Token == "") ||
		a.opts().AuthURL != "" || a.opts().AuthCallback != nil
}

func (a *Auth) authReq(req *http.Request) error {
	switch a.Method {
	case AuthBasic:
		req.SetBasicAuth(a.opts().KeyName(), a.opts().KeySecret())
	case AuthToken:
		if _, err := a.Authorise(nil, nil, false); err != nil {
			return err
		}
		encToken := base64.StdEncoding.EncodeToString([]byte(a.token().Token))
		req.Header.Set("Authorization", "Bearer "+encToken)
	}
	return nil
}

func (a *Auth) authQuery(query url.Values) error {
	switch a.Method {
	case AuthBasic:
		query.Set("key", a.opts().Key)
	case AuthToken:
		if _, err := a.Authorise(nil, nil, false); err != nil {
			return err
		}
		query.Set("access_token", a.token().Token)
	}
	return nil
}

func (a *Auth) opts() *ClientOptions {
	return &a.client.options
}

func (a *Auth) token() *TokenDetails {
	return a.client.options.TokenDetails
}

func (a *Auth) setToken(tok *TokenDetails) {
	a.client.options.TokenDetails = tok
}
