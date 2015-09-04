package ably

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"

	"encoding/base64"
)

var (
	errMissingKey          = errors.New("missing key")
	errInvalidKey          = errors.New("invalid key")
	errMissingTokenOpts    = errors.New("missing options for token authentication")
	errMismatchedKeys      = errors.New("mismatched keys")
	errUnsupportedType     = errors.New("unsupported Content-Type header in response from AuthURL")
	errMissingType         = errors.New("missing Content-Type header in response from AuthURL")
	errInvalidCallbackType = errors.New("invalid value type returned from AuthCallback")
	errInsecureBasicAuth   = errors.New("basic auth is not supported on insecure non-TLS connections")
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
	params *TokenParams // save params to use with token renewal
}

func newAuth(client *RestClient) (*Auth, error) {
	a := &Auth{client: client}
	method, err := detectAuthMethod(a.opts())
	if err != nil {
		return nil, err
	}
	a.Method = method
	if a.opts().Token != "" {
		a.setToken(newTokenDetails(a.opts().Token))
	}
	return a, nil
}

// CreateTokenRequest
func (a *Auth) CreateTokenRequest(opts *AuthOptions, params *TokenParams) (*TokenRequest, error) {
	opts = a.mergeOpts(opts)
	keySecret := opts.KeySecret()
	req := &TokenRequest{KeyName: opts.KeyName()}
	if params != nil {
		req.TokenParams = *params
	}
	if err := a.setDefaults(opts, req); err != nil {
		return nil, err
	}
	// Validate arguments.
	switch {
	case opts.Key == "":
		return nil, newError(40101, errMissingKey)
	case req.KeyName == "" || keySecret == "":
		return nil, newError(40102, errInvalidKey)
	}
	req.sign([]byte(keySecret))
	return req, nil
}

// RequestToken
func (a *Auth) RequestToken(opts *AuthOptions, params *TokenParams) (*TokenDetails, error) {
	switch {
	case opts != nil && opts.Token != "":
		tok := newTokenDetails(opts.Token)
		a.setToken(tok)
		return tok, nil
	case opts != nil && opts.TokenDetails != nil:
		a.setToken(opts.TokenDetails)
		return opts.TokenDetails, nil
	}
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
		res, err := a.requestAuthURL(opts, params)
		if err != nil {
			return nil, err
		}
		switch res := res.(type) {
		case *TokenDetails:
			return res, nil
		case *TokenRequest:
			tokReq = res
		}
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
	if tok := a.token(); tok != nil && !force && (tok.Expires == 0 || !tok.Expired()) {
		return tok, nil
	}
	tok, err := a.RequestToken(opts, params)
	if err != nil {
		return nil, err
	}
	a.setToken(tok)
	a.params = params
	a.Method = AuthToken
	return tok, nil
}

func (a *Auth) reauthorise(force bool) (*TokenDetails, error) {
	return a.Authorise(nil, a.params, force)
}

func (a *Auth) mergeOpts(opts *AuthOptions) *AuthOptions {
	if opts == nil {
		opts = &a.opts().AuthOptions
	} else {
		opts.merge(&a.opts().AuthOptions, false)
	}
	return opts
}

func (a *Auth) setDefaults(opts *AuthOptions, req *TokenRequest) error {
	if req.Nonce == "" {
		req.Nonce = randomString(32)
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
			req.Timestamp = Time(t)
		} else {
			req.Timestamp = TimeNow()
		}
	}
	return nil
}

func (a *Auth) requestAuthURL(opts *AuthOptions, params *TokenParams) (interface{}, error) {
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
	typ, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, newError(40004, err)
	}
	switch typ {
	case "text/plain":
		token, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, newError(40000, err)
		}
		return newTokenDetails(string(token)), nil
	case ProtocolJSON, ProtocolMsgPack:
		var req TokenRequest
		var buf bytes.Buffer
		err := decode(typ, io.TeeReader(resp.Body, &buf), &req)
		if err == nil && req.Mac != "" && req.Nonce != "" {
			return &req, nil
		}
		var token TokenDetails
		if err := decode(typ, io.MultiReader(&buf, resp.Body), &token); err != nil {
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
	return a.opts().Key != "" || a.opts().AuthURL != "" || a.opts().AuthCallback != nil
}

func (a *Auth) authReq(req *http.Request) error {
	switch a.Method {
	case AuthBasic:
		req.SetBasicAuth(a.opts().KeyName(), a.opts().KeySecret())
	case AuthToken:
		if _, err := a.reauthorise(false); err != nil {
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
		if _, err := a.reauthorise(false); err != nil {
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
			return 0, newError(40103, errInsecureBasicAuth)
		}
		return AuthBasic, nil
	case isAuthExternal || isKeyValid:
		return AuthToken, nil
	default:
		return 0, newError(40102, errMissingTokenOpts)
	}
}
