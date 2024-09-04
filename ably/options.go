package ably

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

	// restHost is the primary ably host.
	restHost = "rest.ably.io"
	// realtimeHost is the primary ably host.
	realtimeHost   = "realtime.ably.io"
	Port           = 80
	TLSPort        = 443
	maxMessageSize = 65536 // 64kb, default value TO3l8

	// RTN17c
	internetCheckUrl = "https://internet-up.ably-realtime.com/is-the-internet-up.txt"
	internetCheckOk  = "yes"
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
	IdempotentRESTPublishing: true, // TO3n
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

// authOptions passes authentication-specific properties in authentication requests to Ably.
// Properties set using [ably.authOptions] are used instead of the default values set when the client
// library is instantiated, as opposed to being merged with them.
type authOptions struct {

	// AuthCallback function is called when a new token is required.
	// The role of the callback is to obtain a fresh token, one of
	//
	//	1. An Ably Token string (https://ably.com/docs/core-features/authentication#token-process)
	//	2. A signed [ably.TokenRequest] (https://ably.com/docs/core-features/authentication#token-request-process)
	//	3. An [ably.TokenDetails] (https://ably.com/docs/core-features/authentication#token-process)
	//	4. [An Ably JWT].
	//
	// See the [authentication doc] for more details and associated API calls (RSA4a, RSA4, TO3j5, AO2b).
	//
	// [authentication doc]: https://ably.com/docs/core-features/authentication
	// [An Ably JWT]: https://ably.com/docs/core-features/authentication#ably-jwt-process
	AuthCallback func(context.Context, TokenParams) (Tokener, error)

	// AuthURL is a url that library will use to obtain
	//	1. An Ably Token string (https://ably.com/docs/core-features/authentication#token-process)
	//	2. A signed [ably.TokenRequest] (https://ably.com/docs/core-features/authentication#token-request-process)
	//	3. An [ably.TokenDetails] (https://ably.com/docs/core-features/authentication#token-process)
	//	4. [An Ably JWT].
	//
	// See the [authentication doc] for more details and associated API calls (RSA4a, RSA4, RSA8c, TO3j6, AO2c).
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
	// [authentication doc]: https://ably.com/docs/core-features/authentication
	// [An Ably JWT]: https://ably.com/docs/core-features/authentication#ably-jwt-process
	AuthURL string

	// Key is a full API key string, as obtained from the Ably dashboard.
	// Use this option if you wish to use Basic authentication, or wish to be able to issue Ably Tokens
	// without needing to defer to a separate entity to sign multiple [ably.TokenRequest].
	// Read more about [Basic authentication] (RSA11, RSA14, TO3j1, AO2a).
	//
	// [Basic authentication]: https://ably.com/docs/core-features/authentication#basic-authentication
	Key string

	// Token is an authenticated token.
	// This can either be a token string (obtained from the token property of a [ably.TokenDetails] component of
	// an Ably [ably.TokenRequest] response, or a JSON Web Token satisfying the Ably requirements for JWTs).
	//
	// This option is mostly useful for testing: since tokens are short-lived,
	// in production you almost always want to use an authentication method that enables the client library
	// to renew the token automatically when the previous one expires, such as AuthURL or AuthCallback.
	// Read more about [Token authentication] (RSA4a, RSA4, TO3j2).
	//
	// [Token authentication]: https://ably.com/docs/core-features/authentication#token-authentication
	Token string

	// TokenDetails is an authentication token with type [ably.TokenDetails].
	// Only TokenDetails.Token can be set via token string (obtained from the token property of a [ably.TokenDetails]
	// component of an Ably [ably.TokenRequest] response, or a JSON Web Token satisfying the Ably requirements for JWTs).
	// This option is mostly useful for testing: since tokens are short-lived,
	// in production you almost always want to use an authentication method that enables the client library
	// to renew the token automatically when the previous one expires, such as AuthURL or AuthCallback.
	// Read more about [Token authentication] (RSA4a, RSA4, TO3j3).
	//
	// [Token authentication]: https://ably.com/docs/core-features/authentication#token-authentication
	TokenDetails *TokenDetails

	// AuthMethod specifies HTTP verb to use for any request made to the AuthURL, either GET or POST for
	// getting token information [ably.TokenRequest] or [ably.TokenDetails].
	// The default value is GET (RSA8c, TO3j7, AO2d)
	AuthMethod string

	// AuthHeaders are key-value pair HTTP request headers to be added to any request made to the AuthURL.
	// Useful when an application requires these to be added to validate the request or implement the response.
	// If the authHeaders object contains an authorization key, then withCredentials is set on the XHR request.
	// (RSA8c3, TO3j8, AO2e).
	AuthHeaders http.Header

	// AuthParams are key-value pair HTTP query params to be added to any request made to the AuthURL.
	// When the authMethod is GET, query params are added to the URL, whereas when authMethod is POST,
	// the params are sent as URL encoded form data. Useful when an application requires these to be added to
	// validate the request or implement the response (RSA8c3, RSA8c1, TO3j9, AO2f).
	AuthParams url.Values

	// UseQueryTime when set to true, the library queries the Ably servers for the current time when
	// issuing multiple [ably.TokenRequest] instead of relying on a locally-available time of day.
	// Knowing the time accurately is needed to create valid signed Ably [ably.TokenRequest],
	// so this option is useful for library instances on auth servers where for some reason
	// the server clock cannot be kept synchronized through normal means,
	// such as an NTP daemon. The server is queried for the current time once per client library instance
	// (which stores the offset from the local clock), so if using this option you should avoid instancing
	// a new version of the library for each request.
	// The default is false (RSA9d, TO3j10, AO2a).
	UseQueryTime bool

	// DefaultTokenParams when provided, it overrides the client library defaults when issuing new Ably Tokens
	// for multiple Ably [ably.TokenRequest] (TO3j11).
	DefaultTokenParams *TokenParams

	// UseTokenAuth when set to true, forces token authentication to be used by the library.
	// If a clientId is not specified in the [ably.ClientOption] or [ably.TokenParams],
	// then the Ably Token issued is anonymous (RSA4, RSA14, TO3j4).
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

// clientOptions passes additional client-specific properties to the [ably.NewREST] or to the [ably.NewRealtime].
// Properties set using [ably.clientOptions] are used instead of the [ably.defaultOptions] values.
type clientOptions struct {
	// authOptions Embedded an [ably.authOptions] object (TO3j).
	authOptions

	// RESTHost enables a non-default Ably host to be specified. For development environments only.
	// The default value is rest.ably.io (RSC12, TO3k2).
	RESTHost string

	// Deprecated: this property is deprecated and will be removed in a future version.
	// Enables default fallback hosts to be used (TO3k7).
	FallbackHostsUseDefault bool

	// FallbackHosts is an array of fallback hosts to be used in the case of an error necessitating
	// the use of an alternative host. If you have been provided a set of custom fallback hosts by Ably,
	// please specify them here (RSC15b, RSC15a, TO3k6).
	FallbackHosts []string

	// RealtimeHost enables a non-default Ably host to be specified for realtime connections.
	// For development environments only. The default value is realtime.ably.io (RTC1d, TO3k3).
	RealtimeHost string

	// Environment enables a custom environment to be used with the Ably service.
	// Optional: prefixes both hostname with the environment string (RSC15b, TO3k1).
	Environment string

	// Port is used for non-TLS connections and requests
	Port int

	// TLSPort enables a non-default Ably port to be specified.
	// This is used for TLS connections and requests and restricted to development environments only.
	// The default value is 80 (TO3k4)>
	TLSPort int

	// ClientID is used for identifying this client when publishing messages or for presence purposes.
	// The clientId can be any non-empty string, except it cannot contain a *.
	// This option is primarily intended to be used in situations where the library is instantiated with a key.
	// Note that a clientId may also be implicit in a token used to instantiate the library.
	// An error will be raised if a clientId specified here conflicts with the clientId implicit in the token.
	// (RSC17, RSA4, RSA15, TO3a).
	ClientID string

	// Recover enables a connection to inherit the state of a previous connection that may have existed
	// under a different instance of the Realtime library. This might typically be used by clients of the browser
	// library to ensure connection state can be preserved when the user refreshes the page.
	// A recovery key string can be explicitly provided, or alternatively if a callback function is provided,
	// the client library will automatically persist the recovery key between page reloads and call the callback
	// when the connection is recoverable. The callback is then responsible for confirming whether the connection
	// should be recovered or not. See connection state recovery for further information (RTC1c, TO3i, RTN16i).
	Recover string

	// TransportParams is a set of key-value pairs that can be used to pass in arbitrary connection parameters,
	// such as heartbeatInterval or remainPresentFor (RTC1f).
	TransportParams url.Values

	// HTTPMaxRetryCount denotes the maximum number of fallback hosts to use as a fallback when an HTTP request
	// to the primary host is unreachable or indicates that it is unserviceable. The default value is 3 (TO3l5).
	HTTPMaxRetryCount int

	// HTTPRequestTimeout is a timeout for a client performing a complete HTTP request to Ably, including the connection phase.
	// Will only be used if no custom HTTPClient is set.
	// The default is 10 seconds (TO3l4).
	HTTPRequestTimeout time.Duration

	// FallbackRetryTimeout is the max time in milliseconds before HTTP requests are retried against the default endpoint.
	// The default is 600 seconds (TO3l10).
	FallbackRetryTimeout time.Duration

	// NoTLS when set to true, the client will use an insecure connection.
	// The default is false, meaning a TLS connection will be used to connect to Ably (RSC18, TO3d).
	NoTLS bool

	// NoConnect when set to false, the client connects to Ably as soon as it is instantiated.
	// You can set this to true and explicitly connect to Ably using the [ably.Connection]#connect()
	// The default is false (RTC1b, TO3e).
	NoConnect bool

	// NoEcho if set to true, prevents messages originating from this connection being echoed back
	// on the same connection.
	// The default is false (RTC1a, TO3h).
	NoEcho bool

	// NoQueueing if set to true, this disables the default behavior whereby the library queues messages on a
	// connection in the disconnected or connecting states. The default behavior enables applications to
	// submit messages immediately upon instantiating the library without having to wait for the connection
	// to be established. Applications may use this option to disable queueing if they wish to have
	// application-level control over the queueing.
	// The default is false (RTP16b, TO3g).
	NoQueueing bool

	// NoBinaryProtocol when set to true, JSON text encoding is used.
	// When false, the more efficient MsgPack binary encoding is used.
	// The default is true (TO3f).
	NoBinaryProtocol bool

	// IdempotentRESTPublishing when set to true, enables idempotent publishing by assigning a
	// unique message ID client-side, allowing the Ably servers to discard automatic publish retries
	// following a failure such as a network fault.
	// The default is true (RSL1k1, RTL6a1, TO3n).
	IdempotentRESTPublishing bool

	// Deprecated: use RealtimeRequestTimeout instead.
	// TimeoutConnect is a timeout for the wait of acknowledgement for operations performed via a realtime connection,
	// before the client library considers a request failed and triggers a failure condition.
	// Operations include establishing a connection with Ably, or sending a HEARTBEAT, CONNECT, ATTACH, DETACH
	// or CLOSE request. It is the equivalent of httpRequestTimeout but for realtime operations, rather than REST.
	// The default is 10 seconds (TO3l11).
	TimeoutConnect    time.Duration
	TimeoutDisconnect time.Duration // time period after which disconnect request is failed

	ConnectionStateTTL time.Duration //(DF1a)

	// RealtimeRequestTimeout is the timeout for the wait of acknowledgement for operations performed via a
	// realtime connection, before the client library considers a request failed and triggers a failure condition.
	// Operations include establishing a connection with Ably, or sending a HEARTBEAT, CONNECT, ATTACH, DETACH or
	// CLOSE request. It is the equivalent of httpRequestTimeout but for realtime operations, rather than REST.
	// The default is 10 seconds (TO3l11).
	RealtimeRequestTimeout time.Duration

	// DisconnectedRetryTimeout when the connection enters the [ably.ConnectionStateDisconnected] state, after this
	// timeout, if the state is still [ably.ConnectionStateDisconnected], the client library will attempt
	// to reconnect automatically.
	// The default is 15 seconds (TO3l1).
	DisconnectedRetryTimeout time.Duration

	// SuspendedRetryTimeout is the timeout when the connection enters the [ably.ConnectionStateSuspended] state,
	// after this timeout, if the state is still [ably.ConnectionStateSuspended], the client library attempts
	// to reconnect automatically.
	// The default is 30 seconds (RTN14d, TO3l2).
	SuspendedRetryTimeout time.Duration

	// ChannelRetryTimeout when a channel becomes [ably.ChannelStateSuspended} following a server initiated
	// [ably.ChannelStateDetached], after this delay, if the channel is still [ably.ChannelStateSuspended]
	// and the connection is in [ably.ConnectionStateConnected], the client library will attempt to re-attach
	// the channel automatically.
	// The default is 15 seconds (RTL13b, TO3l7).
	ChannelRetryTimeout time.Duration

	// HTTPOpenTimeout is timeout for opening a connection to Ably to initiate an HTTP request.
	// The default is 4 seconds (TO3l3).
	HTTPOpenTimeout time.Duration

	// Dial specifies the dial function for creating message connections used by Realtime.
	// If Dial is nil, the default websocket connection is used.
	Dial func(protocol string, u *url.URL, timeout time.Duration) (conn, error)

	// HTTPClient specifies the client used for HTTP communication by REST.
	// When set to nil, a client configured with default settings is used.
	HTTPClient *http.Client

	//Trace when provided this will be used on every request.
	Trace *httptrace.ClientTrace

	// Now returns the time the library should take as current.
	Now   func() time.Time
	After func(context.Context, time.Duration) <-chan time.Time

	// Product/version key-value pairs to include in the agent library
	// identifiers. This must only be used by Ably-authored SDKs (RSC7d6).
	Agents map[string]string

	// LogLevel controls the verbosity of the logs output from the library.
	// Levels include verbose, debug, info, warn and error.
	// platform specific (TO3b)
	LogLevel LogLevel

	// LogHandler controls the log output of the library. This is a function to handle each line of log output.
	// platform specific (TO3c)
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

func (opts *clientOptions) realtimeURL(realtimeHost string) (realtimeUrl string) {
	baseUrl := realtimeHost
	_, _, err := net.SplitHostPort(baseUrl)
	if err != nil { // set port if not set in provided realtimeHost
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

// RTN17c
func (opts *clientOptions) hasActiveInternetConnection() bool {
	res, err := opts.httpclient().Get(internetCheckUrl)
	if err != nil || res.StatusCode != 200 {
		return false
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(internetCheckOk))
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

// ClientOption configures a [ably.REST] or [ably.Realtime] instance.
//
// See: https://www.ably.io/documentation/realtime/usage#client-options
type ClientOption func(*clientOptions)

// AuthOption configures authentication/authorization for a [ably.REST] or [ably.Realtime]
// instance or operation.
type AuthOption func(*authOptions)

// Tokener is or can be used to get a [ably.TokenDetails].
type Tokener interface {
	IsTokener()
	isTokener()
}

// TokenString is the string representation of an authentication token.
type TokenString string

func (TokenString) IsTokener() {}
func (TokenString) isTokener() {}

// AuthWithCallback is used for setting AuthCallback function using [ably.AuthOption].
//
// AuthCallback function is called when a new token is required.
// The role of the callback is to obtain a fresh token, one of
//
//  1. An Ably Token string (https://ably.com/docs/core-features/authentication#token-process)
//  2. A signed [ably.TokenRequest] (https://ably.com/docs/core-features/authentication#token-request-process)
//  3. An [ably.TokenDetails] (https://ably.com/docs/core-features/authentication#token-process)
//  4. [An Ably JWT].
//
// See the [authentication doc] for more details and associated API calls (RSA4a, RSA4, TO3j5, AO2b).
//
// [authentication doc]: https://ably.com/docs/core-features/authentication
// [An Ably JWT]: https://ably.com/docs/core-features/authentication#ably-jwt-process
func AuthWithCallback(authCallback func(context.Context, TokenParams) (Tokener, error)) AuthOption {
	return func(os *authOptions) {
		os.AuthCallback = authCallback
	}
}

// AuthWithParams is used for setting AuthParams using [ably.AuthOption].
// AuthParams are key-value pair HTTP query params to be added to any request made to the AuthURL.
// When the authMethod is GET, query params are added to the URL, whereas when authMethod is POST,
// the params are sent as URL encoded form data. Useful when an application requires these to be added to
// validate the request or implement the response (RSA8c3, RSA8c1, TO3j9, AO2f).
func AuthWithParams(params url.Values) AuthOption {
	return func(os *authOptions) {
		os.AuthParams = params
	}
}

// AuthWithURL is used for setting AuthURL using [ably.AuthOption].
// AuthURL is a url that library will use to obtain
//  1. An Ably Token string (https://ably.com/docs/core-features/authentication#token-process)
//  2. A signed [ably.TokenRequest] (https://ably.com/docs/core-features/authentication#token-request-process)
//  3. An [ably.TokenDetails] (https://ably.com/docs/core-features/authentication#token-process)
//  4. [An Ably JWT].
//
// See the [authentication doc] for more details and associated API calls (RSA4a, RSA4, RSA8c, TO3j6, AO2c).
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
// [authentication doc]: https://ably.com/docs/core-features/authentication
// [An Ably JWT]: https://ably.com/docs/core-features/authentication#ably-jwt-process
func AuthWithURL(url string) AuthOption {
	return func(os *authOptions) {
		os.AuthURL = url
	}
}

// AuthWithMethod is used for setting AuthMethod using [ably.AuthOption]
// AuthMethod specifies HTTP verb to use for any request made to the AuthURL, either GET or POST for
// getting token information [ably.TokenRequest] or [ably.TokenDetails].
// The default value is GET (RSA8c, TO3j7, AO2d).
func AuthWithMethod(method string) AuthOption {
	return func(os *authOptions) {
		os.AuthMethod = method
	}
}

// AuthWithHeaders is used for setting AuthHeaders using [ably.AuthOption].
// AuthHeaders are key-value pair HTTP request headers to be added to any request made to the AuthURL.
// Useful when an application requires these to be added to validate the request or implement the response.
// If the authHeaders object contains an authorization key, then withCredentials is set on the XHR request.
// (RSA8c3, TO3j8, AO2e).
func AuthWithHeaders(headers http.Header) AuthOption {
	return func(os *authOptions) {
		os.AuthHeaders = headers
	}
}

// AuthWithKey is used for setting root/non-root apikey using [ably.AuthOption].
// Key is a full API key string, as obtained from the Ably dashboard.
// Use this option if you wish to use Basic authentication, or wish to be able to issue Ably Tokens
// without needing to defer to a separate entity to sign multiple [ably.TokenRequest].
// Read more about [Basic authentication] (RSA11, RSA14, TO3j1, AO2a).
//
// [Basic authentication]: https://ably.com/docs/core-features/authentication#basic-authentication
func AuthWithKey(key string) AuthOption {
	return func(os *authOptions) {
		os.Key = key
	}
}

// AuthWithQueryTime is used for setting UseQueryTime token using [ably.AuthOption].
// UseQueryTime when set to true, the library queries the Ably servers for the current time when
// issuing multiple [ably.TokenRequest] instead of relying on a locally-available time of day.
// Knowing the time accurately is needed to create valid signed Ably [ably.TokenRequest],
// so this option is useful for library instances on auth servers where for some reason
// the server clock cannot be kept synchronized through normal means,
// such as an NTP daemon. The server is queried for the current time once per client library instance
// (which stores the offset from the local clock), so if using this option you should avoid instancing
// a new version of the library for each request.
// The default is false (RSA9d, TO3j10, AO2a).
func AuthWithQueryTime(queryTime bool) AuthOption {
	return func(os *authOptions) {
		os.UseQueryTime = queryTime
	}
}

// AuthWithToken is used for setting authenticated token using [ably.AuthOption].
// This can either be a token string (obtained from the token property of a [ably.TokenDetails] component of
// an Ably [ably.TokenRequest] response, or a JSON Web Token satisfying the Ably requirements for JWTs).
//
// This option is mostly useful for testing: since tokens are short-lived,
// in production you almost always want to use an authentication method that enables the client library
// to renew the token automatically when the previous one expires, such as AuthURL or authCallback.
// Read more about [Token authentication] (RSA4a, RSA4, TO3j2).
//
// [Token authentication]: https://ably.com/docs/core-features/authentication#token-authentication
func AuthWithToken(token string) AuthOption {
	return func(os *authOptions) {
		os.Token = token
	}
}

// AuthWithTokenDetails is used for setting authenticated [ably.TokenDetails] object using [ably.AuthOption].
// Only TokenDetails.Token can be set via token string (obtained from the token property of a [ably.TokenDetails]
// component of an Ably [ably.TokenRequest] response, or a JSON Web Token satisfying the Ably requirements for JWTs).
// This option is mostly useful for testing: since tokens are short-lived,
// in production you almost always want to use an authentication method that enables the client library
// to renew the token automatically when the previous one expires, such as AuthURL or authCallback.
// Read more about [Token authentication] (RSA4a, RSA4, TO3j3).
//
// [Token authentication]: https://ably.com/docs/core-features/authentication#token-authentication
func AuthWithTokenDetails(details *TokenDetails) AuthOption {
	return func(os *authOptions) {
		os.TokenDetails = details
	}
}

// AuthWithUseTokenAuth is used for setting UseTokenAuth using [ably.AuthOption].
// UseTokenAuth when set to true, forces token authentication to be used by the library.
// If a clientId is not specified in the [ably.ClientOption] or [ably.TokenParams],
// then the Ably Token issued is anonymous (RSA4, RSA14, TO3j4).
func AuthWithUseTokenAuth(use bool) AuthOption {
	return func(os *authOptions) {
		os.UseTokenAuth = use
	}
}

// AuthWithDefaultTokenParams is used for setting DefaultTokenParams token using [ably.AuthOption].
// DefaultTokenParams when provided, it overrides the client library defaults when issuing new Ably Tokens
// for multiple Ably [ably.TokenRequest] (TO3j11).
func AuthWithDefaultTokenParams(params TokenParams) AuthOption {
	return func(os *authOptions) {
		os.DefaultTokenParams = &params
	}
}

// WithAuthCallback is used for setting authCallback function using [ably.ClientOption].
// AuthCallback function is called when a new token is required.
// The role of the callback is to obtain a fresh token, one of
//
//  1. An Ably Token string (https://ably.com/docs/core-features/authentication#token-process)
//  2. A signed [ably.TokenRequest] (https://ably.com/docs/core-features/authentication#token-request-process)
//  3. An [ably.TokenDetails] (https://ably.com/docs/core-features/authentication#token-process)
//  4. [An Ably JWT].
//
// See the [authentication doc] for more details and associated API calls (RSA4a, RSA4, TO3j5, AO2b).
//
// [authentication doc]: https://ably.com/docs/core-features/authentication
// [An Ably JWT]: https://ably.com/docs/core-features/authentication#ably-jwt-process
func WithAuthCallback(authCallback func(context.Context, TokenParams) (Tokener, error)) ClientOption {
	return func(os *clientOptions) {
		os.AuthCallback = authCallback
	}
}

// WithAuthParams is used for setting AuthParams using [ably.ClientOption].
// AuthParams are key-value pair HTTP query params to be added to any request made to the AuthURL.
// When the authMethod is GET, query params are added to the URL, whereas when authMethod is POST,
// the params are sent as URL encoded form data. Useful when an application requires these to be added to
// validate the request or implement the response (RSA8c3, RSA8c1, TO3j9, AO2f).
func WithAuthParams(params url.Values) ClientOption {
	return func(os *clientOptions) {
		os.AuthParams = params
	}
}

// WithAuthURL is used for setting AuthURL using [ably.ClientOption].
// AuthURL is a url that library will use to obtain
//  1. An Ably Token string (https://ably.com/docs/core-features/authentication#token-process)
//  2. A signed [ably.TokenRequest] (https://ably.com/docs/core-features/authentication#token-request-process)
//  3. An [ably.TokenDetails] (https://ably.com/docs/core-features/authentication#token-process)
//  4. [An Ably JWT].
//
// See the [authentication doc] for more details and associated API calls (RSA4a, RSA4, RSA8c, TO3j6, AO2c).
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
// [authentication doc]: https://ably.com/docs/core-features/authentication
// [An Ably JWT]: https://ably.com/docs/core-features/authentication#ably-jwt-process
func WithAuthURL(url string) ClientOption {
	return func(os *clientOptions) {
		os.AuthURL = url
	}
}

// WithAuthMethod is used for setting AuthMethod using [ably.ClientOption].
// AuthMethod specifies HTTP verb to use for any request made to the AuthURL, either GET or POST for
// getting token information [ably.TokenRequest] or [ably.TokenDetails].
// The default value is GET (RSA8c, TO3j7, AO2d)
func WithAuthMethod(method string) ClientOption {
	return func(os *clientOptions) {
		os.AuthMethod = method
	}
}

// WithAuthHeaders is used for setting AuthHeaders using [ably.ClientOption].
// AuthHeaders are key-value pair HTTP request headers to be added to any request made to the AuthURL.
// Useful when an application requires these to be added to validate the request or implement the response.
// If the authHeaders object contains an authorization key, then withCredentials is set on the XHR request.
// (RSA8c3, TO3j8, AO2e).
func WithAuthHeaders(headers http.Header) ClientOption {
	return func(os *clientOptions) {
		os.AuthHeaders = headers
	}
}

// WithKey is used for setting root/non-root apikey using [ably.ClientOption].
// Key is a full API key string, as obtained from the Ably dashboard.
// Use this option if you wish to use Basic authentication, or wish to be able to issue Ably Tokens
// without needing to defer to a separate entity to sign multiple [ably.TokenRequest].
// Read more about [Basic authentication] (RSC1, RSA11, RSA14, TO3j1, AO2a).
//
// [Basic authentication]: https://ably.com/docs/core-features/authentication#basic-authentication
func WithKey(key string) ClientOption {
	return func(os *clientOptions) {
		os.Key = key
	}
}

// WithDefaultTokenParams is used for setting DefaultTokenParams token using [ably.ClientOption].
// DefaultTokenParams when provided, it overrides the client library defaults when issuing new Ably Tokens
// for multiple Ably [ably.TokenRequest] (TO3j11).
func WithDefaultTokenParams(params TokenParams) ClientOption {
	return func(os *clientOptions) {
		os.DefaultTokenParams = &params
	}
}

// WithQueryTime is used for setting UseQueryTime token using [ably.ClientOption].
// UseQueryTime when set to true, the library queries the Ably servers for the current time when
// issuing multiple [ably.TokenRequest] instead of relying on a locally-available time of day.
// Knowing the time accurately is needed to create valid signed Ably [ably.TokenRequest],
// so this option is useful for library instances on auth servers where for some reason
// the server clock cannot be kept synchronized through normal means,
// such as an NTP daemon. The server is queried for the current time once per client library instance
// (which stores the offset from the local clock), so if using this option you should avoid instancing
// a new version of the library for each request.
// The default is false (RSA9d, TO3j10, AO2a).
func WithQueryTime(queryTime bool) ClientOption {
	return func(os *clientOptions) {
		os.UseQueryTime = queryTime
	}
}

// WithToken is used for setting authenticated token using [ably.ClientOption].
// This can either be a token string (obtained from the token property of a [ably.TokenDetails] component of
// an Ably [ably.TokenRequest] response, or a JSON Web Token satisfying the Ably requirements for JWTs).
//
// This option is mostly useful for testing: since tokens are short-lived,
// in production you almost always want to use an authentication method that enables the client library
// to renew the token automatically when the previous one expires, such as AuthURL or authCallback.
// Read more about [Token authentication] (RSA4a, RSA4, TO3j2).
//
// [Token authentication]: https://ably.com/docs/core-features/authentication#token-authentication
func WithToken(token string) ClientOption {
	return func(os *clientOptions) {
		os.Token = token
	}
}

// WithTokenDetails is used for setting authenticated [ably.TokenDetails] object using [ably.ClientOption].
// Only TokenDetails.Token can be set via token string (obtained from the token property of a [ably.TokenDetails]
// component of an Ably [ably.TokenRequest] response, or a JSON Web Token satisfying the Ably requirements for JWTs).
// This option is mostly useful for testing: since tokens are short-lived,
// in production you almost always want to use an authentication method that enables the client library
// to renew the token automatically when the previous one expires, such as AuthURL or authCallback.
// Read more about [Token authentication] (RSA4a, RSA4, TO3j3).
//
// [Token authentication]: https://ably.com/docs/core-features/authentication#token-authentication
func WithTokenDetails(details *TokenDetails) ClientOption {
	return func(os *clientOptions) {
		os.TokenDetails = details
	}
}

// WithUseTokenAuth is used for setting UseTokenAuth using [ably.ClientOption].
// UseTokenAuth when set to true, forces token authentication to be used by the library.
// If a clientId is not specified in the [ably.ClientOption] or [ably.TokenParams],
// then the Ably Token issued is anonymous (RSA4, RSA14, TO3j4).
func WithUseTokenAuth(use bool) ClientOption {
	return func(os *clientOptions) {
		os.UseTokenAuth = use
	}
}

// WithAutoConnect is used for setting NoConnect using [ably.ClientOption].
// NoConnect when set to false, the client connects to Ably as soon as it is instantiated.
// You can set this to true and explicitly connect to Ably using the [ably.Connection]#connect()
// The default is false (RTC1b, TO3e).
func WithAutoConnect(autoConnect bool) ClientOption {
	return func(os *clientOptions) {
		os.NoConnect = !autoConnect
	}
}

// WithClientID is used for setting ClientID using [ably.ClientOption].
// ClientID is used for identifying this client when publishing messages or for presence purposes.
// The clientId can be any non-empty string, except it cannot contain a *.
// This option is primarily intended to be used in situations where the library is instantiated with a key.
// Note that a clientId may also be implicit in a token used to instantiate the library.
// An error will be raised if a clientId specified here conflicts with the clientId implicit in the token.
// (RSC17, RSA4, RSA15, TO3a).
func WithClientID(clientID string) ClientOption {
	return func(os *clientOptions) {
		os.ClientID = clientID
	}
}

// WithFallbackRetryTimeout is used for setting FallbackRetryTimeout using [ably.ClientOption].
// FallbackRetryTimeout is the max time in milliseconds before HTTP requests are retried against the default endpoint.
// The default is 600 seconds (TO3l10).
func WithFallbackRetryTimeout(fallbackRetryTimeout time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.FallbackRetryTimeout = fallbackRetryTimeout
	}
}

// WithEchoMessages is used for setting NoEcho using [ably.ClientOption].
// NoEcho if set to true, prevents messages originating from this connection being echoed back
// on the same connection.
// The default is false (RTC1a, TO3h).
func WithEchoMessages(echo bool) ClientOption {
	return func(os *clientOptions) {
		os.NoEcho = !echo
	}
}

// WithEnvironment is used for setting Environment using [ably.ClientOption].
// Environment enables a custom environment to be used with the Ably service.
// Optional: prefixes both hostname with the environment string (RSC15b, TO3k1).
func WithEnvironment(env string) ClientOption {
	return func(os *clientOptions) {
		os.Environment = env
	}
}

// WithLogHandler is used for setting LogHandler using [ably.ClientOption].
// LogHandler controls the log output of the library. This is a function to handle each line of log output.
// platform specific (TO3c)
func WithLogHandler(handler Logger) ClientOption {
	return func(os *clientOptions) {
		os.LogHandler = handler
	}
}

// WithLogLevel is used for setting LogLevel using [ably.ClientOption].
// LogLevel controls the verbosity of the logs output from the library.
// Levels include verbose, debug, info, warn and error.
// platform specific (TO3b)
func WithLogLevel(level LogLevel) ClientOption {
	return func(os *clientOptions) {
		os.LogLevel = level
	}
}

// WithAgents is used to add product/version key-value pairs to include in the
// agent library identifiers. This must only be used by Ably-authored SDKs.
func WithAgents(agents map[string]string) ClientOption {
	return func(os *clientOptions) {
		os.Agents = agents
	}
}

// WithPort is used for setting custom Port using [ably.ClientOption].
// Port is used for non-TLS connections and requests
func WithPort(port int) ClientOption {
	return func(os *clientOptions) {
		os.Port = port
	}
}

// WithQueueMessages is used for setting NoQueueing using [ably.ClientOption].
// NoQueueing if set to true, this disables the default behavior whereby the library queues messages on a
// connection in the disconnected or connecting states. The default behavior enables applications to
// submit messages immediately upon instantiating the library without having to wait for the connection
// to be established. Applications may use this option to disable queueing if they wish to have
// application-level control over the queueing.
// The default is false (RTP16b, TO3g).
func WithQueueMessages(queue bool) ClientOption {
	return func(os *clientOptions) {
		os.NoQueueing = !queue
	}
}

// WithRESTHost is used for setting RESTHost using [ably.ClientOption].
// RESTHost enables a non-default Ably host to be specified. For development environments only.
// The default value is rest.ably.io (RSC12, TO3k2).
func WithRESTHost(host string) ClientOption {
	return func(os *clientOptions) {
		os.RESTHost = host
	}
}

// WithHTTPRequestTimeout is used for setting HTTPRequestTimeout using [ably.ClientOption].
// HTTPRequestTimeout is a timeout for a client performing a complete HTTP request to Ably, including the connection phase.
// Will only be used if no custom HTTPClient is set.
// The default is 10 seconds (TO3l4).
func WithHTTPRequestTimeout(timeout time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.HTTPRequestTimeout = timeout
	}
}

// WithRealtimeHost is used for setting RealtimeHost using [ably.ClientOption].
// RealtimeHost enables a non-default Ably host to be specified for realtime connections.
// For development environments only. The default value is realtime.ably.io (RTC1d, TO3k3).
func WithRealtimeHost(host string) ClientOption {
	return func(os *clientOptions) {
		os.RealtimeHost = host
	}
}

// WithFallbackHosts is used for setting FallbackHosts using [ably.ClientOption].
// FallbackHosts is an array of fallback hosts to be used in the case of an error necessitating
// the use of an alternative host. If you have been provided a set of custom fallback hosts by Ably,
// please specify them here (RSC15b, RSC15a, TO3k6).
func WithFallbackHosts(hosts []string) ClientOption {
	return func(os *clientOptions) {
		os.FallbackHosts = hosts
	}
}

// WithRecover is used for setting Recover using [ably.ClientOption].
// Recover enables a connection to inherit the state of a previous connection that may have existed
// under a different instance of the Realtime library. This might typically be used by clients of the browser
// library to ensure connection state can be preserved when the user refreshes the page.
// A recovery key string can be explicitly provided, or alternatively if a callback function is provided,
// the client library will automatically persist the recovery key between page reloads and call the callback
// when the connection is recoverable. The callback is then responsible for confirming whether the connection
// should be recovered or not. See connection state recovery for further information (RTC1c, TO3i).
func WithRecover(key string) ClientOption {
	return func(os *clientOptions) {
		os.Recover = key
	}
}

// WithTLS is used for setting NoTLS using [ably.ClientOption].
// NoTLS when set to true, the client will use an insecure connection.
// The default is false, meaning a TLS connection will be used to connect to Ably (RSC18, TO3d).
func WithTLS(tls bool) ClientOption {
	return func(os *clientOptions) {
		os.NoTLS = !tls
	}
}

// WithTLSPort is used for setting TLSPort using [ably.ClientOption].
// TLSPort enables a non-default Ably port to be specified.
// This is used for TLS connections and requests and restricted to development environments only.
// The default value is 80 (TO3k4)>
func WithTLSPort(port int) ClientOption {
	return func(os *clientOptions) {
		os.TLSPort = port
	}
}

// WithUseBinaryProtocol is used for setting NoBinaryProtocol using [ably.ClientOption].
// NoBinaryProtocol when set to true, JSON text encoding is used.
// When false, the more efficient MsgPack binary encoding is used.
// The default is true (TO3f).
func WithUseBinaryProtocol(use bool) ClientOption {
	return func(os *clientOptions) {
		os.NoBinaryProtocol = !use
	}
}

// WithTransportParams is used for setting TransportParams using [ably.ClientOption].
// TransportParams is a set of key-value pairs that can be used to pass in arbitrary connection parameters,
// such as heartbeatInterval or remainPresentFor (RTC1f).
func WithTransportParams(params url.Values) ClientOption {
	return func(os *clientOptions) {
		os.TransportParams = params
	}
}

// WithDisconnectedRetryTimeout is used for setting DisconnectedRetryTimeout using [ably.ClientOption].
// DisconnectedRetryTimeout when the connection enters the [ably.ConnectionStateDisconnected] state, after this
// timeout, if the state is still [ably.ConnectionStateDisconnected], the client library will attempt
// to reconnect automatically.
// The default is 15 seconds (TO3l1).
func WithDisconnectedRetryTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.DisconnectedRetryTimeout = d
	}
}

// WithHTTPOpenTimeout is used for setting HTTPOpenTimeout using [ably.ClientOption].
// HTTPOpenTimeout is timeout for opening a connection to Ably to initiate an HTTP request.
// The default is 4 seconds (TO3l3).
func WithHTTPOpenTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.HTTPOpenTimeout = d
	}
}

// WithRealtimeRequestTimeout is used for setting RealtimeRequestTimeout using [ably.ClientOption].
// RealtimeRequestTimeout is the timeout for the wait of acknowledgement for operations performed via a
// realtime connection, before the client library considers a request failed and triggers a failure condition.
// Operations include establishing a connection with Ably, or sending a HEARTBEAT, CONNECT, ATTACH, DETACH or
// CLOSE request. It is the equivalent of httpRequestTimeout but for realtime operations, rather than REST.
// The default is 10 seconds (TO3l11).
func WithRealtimeRequestTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.RealtimeRequestTimeout = d
	}
}

// WithSuspendedRetryTimeout is used for setting SuspendedRetryTimeout using [ably.ClientOption].
// SuspendedRetryTimeout is the timeout when the connection enters the [ably.ConnectionStateSuspended] state,
// after this timeout, if the state is still [ably.ConnectionStateSuspended], the client library attempts
// to reconnect automatically.
// The default is 30 seconds (RTN14d, TO3l2).
func WithSuspendedRetryTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.SuspendedRetryTimeout = d
	}
}

// WithChannelRetryTimeout is used for setting ChannelRetryTimeout using [ably.ClientOption].
// ChannelRetryTimeout when a channel becomes [ably.ChannelStateSuspended} following a server initiated
// [ably.ChannelStateDetached], after this delay, if the channel is still [ably.ChannelStateSuspended]
// and the connection is in [ably.ConnectionStateConnected], the client library will attempt to re-attach
// the channel automatically.
// The default is 15 seconds (RTL13b, TO3l7).
func WithChannelRetryTimeout(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.ChannelRetryTimeout = d
	}
}

// WithHTTPMaxRetryCount is used for setting HTTPMaxRetryCount using [ably.ClientOption].
// HTTPMaxRetryCount denotes the maximum number of fallback hosts to use as a fallback when an HTTP request
// to the primary host is unreachable or indicates that it is unserviceable. The default value is 3 (TO3l5).
func WithHTTPMaxRetryCount(count int) ClientOption {
	return func(os *clientOptions) {
		os.HTTPMaxRetryCount = count
	}
}

// WithIdempotentRESTPublishing is used for setting IdempotentRESTPublishing using [ably.ClientOption].
// IdempotentRESTPublishing when set to true, enables idempotent publishing by assigning a
// unique message ID client-side, allowing the Ably servers to discard automatic publish retries
// following a failure such as a network fault.
// The default is true (RSL1k1, RTL6a1, TO3n).
func WithIdempotentRESTPublishing(idempotent bool) ClientOption {
	return func(os *clientOptions) {
		os.IdempotentRESTPublishing = idempotent
	}
}

// WithHTTPClient is used for setting HTTPClient using [ably.ClientOption].
// HTTPClient specifies the client used for HTTP communication by REST.
// When set to nil, a client configured with default settings is used.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(os *clientOptions) {
		os.HTTPClient = client
	}
}

// Deprecated: This function is deprecated and will be removed in a future versions.
// WithFallbackHostsUseDefault is used for setting FallbackHostsUseDefault using [ably.ClientOption].
// Deprecated: this property is deprecated and will be removed in a future version.
// Enables default fallback hosts to be used (TO3k7).
func WithFallbackHostsUseDefault(fallbackHostsUseDefault bool) ClientOption {
	return func(os *clientOptions) {
		os.FallbackHostsUseDefault = fallbackHostsUseDefault
	}
}

// WithDial is used for setting Dial using [ably.ClientOption].
// Dial specifies the dial function for creating message connections used by Realtime.
// If Dial is nil, the default websocket connection is used.
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
		if set != nil {
			set(&to)
		}
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
