package ably

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
)

const (
	protocolJSON    = "application/json"
	protocolMsgPack = "application/x-msgpack"

	// RestHost is the primary ably host .
	RestHost = "rest.ably.io"
)

var defaultOptions = clientOptions{
	RestHost:                 RestHost,
	HTTPMaxRetryCount:        3,
	RealtimeHost:             "realtime.ably.io",
	TimeoutDisconnect:        30 * time.Second,
	RealtimeRequestTimeout:   10 * time.Second, // DF1b
	DisconnectedRetryTimeout: 15 * time.Second, // TO3l1
	FallbackRetryTimeout:     10 * time.Minute,
	IdempotentRestPublishing: false,
	Port:                     80,
	TLSPort:                  443,
	Now:                      time.Now,
}

func DefaultFallbackHosts() []string {
	return []string{
		"a.ably-realtime.com",
		"b.ably-realtime.com",
		"c.ably-realtime.com",
		"d.ably-realtime.com",
		"e.ably-realtime.com",
	}
}

const (
	authBasic = 1 + iota
	authToken
)

type authOptions struct {
	// AuthCallback is called in order to obtain a signed token request.
	//
	// This enables a client to obtain token requests from another entity,
	// so tokens can be renewed without the client requiring access to keys.
	AuthCallback func(context.Context, TokenParams) (Tokener, error)

	// URL which is queried to obtain a signed token request.
	//
	// This enables a client to obtain token requests from another entity,
	// so tokens can be renewed without the client requiring access to keys.
	//
	// If AuthURL is non-empty and AuthCallback is nil, the Ably library
	// builds a req (*http.Request) which then is issued against the given AuthURL
	// in order to obtain authentication token. The response is expected to
	// carry a single token string in the payload when Content-Type header
	// is "text/plain" or JSON-encoded *ably.TokenDetails when the header
	// is "application/json".
	//
	// The req is built with the following values:
	//
	// GET requests:
	//
	//   - req.URL.RawQuery is encoded from *TokenParams and AuthParams
	//   - req.Header is set to AuthHeaders
	//
	// POST requests:
	//
	//   - req.Header is set to AuthHeaders
	//   - Content-Type is set to "application/x-www-form-urlencoded" and
	//     the payload is encoded from *TokenParams and AuthParams
	//
	AuthURL string

	// Key obtained from the dashboard.
	Key string

	// Token is an authentication token issued for this application against
	// a specific key and TokenParams.
	Token string

	// TokenDetails is an authentication token issued for this application against
	// a specific key and TokenParams.
	TokenDetails *TokenDetails

	// AuthMethod specifies which method, GET or POST, is used to query AuthURL
	// for the token information (*ably.TokenRequest or *ablyTokenDetails).
	//
	// If empty, GET is used by default.
	AuthMethod string

	// AuthHeaders are HTTP request headers to be included in any request made
	// to the AuthURL.
	AuthHeaders http.Header

	// AuthParams are HTTP query parameters to be included in any requset made
	// to the AuthURL.
	AuthParams url.Values

	// UseQueryTime when set to true, the time queried from Ably servers will
	// be used to sign the TokenRequest instead of using local time.
	UseQueryTime bool

	// Spec: TO3j11
	DefaultTokenParams *TokenParams

	// UseTokenAuth makes the Rest and Realtime clients always use token
	// authentication method.
	UseTokenAuth bool
}

func (opts *authOptions) externalTokenAuthSupported() bool {
	return !(opts.Token == "" && opts.TokenDetails == nil && opts.AuthCallback == nil && opts.AuthURL == "")
}

func (opts *authOptions) merge(extra *authOptions, defaults bool) *authOptions {
	ablyutil.Merge(opts, extra, defaults)
	return opts
}

func (opts *authOptions) authMethod() string {
	if opts.AuthMethod != "" {
		return opts.AuthMethod
	}
	return "GET"
}

// KeyName gives the key name parsed from the Key field.
func (opts *authOptions) KeyName() string {
	if i := strings.IndexRune(opts.Key, ':'); i != -1 {
		return opts.Key[:i]
	}
	return ""
}

// KeySecret gives the key secret parsed from the Key field.
func (opts *authOptions) KeySecret() string {
	if i := strings.IndexRune(opts.Key, ':'); i != -1 {
		return opts.Key[i+1:]
	}
	return ""
}

type clientOptions struct {
	authOptions

	RestHost string // optional; overwrite endpoint hostname for REST client

	FallbackHosts   []string
	RealtimeHost    string        // optional; overwrite endpoint hostname for Realtime client
	Environment     string        // optional; prefixes both hostname with the environment string
	Port            int           // optional: port to use for non-TLS connections and requests
	TLSPort         int           // optional: port to use for TLS connections and requests
	ClientID        string        // optional; required for managing realtime presence of the current client
	Recover         string        // optional; used to recover client state
	Logger          LoggerOptions // optional; overwrite logging defaults
	TransportParams url.Values

	// max number of fallback hosts to use as a fallback.
	HTTPMaxRetryCount int

	// The period in milliseconds before HTTP requests are retried against the
	// default endpoint
	//
	// spec TO3l10
	FallbackRetryTimeout time.Duration

	NoTLS            bool // when true REST and realtime client won't use TLS
	NoConnect        bool // when true realtime client will not attempt to connect automatically
	NoEcho           bool // when true published messages will not be echoed back
	NoQueueing       bool // when true drops messages published during regaining connection
	NoBinaryProtocol bool // when true uses JSON for network serialization protocol instead of MsgPack

	// When true idempotent rest publishing will be enabled.
	// Spec TO3n
	IdempotentRestPublishing bool

	// TimeoutConnect is the time period after which connect request is failed.
	//
	// Deprecated: use RealtimeRequestTimeout instead.
	TimeoutConnect    time.Duration
	TimeoutDisconnect time.Duration // time period after which disconnect request is failed

	// RealtimeRequestTimeout is the timeout for realtime connection establishment
	// and each subsequent operation.
	RealtimeRequestTimeout time.Duration

	// DisconnectedRetryTimeout is the time to wait after a disconnection before
	// attempting an automatic reconnection, if still disconnected.
	DisconnectedRetryTimeout time.Duration

	// Dial specifies the dial function for creating message connections used
	// by Realtime.
	//
	// If Dial is nil, the default websocket connection is used.
	Dial func(protocol string, u *url.URL) (proto.Conn, error)

	// Listener if set, will be automatically registered with On method for every
	// realtime connection and realtime channel created by realtime client.
	// The listener will receive events for all state transitions.
	Listener chan<- State

	// HTTPClient specifies the client used for HTTP communication by RestClient.
	//
	// If HTTPClient is nil, the http.DefaultClient is used.
	HTTPClient *http.Client

	//When provided this will be used on every request.
	Trace *httptrace.ClientTrace

	// Now returns the time the library should take as current.
	Now func() time.Time
}

func (opts *clientOptions) timeoutConnect() time.Duration {
	if opts.TimeoutConnect != 0 {
		return opts.TimeoutConnect
	}
	return defaultOptions.RealtimeRequestTimeout
}

func (opts *clientOptions) timeoutDisconnect() time.Duration {
	if opts.TimeoutDisconnect != 0 {
		return opts.TimeoutDisconnect
	}
	return defaultOptions.TimeoutDisconnect
}

func (opts *clientOptions) fallbackRetryTimeout() time.Duration {
	if opts.FallbackRetryTimeout != 0 {
		return opts.FallbackRetryTimeout
	}
	return defaultOptions.FallbackRetryTimeout
}

func (opts *clientOptions) realtimeRequestTimeout() time.Duration {
	if opts.RealtimeRequestTimeout != 0 {
		return opts.RealtimeRequestTimeout
	}
	return defaultOptions.RealtimeRequestTimeout
}

func (opts *clientOptions) disconnectedRetryTimeout() time.Duration {
	if opts.DisconnectedRetryTimeout != 0 {
		return opts.DisconnectedRetryTimeout
	}
	return defaultOptions.DisconnectedRetryTimeout
}

func (opts *clientOptions) restURL() string {
	host := resolveHost(opts.RestHost, opts.Environment, defaultOptions.RestHost)
	if opts.NoTLS {
		port := opts.Port
		if port == 0 {
			port = 80
		}
		return "http://" + net.JoinHostPort(host, strconv.FormatInt(int64(port), 10))
	} else {
		port := opts.TLSPort
		if port == 0 {
			port = 443
		}
		return "https://" + net.JoinHostPort(host, strconv.FormatInt(int64(port), 10))
	}
}

func (opts *clientOptions) realtimeURL() string {
	host := resolveHost(opts.RealtimeHost, opts.Environment, defaultOptions.RealtimeHost)
	if opts.NoTLS {
		port := opts.Port
		if port == 0 {
			port = 80
		}
		return "ws://" + net.JoinHostPort(host, strconv.FormatInt(int64(port), 10))
	} else {
		port := opts.TLSPort
		if port == 0 {
			port = 443
		}
		return "wss://" + net.JoinHostPort(host, strconv.FormatInt(int64(port), 10))
	}
}

func resolveHost(host, environment, defaultHost string) string {
	if host == "" {
		host = defaultHost
	}
	if host == defaultHost && environment != "" && environment != "production" {
		host = environment + "-" + host
	}
	return host
}
func (opts *clientOptions) httpclient() *http.Client {
	if opts.HTTPClient != nil {
		return opts.HTTPClient
	}
	return http.DefaultClient
}

func (opts *clientOptions) protocol() string {
	if opts.NoBinaryProtocol {
		return protocolJSON
	}
	return protocolMsgPack
}

func (opts *clientOptions) idempotentRestPublishing() bool {
	return opts.IdempotentRestPublishing
}

type ScopeParams struct {
	Start time.Time
	End   time.Time
	Unit  string
}

func (s ScopeParams) EncodeValues(out *url.Values) error {
	if !s.Start.IsZero() && !s.End.IsZero() && s.Start.After(s.End) {
		return fmt.Errorf("start mzust be before end")
	}
	if !s.Start.IsZero() {
		out.Set("start", strconv.FormatInt(unixMilli(s.Start), 10))
	}
	if !s.End.IsZero() {
		out.Set("end", strconv.FormatInt(unixMilli(s.End), 10))
	}
	if s.Unit != "" {
		out.Set("unit", s.Unit)
	}
	return nil
}

type PaginateParams struct {
	ScopeParams
	Limit     int
	Direction string
}

func (p *PaginateParams) EncodeValues(out *url.Values) error {
	if p.Limit < 0 {
		out.Set("limit", strconv.Itoa(100))
	} else if p.Limit != 0 {
		out.Set("limit", strconv.Itoa(p.Limit))
	}
	switch p.Direction {
	case "":
		break
	case "backwards", "forwards":
		out.Set("direction", p.Direction)
		break
	default:
		return fmt.Errorf("Invalid value for direction: %s", p.Direction)
	}
	p.ScopeParams.EncodeValues(out)
	return nil
}

type ClientOptions []func(*clientOptions)

func NewClientOptions(key string) ClientOptions {
	return ClientOptions{func(os *clientOptions) {
		os.Key = key
	}}
}

type AuthOptions []func(*authOptions)

// A Tokener is or can be used to get a TokenDetails.
type Tokener interface {
	IsTokener()
	isTokener()
}

// A TokenString is the string representation of an authentication token.
type TokenString string

func (TokenString) IsTokener() {}
func (TokenString) isTokener() {}

func (os AuthOptions) AuthCallback(authCallback func(context.Context, TokenParams) (Tokener, error)) AuthOptions {
	return append(os, func(os *authOptions) {
		os.AuthCallback = authCallback
	})
}

func (os AuthOptions) AuthParams(params url.Values) AuthOptions {
	return append(os, func(os *authOptions) {
		os.AuthParams = params
	})
}

func (os AuthOptions) AuthURL(url string) AuthOptions {
	return append(os, func(os *authOptions) {
		os.AuthURL = url
	})
}

func (os AuthOptions) AuthMethod(url string) AuthOptions {
	return append(os, func(os *authOptions) {
		os.AuthMethod = url
	})
}

func (os AuthOptions) AuthHeaders(headers http.Header) AuthOptions {
	return append(os, func(os *authOptions) {
		os.AuthHeaders = headers
	})
}

func (os AuthOptions) Key(key string) AuthOptions {
	return append(os, func(os *authOptions) {
		os.Key = key
	})
}

func (os AuthOptions) QueryTime(queryTime bool) AuthOptions {
	return append(os, func(os *authOptions) {
		os.UseQueryTime = queryTime
	})
}

func (os AuthOptions) Token(token string) AuthOptions {
	return append(os, func(os *authOptions) {
		os.Token = token
	})
}

func (os AuthOptions) TokenDetails(details *TokenDetails) AuthOptions {
	return append(os, func(os *authOptions) {
		os.TokenDetails = details
	})
}

func (os AuthOptions) UseTokenAuth(use bool) AuthOptions {
	return append(os, func(os *authOptions) {
		os.UseTokenAuth = use
	})
}

func (os ClientOptions) AuthCallback(authCallback func(context.Context, TokenParams) (Tokener, error)) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.AuthCallback = authCallback
	})
}

func (os ClientOptions) AuthParams(params url.Values) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.AuthParams = params
	})
}

func (os ClientOptions) AuthURL(url string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.AuthURL = url
	})
}

func (os ClientOptions) AuthMethod(url string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.AuthMethod = url
	})
}

func (os ClientOptions) AuthHeaders(headers http.Header) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.AuthHeaders = headers
	})
}

func (os ClientOptions) Key(key string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Key = key
	})
}

func (os ClientOptions) QueryTime(queryTime bool) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.UseQueryTime = queryTime
	})
}

func (os ClientOptions) Token(token string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Token = token
	})
}

func (os ClientOptions) TokenDetails(details *TokenDetails) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.TokenDetails = details
	})
}

func (os ClientOptions) UseTokenAuth(use bool) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.UseTokenAuth = use
	})
}

func (os ClientOptions) AutoConnect(autoConnect bool) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.NoConnect = !autoConnect
	})
}

func (os ClientOptions) ClientID(clientID string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.ClientID = clientID
	})
}

func (os AuthOptions) DefaultTokenParams(params TokenParams) AuthOptions {
	return append(os, func(os *authOptions) {
		os.DefaultTokenParams = &params
	})
}

func (os ClientOptions) EchoMessages(echo bool) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.NoEcho = !echo
	})
}

func (os ClientOptions) Environment(env string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Environment = env
	})
}

func (os ClientOptions) LogHandler(handler Logger) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Logger.Logger = handler
	})
}

func (os ClientOptions) LogLevel(level LogLevel) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Logger.Level = level
	})
}

func (os ClientOptions) Port(port int) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Port = port
	})
}

func (os ClientOptions) QueueMessages(queue bool) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.NoQueueing = !queue
	})
}

func (os ClientOptions) RESTHost(host string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.RestHost = host
	})
}

func (os ClientOptions) RealtimeHost(host string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.RealtimeHost = host
	})
}

func (os ClientOptions) FallbackHosts(hosts []string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.FallbackHosts = hosts
	})
}

func (os ClientOptions) Recover(key string) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Recover = key
	})
}

func (os ClientOptions) TLS(tls bool) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.NoTLS = !tls
	})
}

func (os ClientOptions) TLSPort(port int) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.TLSPort = port
	})
}

func (os ClientOptions) UseBinaryProtocol(use bool) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.NoBinaryProtocol = !use
	})
}

func (os ClientOptions) TransportParams(params url.Values) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.TransportParams = params
	})
}

func (os ClientOptions) DisconnectedRetryTimeout(d time.Duration) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.DisconnectedRetryTimeout = d
	})
}

func (os ClientOptions) HTTPMaxRetryCount(count int) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.HTTPMaxRetryCount = count
	})
}

func (os ClientOptions) IdempotentRESTPublishing(idempotent bool) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.IdempotentRestPublishing = idempotent
	})
}

func (os ClientOptions) HTTPClient(client *http.Client) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.HTTPClient = client
	})
}

func (os ClientOptions) Dial(dial func(protocol string, u *url.URL) (proto.Conn, error)) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Dial = dial
	})
}

func (os ClientOptions) applyWithDefaults() *clientOptions {
	to := defaultOptions

	for _, set := range os {
		set(&to)
	}

	if to.DefaultTokenParams == nil {
		to.DefaultTokenParams = &TokenParams{
			TTL: int64(60 * time.Minute / time.Millisecond),
		}
	}

	return &to
}

func (os AuthOptions) applyWithDefaults() *authOptions {
	to := defaultOptions.authOptions

	for _, set := range os {
		set(&to)
	}

	if to.DefaultTokenParams == nil {
		to.DefaultTokenParams = &TokenParams{
			TTL: int64(60 * time.Minute / time.Millisecond),
		}
	}

	return &to
}
