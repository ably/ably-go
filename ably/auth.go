package ably

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

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
	errWildcardClientID    = errors.New("provided ClientID must not be a wildcard")
	errClientIDMismatch    = errors.New("the received ClientID does not match the requested one")
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
	mtx      sync.Mutex
	method   int
	client   *RestClient
	params   *TokenParams // save params to use with token renewal
	host     string       // a host part of AuthURL
	clientID string       // clientID of the authenticated user or wildcard "*"
}

func newAuth(client *RestClient) (*Auth, error) {
	a := &Auth{
		client: client,
	}
	method, err := detectAuthMethod(a.opts())
	if err != nil {
		return nil, err
	}
	if a.opts().AuthURL != "" {
		u, err := url.Parse(a.opts().AuthURL)
		if err != nil {
			return nil, newError(40003, err)
		}
		a.host = u.Host
	}
	a.method = method
	if a.opts().Token != "" {
		a.opts().TokenDetails = newTokenDetails(a.opts().Token)
	}
	a.clientID = a.opts().ClientID
	if a.clientID == "*" {
		return nil, newError(40102, errWildcardClientID)
	}
	return a, nil
}

// ClientID
func (a *Auth) ClientID() string {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	if a.clientID != "*" {
		return a.clientID
	}
	return ""
}

func (a *Auth) clientIDForCheck() string {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	if a.method == authBasic {
		return "*" // for Basic Auth no ClientID check is performed
	}
	return a.clientID
}

func (a *Auth) updateClientID(clientID string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	if clientID != "*" && clientID != "" {
		a.clientID = clientID
	}
}

// CreateTokenRequest
func (a *Auth) CreateTokenRequest(params *TokenParams, opts *AuthOptions) (*TokenRequest, error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.createTokenRequest(params, opts)
}

func (a *Auth) createTokenRequest(params *TokenParams, opts *AuthOptions) (*TokenRequest, error) {
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
func (a *Auth) RequestToken(params *TokenParams, opts *AuthOptions) (*TokenDetails, error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	tok, _, err := a.requestToken(params, opts)
	return tok, err
}

func (a *Auth) requestToken(params *TokenParams, opts *AuthOptions) (tok *TokenDetails, tokReqClientID string, err error) {
	switch {
	case opts != nil && opts.Token != "":
		return newTokenDetails(opts.Token), "", nil
	case opts != nil && opts.TokenDetails != nil:
		return opts.TokenDetails, "", nil
	}
	opts = a.mergeOpts(opts)
	var tokReq *TokenRequest
	switch {
	case opts.AuthCallback != nil:
		v, err := opts.AuthCallback(params)
		if err != nil {
			return nil, "", newError(40170, err)
		}
		switch v := v.(type) {
		case *TokenRequest:
			tokReq = v
			tokReqClientID = tokReq.ClientID
		case *TokenDetails:
			return v, "", nil
		case string:
			return newTokenDetails(v), "", nil
		default:
			return nil, "", newError(40170, errInvalidCallbackType)
		}
	case opts.AuthURL != "":
		res, err := a.requestAuthURL(params, opts)
		if err != nil {
			return nil, "", err
		}
		switch res := res.(type) {
		case *TokenDetails:
			return res, "", nil
		case *TokenRequest:
			tokReq = res
			tokReqClientID = tokReq.ClientID
		}
	default:
		req, err := a.createTokenRequest(params, opts)
		if err != nil {
			return nil, "", err
		}
		tokReq = req
	}
	tok = &TokenDetails{}
	r := &Request{
		Method: "POST",
		Path:   "/keys/" + tokReq.KeyName + "/requestToken",
		In:     tokReq,
		Out:    tok,
		NoAuth: true,
	}
	if _, err := a.client.do(r); err != nil {
		return nil, "", err
	}
	return tok, tokReqClientID, nil
}

// Authorise
func (a *Auth) Authorise(params *TokenParams, opts *AuthOptions) (*TokenDetails, error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	force := a.opts().Force
	if opts != nil && opts.Force {
		force = true
	}
	return a.authorise(params, opts, force)
}

func (a *Auth) authorise(params *TokenParams, opts *AuthOptions, force bool) (*TokenDetails, error) {
	switch tok := a.token(); {
	case tok != nil && !force && (tok.Expires == 0 || !tok.Expired()):
		return tok, nil
	case params != nil && params.ClientID == "":
		params.ClientID = a.clientID
	case params == nil && a.clientID != "":
		params = &TokenParams{ClientID: a.clientID}
	}
	tok, tokReqClientID, err := a.requestToken(params, opts)
	if err != nil {
		return nil, err
	}
	// Fail if the non-empty ClientID, that was set explicitely via ClientOptions, does
	// not match the non-wildcard ClientID returned with the token.
	if areClientIDsSet(a.clientID, tok.ClientID) && a.clientID != tok.ClientID {
		return nil, newError(40012, errClientIDMismatch)
	}
	// Fail if non-empty ClientID requested by a TokenRequest
	// does not match the non-wildcard ClientID that arrived with the token.
	if areClientIDsSet(tokReqClientID, tok.ClientID) && tokReqClientID != tok.ClientID {
		return nil, newError(40012, errClientIDMismatch)
	}
	a.method = authToken
	a.opts().TokenDetails = tok
	a.params = params
	a.clientID = tok.ClientID
	return tok, nil
}

func (a *Auth) reauthorise() (*TokenDetails, error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	return a.authorise(a.params, nil, true)
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

func (a *Auth) requestAuthURL(params *TokenParams, opts *AuthOptions) (interface{}, error) {
	req, err := http.NewRequest(opts.authMethod(), opts.AuthURL, nil)
	if err != nil {
		return nil, a.newError(40000, err)
	}
	query := addParams(params.Query(), opts.AuthParams).Encode()
	req.Header = addHeaders(req.Header, opts.AuthHeaders)
	switch opts.authMethod() {
	case "GET":
		req.URL.RawQuery = query
	case "POST":
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Content-Length", strconv.Itoa(len(query)))
		req.Body = ioutil.NopCloser(strings.NewReader(query))
	default:
		return nil, a.newError(40500, nil)
	}
	resp, err := a.opts().httpclient().Do(req)
	if err != nil {
		return nil, a.newError(40170, err)
	}
	if err = checkValidHTTPResponse(resp); err != nil {
		return nil, a.newError(40170, err)
	}
	defer resp.Body.Close()
	typ, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, a.newError(40004, err)
	}
	switch typ {
	case "text/plain":
		token, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, a.newError(40000, err)
		}
		return newTokenDetails(string(token)), nil
	case protocolJSON, protocolMsgPack:
		var req TokenRequest
		var buf bytes.Buffer
		err := decode(typ, io.TeeReader(resp.Body, &buf), &req)
		if err == nil && req.Mac != "" && req.Nonce != "" {
			return &req, nil
		}
		var token TokenDetails
		if err := decode(typ, io.MultiReader(&buf, resp.Body), &token); err != nil {
			return nil, a.newError(40000, err)
		}
		return &token, nil
	case "":
		return nil, a.newError(40000, errMissingType)
	default:
		return nil, a.newError(40000, errUnsupportedType)
	}
}

func (a *Auth) isTokenRenewable() bool {
	return a.opts().Key != "" || a.opts().AuthURL != "" || a.opts().AuthCallback != nil
}

func (a *Auth) newError(code int, err error) error {
	e := newError(code, err)
	e.Server = a.host
	return e
}

func (a *Auth) authReq(req *http.Request) error {
	switch a.method {
	case authBasic:
		req.SetBasicAuth(a.opts().KeyName(), a.opts().KeySecret())
	case authToken:
		if _, err := a.authorise(a.params, nil, false); err != nil {
			return err
		}
		encToken := base64.StdEncoding.EncodeToString([]byte(a.token().Token))
		req.Header.Set("Authorization", "Bearer "+encToken)
	}
	return nil
}

func (a *Auth) authQuery(query url.Values) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	switch a.method {
	case authBasic:
		query.Set("key", a.opts().Key)
	case authToken:
		if _, err := a.authorise(a.params, nil, false); err != nil {
			return err
		}
		query.Set("access_token", a.token().Token)
	}
	return nil
}

func (a *Auth) opts() *ClientOptions {
	return &a.client.opts
}

func (a *Auth) token() *TokenDetails {
	return a.opts().TokenDetails
}

func (a *Auth) logger() *Logger {
	return a.client.logger()
}

func detectAuthMethod(opts *ClientOptions) (int, error) {
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
		return authBasic, nil
	case isAuthExternal || isKeyValid:
		return authToken, nil
	default:
		return 0, newError(40102, errMissingTokenOpts)
	}
}

func areClientIDsSet(clientIDs ...string) bool {
	for _, s := range clientIDs {
		switch s {
		case "", "*":
			return false
		}
	}
	return true
}

func isClientIDAllowed(clientID, msgClientID string) bool {
	return clientID == "*" || msgClientID == "" || clientID == msgClientID
}
