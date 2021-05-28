package ably

import (
	"context"
	"net/http"
	"net/http/httptrace"
	"time"
)

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
	return c.pending.Len()
}

func NewInternalLogger(l Logger) logger {
	return logger{l: l}
}

type FilteredLogger = filteredLogger
