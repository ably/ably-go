package ably

import (
	"context"
	"net/http"
	"net/http/httptrace"
	"time"
)

func (p *PaginatedResult) BuildPath(base, rel string) string {
	return p.buildPath(base, rel)
}

func (opts *clientOptions) RestURL() string {
	return opts.restURL()
}

func (opts *clientOptions) RealtimeURL() string {
	return opts.realtimeURL()
}

func (c *REST) Post(path string, in, out interface{}) (*http.Response, error) {
	return c.post(path, in, out)
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

// GetAndAttach is a helper method, which returns attached channel or panics if
// the attaching failed.
func (ch *Channels) GetAndAttach(name string) *RealtimeChannel {
	channel := ch.Get(name)
	if err := wait(channel.Attach()); err != nil {
		panic(`attach to "` + name + `" failed: ` + err.Error())
	}
	return channel
}

func (a *Auth) Timestamp(query bool) (time.Time, error) {
	return a.timestamp(query)
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

func (os ClientOptions) RealtimeRequestTimeout(d time.Duration) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.RealtimeRequestTimeout = d
	})
}

func (os ClientOptions) Trace(trace *httptrace.ClientTrace) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Trace = trace
	})
}

func (os ClientOptions) Now(now func() time.Time) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.Now = now
	})
}

func (os ClientOptions) After(after func(context.Context, time.Duration) <-chan time.Time) ClientOptions {
	return append(os, func(os *clientOptions) {
		os.After = after
	})
}

func (os ClientOptions) ApplyWithDefaults() *clientOptions {
	return os.applyWithDefaults()
}

type ConnStateChanges = connStateChanges

type ChannelStateChanges = channelStateChanges
