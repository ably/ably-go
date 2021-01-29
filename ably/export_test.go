package ably

import (
	"net/http"
	"time"
)

func (p *PaginatedResult) BuildPath(base, rel string) string {
	return p.buildPath(base, rel)
}

func (opts *ClientOptions) GetRestHost() string {
	return opts.getRestHost()
}

func (opts *ClientOptions) GetRealtimeHost() string {
	return opts.getRealtimeHost()
}

func (opts *ClientOptions) ActivePort() (int, bool) {
	return opts.activePort()
}

func (opts *ClientOptions) GetFallbackHosts() ([]string, error) {
	return opts.getFallbackHosts()
}

func (opts *ClientOptions) RestURL() string {
	return opts.restURL()
}

func (opts *ClientOptions) RealtimeURL() string {
	return opts.realtimeURL()
}

func (c *RestClient) Post(path string, in, out interface{}) (*http.Response, error) {
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

// MustRealtimeClient is like NewRealtimeClient, but panics on error.
func MustRealtimeClient(opts *ClientOptions) *RealtimeClient {
	client, err := NewRealtimeClient(opts)
	if err != nil {
		panic("ably.NewRealtimeClient failed: " + err.Error())
	}
	return client
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

func (c *RestClient) SetSuccessFallbackHost(duration time.Duration) {
	c.successFallbackHost = &fallbackCache{duration: duration}
}

func (c *RestClient) GetCachedFallbackHost() string {
	return c.successFallbackHost.get()
}

func (opts *ClientOptions) GetFallbackRetryTimeout() time.Duration {
	return opts.fallbackRetryTimeout()
}
