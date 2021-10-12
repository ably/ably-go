package ably

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

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

const wildcardClientID = "*"

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
	client   *REST
	params   *TokenParams // save params to use with token renewal
	host     string       // a host part of AuthURL
	clientID string       // clientID of the authenticated user or wildcard "*"

	// onExplicitAuthorize is the callback that Realtime sets to reauthorize with the
	// server when Authorize is explicitly called.
	onExplicitAuthorize func(context.Context, *TokenDetails)

	serverTimeOffset time.Duration

	// ServerTimeHandler when provided this will be used to query server time.
	serverTimeHandler func() (time.Time, error)
}

func newAuth(client *REST) (*Auth, error) {
	a := &Auth{
		client:              client,
		onExplicitAuthorize: func(context.Context, *TokenDetails) {},
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
	if a.opts().ClientID != "" {
		if a.opts().ClientID == wildcardClientID {
			// References RSA7c
			return nil, newError(ErrIncompatibleCredentials, errWildcardClientID)
		}
		// References RSC17, RSA7b1
		a.clientID = a.opts().ClientID
	}

	return a, nil
}

// ClientID
func (a *Auth) ClientID() string {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	if a.clientID != wildcardClientID {
		return a.clientID
	}
	return ""
}

func (a *Auth) clientIDForCheck() string {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	if a.method == authBasic {
		return wildcardClientID // for Basic Auth no ClientID check is performed
	}
	return a.clientID
}

func (a *Auth) updateClientID(clientID string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	//Spec RSA7b3, RSA7b4, RSA12a,RSA12b, RSA7b2,
	a.clientID = clientID
}

// CreateTokenRequest
func (a *Auth) CreateTokenRequest(params *TokenParams, opts ...AuthOption) (*TokenRequest, error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	var o *authOptions
	if opts != nil {
		o = applyAuthOptionsWithDefaults(opts...)
	}
	return a.createTokenRequest(params, o)
}

func (a *Auth) createTokenRequest(params *TokenParams, opts *authOptions) (*TokenRequest, error) {
	if opts == nil {
		opts = &a.opts().authOptions
	}
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
		return nil, newError(ErrInvalidCredentials, errMissingKey)
	case req.KeyName == "" || keySecret == "":
		return nil, newError(ErrIncompatibleCredentials, errInvalidKey)
	}
	req.sign([]byte(keySecret))
	return req, nil
}

// RequestToken
func (a *Auth) RequestToken(ctx context.Context, params *TokenParams, opts ...AuthOption) (*TokenDetails, error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	var o *authOptions
	if opts != nil {
		o = applyAuthOptionsWithDefaults(opts...)
	}
	tok, _, err := a.requestToken(ctx, params, o)
	return tok, err
}

func (a *Auth) requestToken(ctx context.Context, params *TokenParams, opts *authOptions) (tok *TokenDetails, tokReqClientID string, err error) {
	switch {
	case opts != nil && opts.Token != "":
		a.log().Verbose("Auth: found token in []AuthOption")
		return newTokenDetails(opts.Token), "", nil
	case opts != nil && opts.TokenDetails != nil:
		a.log().Verbose("Auth: found TokenDetails in []AuthOption")
		return opts.TokenDetails, "", nil
	}
	if params == nil {
		params = a.opts().DefaultTokenParams
	}
	opts = a.mergeOpts(opts)
	var tokReq *TokenRequest
	switch {
	case opts.AuthCallback != nil:
		a.log().Verbose("Auth: found AuthCallback in []AuthOption")
		v, err := opts.AuthCallback(context.TODO(), *params)
		if err != nil {
			a.log().Error("Auth: failed calling opts.AuthCallback ", err)
			return nil, "", newError(ErrErrorFromClientTokenCallback, err)
		}

		// Simplify the switch below by removing the pointer-to-TokenLike cases.
		switch p := v.(type) {
		case *TokenRequest:
			v = *p
		case *TokenDetails:
			v = *p
		}

		switch v := v.(type) {
		case TokenRequest:
			tokReq = &v
			tokReqClientID = tokReq.ClientID
		case TokenDetails:
			return &v, "", nil
		case TokenString:
			return newTokenDetails(string(v)), "", nil
		default:
			panic(fmt.Errorf("unhandled TokenLike: %T", v))
		}
	case opts.AuthURL != "":
		a.log().Verbose("Auth: found AuthURL in []AuthOption")
		res, err := a.requestAuthURL(ctx, params, opts)
		if err != nil {
			a.log().Error("Auth: failed calling requesting token with AuthURL ", err)
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
		a.log().Verbose("Auth: using default token request")

		req, err := a.createTokenRequest(params, opts)
		if err != nil {
			return nil, "", err
		}
		tokReq = req
	}
	tok = &TokenDetails{}
	r := &request{
		Method: "POST",
		Path:   "/keys/" + tokReq.KeyName + "/requestToken",
		In:     tokReq,
		Out:    tok,
		NoAuth: true,
	}
	if _, err := a.client.do(ctx, r); err != nil {
		return nil, "", err
	}
	return tok, tokReqClientID, nil
}

// Authorize performs authorization with ably service and returns the
// authorization token details.
//
// Refers to RSA10
func (a *Auth) Authorize(ctx context.Context, params *TokenParams, setOpts ...AuthOption) (*TokenDetails, error) {
	var opts *authOptions
	if setOpts != nil {
		opts = applyAuthOptionsWithDefaults(setOpts...)
	}
	a.mtx.Lock()
	token, err := a.authorize(ctx, params, opts, true)
	a.mtx.Unlock()
	if err != nil {
		return nil, err
	}
	a.onExplicitAuthorize(ctx, token)
	return token, nil
}

func (a *Auth) authorize(ctx context.Context, params *TokenParams, opts *authOptions, force bool) (*TokenDetails, error) {
	switch tok := a.token(); {
	case tok != nil && !force && (tok.Expires == 0 || !tok.expired(a.opts().Now())):
		return tok, nil
	case params != nil && params.ClientID == "":
		params.ClientID = a.clientID
	case params == nil && a.clientID != "":
		params = &TokenParams{ClientID: a.clientID}
	}
	a.log().Info("Auth: sending  token request")
	tok, tokReqClientID, err := a.requestToken(ctx, params, opts)
	if err != nil {
		a.log().Error("Auth: failed to get token", err)
		return nil, err
	}
	// Fail if the non-empty ClientID, that was set explicitly via clientOptions, does
	// not match the non-wildcard ClientID returned with the token.
	if areClientIDsSet(a.clientID, tok.ClientID) && a.clientID != tok.ClientID {
		a.log().Error("Auth: ", errClientIDMismatch)
		return nil, newError(ErrInvalidClientID, errClientIDMismatch)
	}
	// Fail if non-empty ClientID requested by a TokenRequest
	// does not match the non-wildcard ClientID that arrived with the token.
	if areClientIDsSet(tokReqClientID, tok.ClientID) && tokReqClientID != tok.ClientID {
		a.log().Error("Auth: ", errClientIDMismatch)
		return nil, newError(ErrInvalidClientID, errClientIDMismatch)
	}
	a.method = authToken
	a.opts().TokenDetails = tok
	a.params = params
	a.clientID = tok.ClientID // Spec RSA7b2
	return tok, nil
}

func (a *Auth) reauthorize(ctx context.Context) (*TokenDetails, error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.log().Info("Auth: reauthorize")
	return a.authorize(ctx, a.params, nil, true)
}

func (a *Auth) mergeOpts(opts *authOptions) *authOptions {
	if opts == nil {
		opts = &a.opts().authOptions
	} else {
		a.opts().authOptions.merge(opts, false)
	}
	return opts
}

func (a *Auth) setDefaults(opts *authOptions, req *TokenRequest) error {
	if req.Nonce == "" {
		req.Nonce = randomString(32)
	}
	if req.Capability == "" {
		req.Capability = `{"*":["*"]}`
	}
	if req.TTL == 0 {
		req.TTL = 60 * 60 * 1000
	}
	if req.ClientID == "" {
		req.ClientID = a.opts().ClientID
	}
	if req.Timestamp == 0 {
		ts, err := a.timestamp(context.Background(), opts.UseQueryTime)
		if err != nil {
			return err
		}
		req.Timestamp = unixMilli(ts)
	}
	return nil
}

//Timestamp returns the timestamp to be used in authorization request.
func (a *Auth) timestamp(ctx context.Context, query bool) (time.Time, error) {
	now := a.client.opts.Now()
	if !query {
		return now, nil
	}
	if a.serverTimeOffset != 0 {
		// refers to rsa10k
		//
		// No need to do api call for time from the server. We are calculating it
		// using the cached offset(duration) value.
		return now.Add(a.serverTimeOffset), nil
	}
	var serverTime time.Time
	if a.serverTimeHandler != nil {
		t, err := a.serverTimeHandler()
		if err != nil {
			return time.Time{}, newError(ErrUnauthorized, err)
		}
		serverTime = t
	} else {
		t, err := a.client.Time(ctx)
		if err != nil {
			return time.Time{}, newError(ErrUnauthorized, err)
		}
		serverTime = t
	}
	a.serverTimeOffset = serverTime.Sub(now)
	return serverTime, nil
}

func (a *Auth) requestAuthURL(ctx context.Context, params *TokenParams, opts *authOptions) (interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, opts.authMethod(), opts.AuthURL, nil)
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
		return nil, a.newError(ErrErrorFromClientTokenCallback, err)
	}
	if err = checkValidHTTPResponse(resp); err != nil {
		return nil, a.newError(ErrErrorFromClientTokenCallback, err)
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
		if err == nil && req.MAC != "" && req.Nonce != "" {
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

func (a *Auth) newError(code ErrorCode, err error) error {
	return newError(code, err)
}

func (a *Auth) authReq(req *http.Request) error {
	switch a.method {
	case authBasic:
		req.SetBasicAuth(a.opts().KeyName(), a.opts().KeySecret())
	case authToken:
		if _, err := a.authorize(req.Context(), a.params, nil, false); err != nil {
			return err
		}
		encToken := base64.StdEncoding.EncodeToString([]byte(a.token().Token))
		req.Header.Set("Authorization", "Bearer "+encToken)
	}
	return nil
}

func (a *Auth) authQuery(ctx context.Context, query url.Values) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	switch a.method {
	case authBasic:
		query.Set("key", a.opts().Key)
	case authToken:
		if _, err := a.authorize(ctx, a.params, nil, false); err != nil {
			return err
		}
		query.Set("access_token", a.token().Token)
	}
	return nil
}

func (a *Auth) opts() *clientOptions {
	return a.client.opts
}

func (a *Auth) token() *TokenDetails {
	return a.opts().TokenDetails
}

func (a *Auth) log() logger {
	return a.client.log
}

func detectAuthMethod(opts *clientOptions) (int, error) {
	isKeyValid := opts.KeyName() != "" && opts.KeySecret() != ""
	isAuthExternal := opts.externalTokenAuthSupported()
	if opts.UseTokenAuth || isAuthExternal {
		return authToken, nil
	}
	if !isKeyValid {
		return 0, newError(ErrInvalidCredential, errInvalidKey)
	}
	if opts.NoTLS {
		return 0, newError(ErrInvalidUseOfBasicAuthOverNonTLSTransport, errInsecureBasicAuth)
	}
	return authBasic, nil
}

func areClientIDsSet(clientIDs ...string) bool {
	for _, s := range clientIDs {
		switch s {
		case "", wildcardClientID:
			return false
		}
	}
	return true
}

func isClientIDAllowed(clientID, msgClientID string) bool {
	return clientID == wildcardClientID || msgClientID == "" || clientID == msgClientID
}
