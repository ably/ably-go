package ably

import (
	"context"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"
)

func NewClientOptions(os ...ClientOption) *clientOptions {
	return applyOptionsWithDefaults(os...)
}

func GetEnvFallbackHosts(env string) []string {
	return getEnvFallbackHosts(env)
}

func (opts *clientOptions) GetRestHost() string {
	return opts.getRestHost()
}

func (opts *clientOptions) GetRealtimeHost() string {
	return opts.getRealtimeHost()
}

func (opts *clientOptions) ActivePort() (int, bool) {
	return opts.activePort()
}

func (opts *clientOptions) GetFallbackHosts() ([]string, error) {
	return opts.getFallbackHosts()
}

func (opts *clientOptions) RestURL() string {
	return opts.restURL()
}

func (opts *clientOptions) RealtimeURL() string {
	return opts.realtimeURL()
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

func (a *Auth) Timestamp(ctx context.Context, query bool) (time.Time, error) {
	return a.timestamp(ctx, query)
}

func (c *REST) Timestamp(query bool) (time.Time, error) {
	return c.Auth.timestamp(context.Background(), query)
}

func (a *Auth) SetServerTimeFunc(st func() (time.Time, error)) {
	a.serverTimeHandler = st
}

func (c *REST) SetSuccessFallbackHost(duration time.Duration) {
	c.successFallbackHost = &fallbackCache{duration: duration}
}

func (c *REST) GetCachedFallbackHost() string {
	return c.successFallbackHost.get()
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
	LibraryVersion    = libraryVersion
	AblyAgentHeader   = ablyAgentHeader
	AblySDKIdentifier = ablySDKIdentifier

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

var GoRuntimeIdentifier = goRuntimeIdentifier
var GoOSIdentifier = goOSIdentifier

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

func NewCBCCipher(opts CipherParams) (*cbcCipher, error) {
	return newCBCCipher(opts)
}

func (p *CipherParams) SetIV(iv []byte) {
	p.iv = iv
}
