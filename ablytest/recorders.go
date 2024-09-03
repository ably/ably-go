package ablytest

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/ably/ably-go/ably"
)

// RoundTripRecorder is a http.Transport wrapper which records
// HTTP request/response pairs.
type RoundTripRecorder struct {
	*http.Transport

	mtx     sync.Mutex
	reqs    []*http.Request
	resps   []*http.Response
	stopped int32
}

func NewHttpRecorder() (*RoundTripRecorder, []ably.ClientOption) {
	rec := &RoundTripRecorder{}
	httpClient := &http.Client{Transport: &http.Transport{}}
	httpClient.Transport = rec.Hijack(httpClient.Transport)
	return rec, []ably.ClientOption{ably.WithHTTPClient(httpClient)}
}

var _ http.RoundTripper = (*RoundTripRecorder)(nil)

// Len gives number of recorded request/response pairs.
//
// It is save to call Len() before calling Stop().
func (rec *RoundTripRecorder) Len() int {
	rec.mtx.Lock()
	defer rec.mtx.Unlock()
	return len(rec.reqs)
}

// Request gives nth recorded http.Request.
func (rec *RoundTripRecorder) Request(n int) *http.Request {
	rec.mtx.Lock()
	defer rec.mtx.Unlock()
	return rec.reqs[n]
}

// Response gives nth recorded http.Response.
func (rec *RoundTripRecorder) Response(n int) *http.Response {
	rec.mtx.Lock()
	defer rec.mtx.Unlock()
	return rec.resps[n]
}

// Requests gives all HTTP requests in order they were recorded.
func (rec *RoundTripRecorder) Requests() []*http.Request {
	rec.mtx.Lock()
	defer rec.mtx.Unlock()
	reqs := make([]*http.Request, len(rec.reqs))
	copy(reqs, rec.reqs)
	return reqs
}

// Responses gives all HTTP responses in order they were recorded.
func (rec *RoundTripRecorder) Responses() []*http.Response {
	rec.mtx.Lock()
	defer rec.mtx.Unlock()
	resps := make([]*http.Response, len(rec.resps))
	copy(resps, rec.resps)
	return resps
}

// RoundTrip implements the http.RoundTripper interface.
func (rec *RoundTripRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	if atomic.LoadInt32(&rec.stopped) == 0 {
		return rec.roundTrip(req)
	}
	return rec.Transport.RoundTrip(req)
}

// Stop makes the recorder stop recording new requests/responses.
func (rec *RoundTripRecorder) Stop() {
	atomic.StoreInt32(&rec.stopped, 1)
}

// Hijack injects http.Transport into the wrapper.
func (rec *RoundTripRecorder) Hijack(rt http.RoundTripper) http.RoundTripper {
	if tr, ok := rt.(*http.Transport); ok {
		rec.Transport = tr
	}
	return rec
}

// Reset resets the recorder requests and responses.
func (rec *RoundTripRecorder) Reset() {
	rec.mtx.Lock()
	rec.reqs = nil
	rec.resps = nil
	rec.mtx.Unlock()
}

func (rec *RoundTripRecorder) roundTrip(req *http.Request) (*http.Response, error) {
	var buf bytes.Buffer
	if req.Body != nil {
		req.Body = io.NopCloser(io.TeeReader(req.Body, &buf))
	}
	resp, err := rec.Transport.RoundTrip(req)
	req.Body = body(buf.Bytes())
	buf.Reset()
	if resp != nil && resp.Body != nil {
		_, e := io.Copy(&buf, resp.Body)
		err = nonil(err, e, resp.Body.Close())
		resp.Body = body(buf.Bytes())
	}
	rec.mtx.Lock()
	respCopy := *resp
	respCopy.Body = body(buf.Bytes())
	rec.reqs = append(rec.reqs, req)
	rec.resps = append(rec.resps, &respCopy)
	rec.mtx.Unlock()
	return resp, err
}

type ConnStatesRecorder struct {
	mtx    sync.Mutex
	states []ably.ConnectionState
}

func (cs *ConnStatesRecorder) append(state ably.ConnectionState) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.states = append(cs.states, state)
}

func (cs *ConnStatesRecorder) Listen(r *ably.Realtime) (off func()) {
	cs.append(r.Connection.State())
	return r.Connection.OnAll(func(c ably.ConnectionStateChange) {
		cs.append(c.Current)
	})
}

func (cs *ConnStatesRecorder) States() []ably.ConnectionState {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.states
}

type ChanStatesRecorder struct {
	mtx    sync.Mutex
	states []ably.ChannelState
}

func (cs *ChanStatesRecorder) append(state ably.ChannelState) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.states = append(cs.states, state)
}

func (cs *ChanStatesRecorder) Listen(channel *ably.RealtimeChannel) (off func()) {
	cs.append(channel.State())
	return channel.OnAll(func(c ably.ChannelStateChange) {
		cs.append(c.Current)
	})
}

func (cs *ChanStatesRecorder) States() []ably.ChannelState {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.states
}

type ConnErrorsRecorder struct {
	mtx    sync.Mutex
	errors []*ably.ErrorInfo
}

func (ce *ConnErrorsRecorder) appendNonNil(err *ably.ErrorInfo) {
	ce.mtx.Lock()
	defer ce.mtx.Unlock()
	if err != nil {
		ce.errors = append(ce.errors, err)
	}
}

func (ce *ConnErrorsRecorder) Listen(r *ably.Realtime) (off func()) {
	ce.appendNonNil(r.Connection.ErrorReason())
	return r.Connection.OnAll(func(c ably.ConnectionStateChange) {
		ce.appendNonNil(c.Reason)
	})
}

func (ce *ConnErrorsRecorder) Errors() []*ably.ErrorInfo {
	ce.mtx.Lock()
	defer ce.mtx.Unlock()
	return ce.errors
}

// FullRealtimeCloser returns an io.Closer that, on Close, calls Close on the
// Realtime instance and waits for its effects.
func FullRealtimeCloser(c *ably.Realtime) io.Closer {
	return realtimeIOCloser{c: c}
}

type realtimeIOCloser struct {
	c *ably.Realtime
}

func (c realtimeIOCloser) Close() error {
	switch c.c.Connection.State() {
	case
		ably.ConnectionStateInitialized,
		ably.ConnectionStateClosed,
		ably.ConnectionStateFailed:

		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	errCh := make(chan error, 1)

	off := make(chan func(), 1)
	off <- c.c.Connection.OnAll(func(c ably.ConnectionStateChange) {
		switch c.Current {
		default:
			return
		case
			ably.ConnectionStateClosed,
			ably.ConnectionStateFailed:
		}

		(<-off)()

		var err error
		if c.Reason != nil {
			err = *c.Reason
		}
		errCh <- err
	})

	c.c.Close()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func body(p []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(p))
}
