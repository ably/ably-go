package ably

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
)

const (
	protocolJSON    = "application/json"
	protocolMsgPack = "application/x-msgpack"

	// restHost is the primary ably host .
	restHost     = "rest.ably.io"
	realtimeHost = "realtime.ably.io"
	Port         = 80
	TLSPort      = 443
)

var defaultOptions = clientOptions{
	RESTHost:                 restHost,
	FallbackHosts:            defaultFallbackHosts(),
	HTTPMaxRetryCount:        3,
	HTTPRequestTimeout:       10 * time.Second,
	RealtimeHost:             realtimeHost,
	TimeoutDisconnect:        30 * time.Second,
	ConnectionStateTTL:       120 * time.Second,
	RealtimeRequestTimeout:   10 * time.Second, // DF1b
	SuspendedRetryTimeout:    30 * time.Second, //  RTN14d, TO3l2
	DisconnectedRetryTimeout: 15 * time.Second, // TO3l1
	HTTPOpenTimeout:          4 * time.Second,  //TO3l3
	ChannelRetryTimeout:      15 * time.Second, // TO3l7
	FallbackRetryTimeout:     10 * time.Minute,
	IdempotentRESTPublishing: false,
	Port:                     Port,
	TLSPort:                  TLSPort,
	Now:                      time.Now,
	After:                    ablyutil.After,
	LogLevel:                 LogWarning, // RSC2
}

func defaultFallbackHosts() []string {
	return []string{
		"a.ably-realtime.com",
		"b.ably-realtime.com",
		"c.ably-realtime.com",
		"d.ably-realtime.com",
		"e.ably-realtime.com",
	}
}

func getEnvFallbackHosts(env string) []string {
	return []string{
		fmt.Sprintf("%s-%s", env, "a-fallback.ably-realtime.com"),
		fmt.Sprintf("%s-%s", env, "b-fallback.ably-realtime.com"),
		fmt.Sprintf("%s-%s", env, "c-fallback.ably-realtime.com"),
		fmt.Sprintf("%s-%s", env, "d-fallback.ably-realtime.com"),
		fmt.Sprintf("%s-%s", env, "e-fallback.ably-realtime.com"),
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

	// AuthParams are HTTP query parameters to be included in any request made
	// to the AuthURL.
	AuthParams url.Values

	// UseQueryTime when set to true, the time queried from Ably servers will
	// be used to sign the TokenRequest instead of using local time.
	UseQueryTime bool

	// Spec: TO3j11
	DefaultTokenParams *TokenParams

	// UseTokenAuth makes the REST and Realtime clients always use token
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

	RESTHost string // optional; overwrite endpoint hostname for REST client
	// Deprecated: The library will automatically use default fallback hosts when a custom REST host or custom fallback hosts aren't provided.
	FallbackHostsUseDefault bool

	FallbackHosts   []string
	RealtimeHost    string // optional; overwrite endpoint hostname for Realtime client
	Environment     string // optional; prefixes both hostname with the environment string
	Port            int    // optional: port to use for non-TLS connections and requests
	TLSPort         int    // optional: port to use for TLS connections and requests
	ClientID        string // optional; required for managing realtime presence of the current client
	Recover         string // optional; used to recover client state
	TransportParams url.Values

	// max number of fallback hosts to use as a fallback.
	HTTPMaxRetryCount int
	// HTTPRequestTimeout is the timeout for getting a response for outgoing HTTP requests.
	//
	// Will only be used if no custom HTTPClient is set.
	HTTPRequestTimeout time.Duration

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
	IdempotentRESTPublishing bool

	// TimeoutConnect is the time period after which connect request is failed.
	//
	// Deprecated: use RealtimeRequestTimeout instead.
	TimeoutConnect    time.Duration
	TimeoutDisconnect time.Duration // time period after which disconnect request is failed

	ConnectionStateTTL time.Duration //(DF1a)

	// RealtimeRequestTimeout is the timeout for realtime connection establishment
	// and each subsequent operation.
	RealtimeRequestTimeout time.Duration

	// DisconnectedRetryTimeout is the time to wait after a disconnection before
	// attempting an automatic reconnection, if still disconnected.
	DisconnectedRetryTimeout time.Duration
	SuspendedRetryTimeout    time.Duration
	ChannelRetryTimeout      time.Duration
	HTTPOpenTimeout          time.Duration

	// Dial specifies the dial function for creating message connections used
	// by Realtime.
	//
	// If Dial is nil, the default websocket connection is used.
	Dial func(protocol string, u *url.URL, timeout time.Duration) (conn, error)

	// HTTPClient specifies the client used for HTTP communication by REST.
	//
	// If HTTPClient is nil, a client configured with default settings is used.
	HTTPClient *http.Client

	//When provided this will be used on every request.
	Trace *httptrace.ClientTrace

	// Now returns the time the library should take as current.
	Now   func() time.Time
	After func(context.Context, time.Duration) <-chan time.Time

	LogLevel   LogLevel
	LogHandler Logger
}

func (opts *clientOptions) validate() error {
	_, err := opts.getFallbackHosts()
	if err != nil {
		logger := opts.LogHandler
		logger.Printf(LogError, "Error getting fallbackHosts : %v", err.Error())
		return err
	}
	return nil
}

func (opts *clientOptions) isProductionEnvironment() bool {
	env := opts.Environment
	return empty(env) || strings.EqualFold(env, "production")
}

func (opts *clientOptions) activePort() (port int, isDefault bool) {
	if opts.NoTLS {
		port = opts.Port
		if port == 0 {
			port = defaultOptions.Port
		}
		if port == defaultOptions.Port {
			isDefault = true
		}
		return
	}
	port = opts.TLSPort
	if port == 0 {
		port = defaultOptions.TLSPort
	}
	if port == defaultOptions.TLSPort {
		isDefault = true
	}
	return
}

func (opts *clientOptions) getRestHost() string {
	if !empty(opts.RESTHost) {
		return opts.RESTHost
	}
	if !opts.isProductionEnvironment() {
		return opts.Environment + "-" + defaultOptions.RESTHost
	}
	return defaultOptions.RESTHost
}

func (opts *clientOptions) getRealtimeHost() string {
	if !empty(opts.RealtimeHost) {
		return opts.RealtimeHost
	}
	if !empty(opts.RESTHost) {
		logger := opts.LogHandler
		logger.Printf(LogWarning, "restHost is set to %s but realtimeHost is not set so setting realtimeHost to %s too. If this is not what you want, please set realtimeHost explicitly.", opts.RESTHost, opts.RealtimeHost)
		return opts.RESTHost
	}
	if !opts.isProductionEnvironment() {
		return opts.Environment + "-" + defaultOptions.RealtimeHost
	}
	return defaultOptions.RealtimeHost
}

func empty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

func (opts *clientOptions) restURL() (restUrl string) {
	baseUrl := opts.getRestHost()
	_, _, err := net.SplitHostPort(baseUrl)
	if err != nil { // set port if not set in baseUrl
		port, _ := opts.activePort()
		baseUrl = net.JoinHostPort(baseUrl, strconv.Itoa(port))
	}
	if opts.NoTLS {
		return "http://" + baseUrl
	}
	return "https://" + baseUrl
}

func (opts *clientOptions) realtimeURL() (realtimeUrl string) {
	baseUrl := opts.getRealtimeHost()
	_, _, err := net.SplitHostPort(baseUrl)
	if err != nil { // set port if not set in baseUrl
		port, _ := opts.activePort()
		baseUrl = net.JoinHostPort(baseUrl, strconv.Itoa(port))
	}
	if opts.NoTLS {
		return "ws://" + baseUrl
	}
	return "wss://" + baseUrl
}

func (opts *clientOptions) getFallbackHosts() ([]string, error) {
	logger := opts.LogHandler
	_, isDefaultPort := opts.activePort()
	if opts.FallbackHostsUseDefault {
		if opts.FallbackHosts != nil {
			return nil, errors.New("fallbackHosts and fallbackHostsUseDefault cannot both be set")
		}
		if !isDefaultPort {
			return nil, errors.New("fallbackHostsUseDefault cannot be set when port or tlsPort are set")
		}
		if !empty(opts.Environment) {
			logger.Printf(LogWarning, "Deprecated fallbackHostsUseDefault : There is no longer a need to set this when the environment option is also set since the library can generate the correct fallback hosts using the environment option.")
		}
		logger.Printf(LogWarning, "Deprecated fallbackHostsUseDefault : using default fallbackhosts")
		return defaultOptions.FallbackHosts, nil
	}
	if opts.FallbackHosts == nil && empty(opts.RESTHost) && empty(opts.RealtimeHost) && isDefaultPort {
		if opts.isProductionEnvironment() {
			return defaultOptions.FallbackHosts, nil
		}
		return getEnvFallbackHosts(opts.Environment), nil
	}
	return opts.FallbackHosts, nil
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
func (opts *clientOptions) connectionStateTTL() time.Duration {
	if opts.ConnectionStateTTL != 0 {
		return opts.ConnectionStateTTL
	}
	return defaultOptions.ConnectionStateTTL
}

func (opts *clientOptions) disconnectedRetryTimeout() time.Duration {
	if opts.DisconnectedRetryTimeout != 0 {
		return opts.DisconnectedRetryTimeout
	}
	return defaultOptions.DisconnectedRetryTimeout
}

func (opts *clientOptions) httpOpenTimeout() time.Duration {
	if opts.HTTPOpenTimeout != 0 {
		return opts.HTTPOpenTimeout
	}
	return defaultOptions.HTTPOpenTimeout
}

func (opts *clientOptions) suspendedRetryTimeout() time.Duration {
	if opts.SuspendedRetryTimeout != 0 {
		return opts.SuspendedRetryTimeout
	}
	return defaultOptions.SuspendedRetryTimeout
}

func (opts *clientOptions) httpclient() *http.Client {
	if opts.HTTPClient != nil {
		return opts.HTTPClient
	}
	return &http.Client{
		Timeout: opts.HTTPRequestTimeout,
	}
}

func (opts *clientOptions) protocol() string {
	if opts.NoBinaryProtocol {
		return protocolJSON
	}
	return protocolMsgPack
}

func (opts *clientOptions) idempotentRESTPublishing() bool {
	return opts.IdempotentRESTPublishing
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

// A ClientOption configures a REST or Realtime instance.
//
// See: https://www.ably.io/documentation/realtime/usage#client-options
type ClientOption func(*clientOptions)

// An AuthOption configures authentication/authorization for a REST or Realtime
// instance or operation.
type AuthOption func(*authOptions)

// A Tokener is or can be used to get a TokenDetails.
type Tokener interface {
	IsTokener()
	isTokener()
}

// A TokenString is the string representation of an authentication token.
type TokenString string

func (TokenString) IsTokener() {}
func (TokenString) isTokener() {}

func AuthWithCallback(authCallback func(context.Context, TokenParams) (Tokener, error)) AuthOption {
	return func(os *authOptions) {
		os.AuthCallback = authCallback
	}
}

func AuthWithParams(params url.Values) AuthOption {
	return func(os *authOptions) {
		os.AuthParams = params
	}
}

func AuthWithURL(url string) AuthOption {
	return func(os *authOptions) {
		os.AuthURL = url
	}
}

func AuthWithMethod(method string) AuthOption {
	return func(os *authOptions) {
		os.AuthMethod = method
	}
}

func AuthWithHeaders(headers http.Header) AuthOption {
	return func(os *authOptions) {
		os.AuthHeaders = headers
	}
}

func AuthWithKey(key string) AuthOption {
	return func(os *authOptions) {
		os.Key = key
	}
}

func AuthWithQueryTime(queryTime bool) AuthOption {
	return func(os *authOptions) {
		os.UseQueryTime = queryTime
	}
}

func AuthWithToken(token string) AuthOption {
	return func(os *authOptions) {
		os.Token = token
	}
}

func AuthWithTokenDetails(details *TokenDetails) AuthOption {
	return func(os *authOptions) {
		os.TokenDetails = details
	}
}

func AuthWithUseTokenAuth(use bool) AuthOption {
	return func(os *authOptions) {
		os.UseTokenAuth = use
	}
}

func WithAuthCallback(authCallback func(context.Context, TokenParams) (Tokener, error)) ClientOption {
	return func(os *clientOptions) {
		os.AuthCallback = authCallback
	}
}

func WithAuthParams(params url.Values) ClientOption {
	return func(os *clientOptions) {
		os.AuthParams = params
	}
}

func WithAuthURL(url string) ClientOption {
	return func(os *clientOptions) {
		os.AuthURL = url
	}
}

func WithAuthMethod(method string) ClientOption {
	return func(os *clientOptions) {
		os.AuthMethod = method
	}
}

func WithAuthHeaders(headers http.Header) ClientOption {
	return func(os *clientOptions) {
		os.AuthHeaders = headers
	}
}

func WithKey(key string) ClientOption {
	return func(os *clientOptions) {
		os.Key = key
	}
}

func WithDefaultTokenParams(params TokenParams) ClientOption {
	return func(os *clientOptions) {
		os.DefaultTokenParams = &params
	}
}

func WithQueryTime(queryTime bool) ClientOption {
	return func(os *clientOptions) {
		os.UseQueryTime = queryTime
	}
}

func WithToken(token string) ClientOption {
	return func(os *clientOptions) {
		os.Token = token
	}
}

func WithTokenDetails(details *TokenDetails) ClientOption {
	return func(os *clientOptions) {
		os.TokenDetails = details
	}
}

func WithUseTokenAuth(use bool) ClientOption {
	return func(os *clientOptions) {
		os.UseTokenAuth = use
	}
}

func WithAutoConnect(autoConnect bool) ClientOption {
	return func(os *clientOptions) {
		os.NoConnect = !autoConnect
	}
}

func WithClientID(clientID string) ClientOption {
	return func(os *clientOptions) {
		os.ClientID = clientID
	}
}

func AuthWithDefaultTokenParams(params TokenParams) AuthOption {
	return func(os *authOptions) {
		os.DefaultTokenParams = &params
	}
}

func WithEchoMessages(echo bool) ClientOption {
	return func(os *clientOptions) {
		os.NoEcho = !echo
	}
}

func WithEnvironment(env string) ClientOption {
	return func(os *clientOptions) {
		os.Environment = env
	}
}

func WithLogHandler(handler Logger) ClientOption {
	return func(os *clientOptions) {
		os.LogHandler = handler
	}
}

func WithLogLevel(level LogLevel) ClientOption {
	return func(os *clientOptions) {
		os.LogLevel = level
	}
}

func WithPort(port int) ClientOption {
	return func(os *clientOptions) {
		os.Port = port
	}
}

func WithQueueMessages(queue bool) ClientOption {
	return func(os *clientOptions) {
		os.NoQueueing = !queue
	}
}

func WithRESTHost(host string) ClientOption {
	return func(os *clientOptions) {
		os.RESTHost = host
	}
}

func WithHTTPRequestTimeout(timeout time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.HTTPRequestTimeout = timeout
	}
}

func WithRealtimeHost(host string) ClientOption {
	return func(os *clientOptions) {
		os.RealtimeHost = host
	}
}

func WithFallbackHosts(hosts []string) ClientOption {
	return func(os *clientOptions) {
		os.FallbackHosts = hosts
	}
}

func WithRecover(key string) ClientOption {
	return func(os *clientOptions) {
		os.Recover = key
	}
}

func WithTLS(tls bool) ClientOption {
	return func(os *clientOptions) {
		os.NoTLS = !tls
	}
}

func WithTLSPort(port int) ClientOption {
	return func(os *clientOptions) {
		os.TLSPort = port
	}
}

func WithUseBinaryProtocol(use bool) ClientOption {
	return func(os *clientOptions) {
		os.NoBinaryProtocol = !use
	}
}

func WithTransportParams(params url.Values) ClientOption {
	return func(os *clientOptions) {
		os.TransportParams = params
	}
}

func WithDisconnectedRetryTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.DisconnectedRetryTimeout = d
	}
}

func WithHTTPOpenTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.HTTPOpenTimeout = d
	}
}

func WithRealtimeRequestTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.RealtimeRequestTimeout = d
	}
}

func WithSuspendedRetryTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.SuspendedRetryTimeout = d
	}
}

func WithChannelRetryTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.ChannelRetryTimeout = d
	}
}

func WithHTTPMaxRetryCount(count int) ClientOption {
	return func(os *clientOptions) {
		os.HTTPMaxRetryCount = count
	}
}

func WithIdempotentRESTPublishing(idempotent bool) ClientOption {
	return func(os *clientOptions) {
		os.IdempotentRESTPublishing = idempotent
	}
}

func WithHTTPClient(client *http.Client) ClientOption {
	return func(os *clientOptions) {
		os.HTTPClient = client
	}
}

func WithFallbackHostsUseDefault(fallbackHostsUseDefault bool) ClientOption {
	return func(os *clientOptions) {
		os.FallbackHostsUseDefault = fallbackHostsUseDefault
	}
}

func WithDial(dial func(protocol string, u *url.URL, timeout time.Duration) (conn, error)) ClientOption {
	return func(os *clientOptions) {
		os.Dial = dial
	}
}

func applyOptionsWithDefaults(opts ...ClientOption) *clientOptions {
	to := defaultOptions
	// No need to set hosts by default
	to.RESTHost = ""
	to.RealtimeHost = ""
	to.FallbackHosts = nil

	for _, set := range opts {
		set(&to)
	}

	if to.DefaultTokenParams == nil {
		to.DefaultTokenParams = &TokenParams{
			TTL: int64(60 * time.Minute / time.Millisecond),
		}
	}

	if to.LogHandler == nil {
		to.LogHandler = &stdLogger{Logger: log.New(os.Stderr, "", log.LstdFlags)}
	}
	to.LogHandler = filteredLogger{Logger: to.LogHandler, Level: to.LogLevel}

	return &to
}

func applyAuthOptionsWithDefaults(os ...AuthOption) *authOptions {
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

func (o *clientOptions) contextWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return ablyutil.ContextWithTimeout(ctx, o.After, timeout)
}
