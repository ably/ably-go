package ably

import (
	"context"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
)

func NewClientOptions(os ...ClientOption) *clientOptions {
	return applyOptionsWithDefaults(os...)
}

func NewRestHosts(opts *clientOptions) *restHosts {
	return newRestHosts(opts)
}

func (restHosts *restHosts) GetPrimaryHost() string {
	return restHosts.getPrimaryHost()
}

func (restHosts *restHosts) NextFallbackHost() string {
	return restHosts.nextFallbackHost()
}

func (restHosts *restHosts) GetAllRemainingFallbackHosts() []string {
	var hosts []string
	for true {
		fallbackHost := restHosts.NextFallbackHost()
		if ablyutil.Empty(fallbackHost) {
			break
		}
		hosts = append(hosts, fallbackHost)
	}
	return hosts
}

func (restHosts *restHosts) ResetVisitedFallbackHosts() {
	restHosts.resetVisitedFallbackHosts()
}

func (restHosts *restHosts) FallbackHostsRemaining() int {
	return restHosts.fallbackHostsRemaining()
}

func (restHosts *restHosts) SetPrimaryFallbackHost(host string) {
	restHosts.setPrimaryFallbackHost(host)
}

func (restHosts *restHosts) GetPreferredHost() string {
	return restHosts.getPreferredHost()
}

func (restHosts *restHosts) CacheHost(host string) {
	restHosts.cacheHost(host)
}

func NewRealtimeHosts(opts *clientOptions) *realtimeHosts {
	return newRealtimeHosts(opts)
}

func (realtimeHosts *realtimeHosts) GetPrimaryHost() string {
	return realtimeHosts.getPrimaryHost()
}

func (realtimeHosts *realtimeHosts) NextFallbackHost() string {
	return realtimeHosts.nextFallbackHost()
}

func (realtimeHosts *realtimeHosts) GetAllRemainingFallbackHosts() []string {
	var hosts []string
	for true {
		fallbackHost := realtimeHosts.NextFallbackHost()
		if ablyutil.Empty(fallbackHost) {
			break
		}
		hosts = append(hosts, fallbackHost)
	}
	return hosts
}

func (realtimeHosts *realtimeHosts) ResetVisitedFallbackHosts() {
	realtimeHosts.resetVisitedFallbackHosts()
}

func (realtimeHosts *realtimeHosts) FallbackHostsRemaining() int {
	return realtimeHosts.fallbackHostsRemaining()
}

func (realtimeHosts *realtimeHosts) GetPreferredHost() string {
	return realtimeHosts.getPreferredHost()
}

func GetEnvFallbackHosts(env string) []string {
	return getEnvFallbackHosts(env)
}

func (opts *clientOptions) GetPrimaryRestHost() string {
	return opts.getPrimaryRestHost()
}

func (opts *clientOptions) GetPrimaryRealtimeHost() string {
	return opts.getPrimaryRealtimeHost()
}

func (opts *clientOptions) ActivePort() (int, bool) {
	return opts.activePort()
}

func (opts *clientOptions) GetFallbackHosts() ([]string, error) {
	return opts.getFallbackHosts()
}

func (opts *clientOptions) RestURL(restHost string) string {
	return opts.restURL(restHost)
}

func (opts *clientOptions) RealtimeURL(realtimeHost string) string {
	return opts.realtimeURL(realtimeHost)
}

func (opts *clientOptions) HasActiveInternetConnection() bool {
	return opts.hasActiveInternetConnection()
}

func (c *REST) Post(ctx context.Context, path string, in, out interface{}) (*http.Response, error) {
	return c.post(ctx, path, in, out)
}

const (
	AuthBasic = authBasic
	AuthToken = authToken
)

func (a *Auth) Method() int {
	return a.method
}

func DecodeResp(resp *http.Response, out interface{}) error {
	return decodeResp(resp, out)
}

func UnwrapErrorCode(err error) ErrorCode {
	return code(err)
}

func UnwrapStatusCode(err error) int {
	return statusCode(err)
}

func IsTimeoutOrDnsErr(err error) bool {
	return isTimeoutOrDnsErr(err)
}

func (a *Auth) Timestamp(ctx context.Context, query bool) (time.Time, error) {
	return a.timestamp(ctx, query)
}

func (c *REST) Timestamp(query bool) (time.Time, error) {
	return c.Auth.timestamp(context.Background(), query)
}

func (a *Auth) SetServerTimeFunc(st func() (time.Time, error)) {
	a.serverTimeHandler = st
}

func (c *REST) GetCachedFallbackHost() string {
	return c.hosts.cache.get()
}

func (c *RealtimeChannel) GetAttachResume() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.attachResume
}

func (c *RealtimeChannel) SetAttachResume(value bool) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.attachResume = value
}

func (opts *clientOptions) GetFallbackRetryTimeout() time.Duration {
	return opts.fallbackRetryTimeout()
}

func NewErrorInfo(code ErrorCode, err error) *ErrorInfo {
	return newError(code, err)
}

var NewEventEmitter = newEventEmitter

type EventEmitter = eventEmitter
type EmitterEvent = emitterEvent
type EmitterData = emitterData

type EmitterString string

func (EmitterString) isEmitterEvent() {}
func (EmitterString) isEmitterData()  {}

func (c *Connection) RemoveKey() {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.key = ""
}

func (c *Connection) MsgSerial() int64 {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.msgSerial
}

func WithTrace(trace *httptrace.ClientTrace) ClientOption {
	return func(os *clientOptions) {
		os.Trace = trace
	}
}

func WithNow(now func() time.Time) ClientOption {
	return func(os *clientOptions) {
		os.Now = now
	}
}

func WithAfter(after func(context.Context, time.Duration) <-chan time.Time) ClientOption {
	return func(os *clientOptions) {
		os.After = after
	}
}

func WithConnectionStateTTL(d time.Duration) ClientOption {
	return func(os *clientOptions) {
		os.ConnectionStateTTL = d
	}
}

func ApplyOptionsWithDefaults(o ...ClientOption) *clientOptions {
	return applyOptionsWithDefaults(o...)
}

type ConnStateChanges = connStateChanges

type ChannelStateChanges = channelStateChanges

const ConnectionStateTTLErrFmt = connectionStateTTLErrFmt

func DefaultFallbackHosts() []string {
	return defaultFallbackHosts()
}

// PendingItems returns the number of messages waiting for Ack/Nack
func (c *Connection) PendingItems() int {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return len(c.pending.queue)
}

func (c *Connection) ConnectionStateTTL() time.Duration {
	return c.connectionStateTTL()
}

func NewInternalLogger(l Logger) logger {
	return logger{l: l}
}

type FilteredLogger = filteredLogger

type ProtoAction = protoAction
type ProtoChannelOptions = protoChannelOptions
type Conn = conn
type ConnectionDetails = connectionDetails
type DurationFromMsecs = durationFromMsecs
type ProtoErrorInfo = errorInfo
type ProtoFlag = protoFlag
type ProtocolMessage = protocolMessage

const (
	DefaultCipherKeyLength = defaultCipherKeyLength
	DefaultCipherAlgorithm = defaultCipherAlgorithm
	DefaultCipherMode      = defaultCipherMode

	LibraryString     = libraryString
	AblyVersionHeader = ablyVersionHeader
	AblyVersion       = ablyVersion
	AblyLibHeader     = ablyLibHeader

	EncUTF8   = encUTF8
	EncJSON   = encJSON
	EncBase64 = encBase64
	EncCipher = encCipher

	ActionHeartbeat    = actionHeartbeat
	ActionAck          = actionAck
	ActionNack         = actionNack
	ActionConnect      = actionConnect
	ActionConnected    = actionConnected
	ActionDisconnect   = actionDisconnect
	ActionDisconnected = actionDisconnected
	ActionClose        = actionClose
	ActionClosed       = actionClosed
	ActionError        = actionError
	ActionAttach       = actionAttach
	ActionAttached     = actionAttached
	ActionDetach       = actionDetach
	ActionDetached     = actionDetached
	ActionPresence     = actionPresence
	ActionMessage      = actionMessage
	ActionSync         = actionSync

	FlagHasPresence       = flagHasPresence
	FlagHasBacklog        = flagHasBacklog
	FlagResumed           = flagResumed
	FlagTransient         = flagTransient
	FlagAttachResume      = flagAttachResume
	FlagPresence          = flagPresence
	FlagPublish           = flagPublish
	FlagSubscribe         = flagSubscribe
	FlagPresenceSubscribe = flagPresenceSubscribe
)

func MessageWithEncodedData(m Message, cipher channelCipher) (Message, error) {
	return m.withEncodedData(cipher)
}

func MessageWithDecodedData(m Message, cipher channelCipher) (Message, error) {
	return m.withDecodedData(cipher)
}

func ChannelModeFromFlag(flags ProtoFlag) []ChannelMode {
	return channelModeFromFlag(flags)
}

func ChannelModeToFlag(mode ChannelMode) ProtoFlag {
	return mode.toFlag()
}

func DialWebsocket(proto string, u *url.URL, timeout time.Duration) (Conn, error) {
	return dialWebsocket(proto, u, timeout)
}

func (p *CipherParams) SetIV(iv []byte) {
	p.iv = iv
}
