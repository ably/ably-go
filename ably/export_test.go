package ably

import (
	"net/http"
	"net/http/httptrace"
	"time"
)

func (p *PaginatedResult) BuildPath(base, rel string) string {
	return p.buildPath(base, rel)
}

func (opts *ClientOptions) RestURL() string {
	return opts.restURL()
}

func (opts *ClientOptions) RealtimeURL() string {
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

func ErrorCode(err error) int {
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

func (a *Auth) SetNowFunc(now func() time.Time) {
	a.now = now
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

func (opts *ClientOptions) GetFallbackRetryTimeout() time.Duration {
	return opts.fallbackRetryTimeout()
}

func NewErrorInfo(code int, err error) *ErrorInfo {
	return newError(code, err)
}

var NewEventEmitter = newEventEmitter

type EventEmitter = eventEmitter
type EmitterEvent = emitterEvent
type EmitterData = emitterData

type EmitterString string

func (EmitterString) isEmitterEvent() {}
func (EmitterString) isEmitterData()  {}

// TODO: Once channels have also an EventEmitter, refactor tests to use
// EventEmitters for both connection and channels.

func (c *Connection) OnState(ch chan<- State, states ...StateEnum) {
	c.onState(ch, states...)
}

func (c *Connection) OffState(ch chan<- State, states ...StateEnum) {
	c.offState(ch, states...)
}

func (os ClientOptionsV12) Listener(ch chan<- State) ClientOptionsV12 {
	return append(os, func(os *ClientOptions) {
		os.Listener = ch
	})
}

func (os ClientOptionsV12) RealtimeRequestTimeout(d time.Duration) ClientOptionsV12 {
	return append(os, func(os *ClientOptions) {
		os.RealtimeRequestTimeout = d
	})
}

func (os ClientOptionsV12) Trace(trace *httptrace.ClientTrace) ClientOptionsV12 {
	return append(os, func(os *ClientOptions) {
		os.Trace = trace
	})
}
