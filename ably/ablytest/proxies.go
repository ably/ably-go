package ablytest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/ably/ably-go/ably"
)

var hopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailers":            {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

func NewTokenParams(query url.Values) *ably.TokenParams {
	params := &ably.TokenParams{}
	if n, err := strconv.ParseInt(query.Get("ttl"), 10, 64); err == nil {
		params.TTL = n
	}
	if s := query.Get("capability"); s != "" {
		params.RawCapability = s
	}
	if s := query.Get("clientId"); s != "" {
		params.ClientID = s
	}
	if n, err := strconv.ParseInt(query.Get("timestamp"), 10, 64); err == nil {
		params.Timestamp = n
	}
	return params
}

func Query(req *http.Request) (url.Values, error) {
	switch req.Method {
	case "GET":
		return req.URL.Query(), nil
	case "POST":
		p, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		return url.ParseQuery(string(p))
	default:
		return nil, errors.New(http.StatusText(405))
	}
}

func MustQuery(req *http.Request) url.Values {
	query, err := Query(req)
	if err != nil {
		panic(err)
	}
	return query
}

// AuthReverseProxy serves token requests by reverse proxying them to
// the Ably servers. Use URL method for creating values for AuthURL
// option and Callback method - for AuthCallback ones.
type AuthReverseProxy struct {
	TokenQueue []*ably.TokenDetails // when non-nil pops the token from the queue instead querying Ably servers
	Listener   net.Listener         // listener which accepts token request connections

	auth  *ably.Auth
	proto string
}

// NewAuthReverseProxy creates new auth reverse proxy. The given opts
// are used to create a Auth client, used to reverse proxying token requests.
func NewAuthReverseProxy(opts *ably.ClientOptions) (*AuthReverseProxy, error) {
	opts.UseTokenAuth = true
	client, err := ably.NewRestClient(opts)
	if err != nil {
		return nil, err
	}
	lis, err := net.Listen("tcp4", ":0")
	if err != nil {
		return nil, err
	}
	srv := &AuthReverseProxy{
		Listener: lis,
		auth:     client.Auth,
		proto:    protocol(opts),
	}
	go http.Serve(lis, srv)
	return srv, nil
}

// MustAuthReverseProxy panics when creating the proxy fails.
func MustAuthReverseProxy(opts *ably.ClientOptions) *AuthReverseProxy {
	srv, err := NewAuthReverseProxy(opts)
	if err != nil {
		panic(err)
	}
	return srv
}

// URL gives new AuthURL for the requested responseType. Available response
// types are:
//
//   - "token", which responds with (ably.TokenDetails).Token as a string
//   - "details", which responds with ably.TokenDetails
//   - "request", which responds with ably.TokenRequest
//
func (srv *AuthReverseProxy) URL(responseType string) string {
	return "http://" + srv.Listener.Addr().String() + "/" + responseType
}

// Callback gives new AuthCallback. Available response types are the same
// as for URL method.
func (srv *AuthReverseProxy) Callback(responseType string) func(context.Context, ably.TokenParamsV12) (ably.Tokener, error) {
	return func(ctx context.Context, params ably.TokenParamsV12) (ably.Tokener, error) {
		token, _, err := srv.handleAuth(responseType, params)
		return token, err
	}
}

// Close makes the proxy server stop accepting connections.
func (srv *AuthReverseProxy) Close() error {
	return srv.Listener.Close()
}

// ServeHTTP implements the http.Handler interface.
func (srv *AuthReverseProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	query, err := Query(req)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	token, contentType, err := srv.handleAuth(req.URL.Path[1:], *NewTokenParams(query))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	p, err := encode(contentType, token)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	for k, v := range req.Header {
		if _, ok := hopHeaders[k]; !ok {
			w.Header()[k] = v
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(p)))
	w.WriteHeader(200)
	if _, err = io.Copy(w, bytes.NewReader(p)); err != nil {
		panic(err)
	}
}

func (srv *AuthReverseProxy) handleAuth(responseType string, params ably.TokenParams) (token ably.Tokener, typ string, err error) {
	switch responseType {
	case "token", "details":
		var tok *ably.TokenDetails
		if len(srv.TokenQueue) != 0 {
			tok, srv.TokenQueue = srv.TokenQueue[0], srv.TokenQueue[1:]
		} else {
			tok, err = srv.auth.Authorize(&params, nil)
			if err != nil {
				return nil, "", err
			}
		}
		if responseType == "token" {
			return ably.TokenString(tok.Token), "text/plain", nil
		}
		return tok, srv.proto, nil
	case "request":
		tokReq, err := srv.auth.CreateTokenRequest(&params, nil)
		if err != nil {
			return nil, "", err
		}
		return tokReq, srv.proto, nil
	default:
		return nil, "", errors.New("unexpected token value type: " + typ)
	}
}
