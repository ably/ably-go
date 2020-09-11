package ablytest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"
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
		req.Body = ioutil.NopCloser(io.TeeReader(req.Body, &buf))
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

type MessagePipeOption func(*pipeConn)

// MessagePipeWithNowFunc sets a function to get the current time. This time
// will be used to determine whether a Receive times out.
//
// If not set, receives won't timeout.
func MessagePipeWithNowFunc(now func() time.Time) MessagePipeOption {
	return func(pc *pipeConn) {
		pc.now = now
	}
}

func MessagePipe(in <-chan *proto.ProtocolMessage, out chan<- *proto.ProtocolMessage, opts ...MessagePipeOption) func(string, *url.URL, time.Duration) (proto.Conn, error) {
	return func(proto string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
		pc := pipeConn{
			in:  in,
			out: out,
		}
		for _, opt := range opts {
			opt(&pc)
		}
		return pc, nil
	}
}

type pipeConn struct {
	in  <-chan *proto.ProtocolMessage
	out chan<- *proto.ProtocolMessage
	now func() time.Time
}

func (pc pipeConn) Send(msg *proto.ProtocolMessage) error {
	pc.out <- msg
	return nil
}

func (pc pipeConn) Receive(deadline time.Time) (*proto.ProtocolMessage, error) {
	var timeout <-chan time.Time
	if pc.now != nil {
		timeout = time.After(deadline.Sub(pc.now()))
	}
	select {
	case m, ok := <-pc.in:
		if !ok || m == nil {
			return nil, io.EOF
		}
		return m, nil
	case <-timeout:
		return nil, errTimeout{}
	}
}

type errTimeout struct{}

func (errTimeout) Error() string   { return "timeout" }
func (errTimeout) Temporary() bool { return true }
func (errTimeout) Timeout() bool   { return true }

var _ net.Error = errTimeout{}

func (pc pipeConn) Close() error {
	return nil
}

// MessageRecorder
type MessageRecorder struct {
	mu       sync.Mutex
	url      []*url.URL
	sent     []*proto.ProtocolMessage
	received []*proto.ProtocolMessage
}

// NewMessageRecorder gives new spy value that records incoming and outgoing
// ProtocolMessages and dialed endpoints.
//
// For use with Dial field of clientOptions.
func NewMessageRecorder() *MessageRecorder {
	return &MessageRecorder{}
}

// Dial
func (rec *MessageRecorder) Dial(proto string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
	rec.mu.Lock()
	rec.url = append(rec.url, u)
	rec.mu.Unlock()
	conn, err := ablyutil.DialWebsocket(proto, u, timeout)
	if err != nil {
		return nil, err
	}
	return recConn{
		conn: conn,
		rec:  rec,
	}, nil
}

// URL
func (rec *MessageRecorder) URL() []*url.URL {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	url := make([]*url.URL, len(rec.url))
	copy(url, rec.url)
	return url
}

// Sent
func (rec *MessageRecorder) Sent() []*proto.ProtocolMessage {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	sent := make([]*proto.ProtocolMessage, len(rec.sent))
	copy(sent, rec.sent)
	return sent
}

// Received
func (rec *MessageRecorder) Received() []*proto.ProtocolMessage {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	received := make([]*proto.ProtocolMessage, len(rec.received))
	copy(received, rec.received)
	return received
}

type recConn struct {
	conn proto.Conn
	rec  *MessageRecorder
}

func (c recConn) Send(msg *proto.ProtocolMessage) error {
	if err := c.conn.Send(msg); err != nil {
		return err
	}
	c.rec.mu.Lock()
	c.rec.sent = append(c.rec.sent, msg)
	c.rec.mu.Unlock()
	return nil
}

func (c recConn) Receive(deadline time.Time) (*proto.ProtocolMessage, error) {
	msg, err := c.conn.Receive(deadline)
	if err != nil {
		return nil, err
	}
	c.rec.mu.Lock()
	c.rec.received = append(c.rec.received, msg)
	c.rec.mu.Unlock()
	return msg, nil
}

func (c recConn) Close() error {
	return c.conn.Close()
}

type HostRecorder struct {
	Hosts map[string]struct{}

	mu         sync.Mutex
	httpClient *http.Client
	dialWS     func(string, *url.URL, time.Duration) (proto.Conn, error)
}

func NewRecorder(httpClient *http.Client) *HostRecorder {
	hr := &HostRecorder{
		Hosts:      make(map[string]struct{}),
		httpClient: httpClient,
	}
	transport := httpClient.Transport.(*http.Transport)
	dial := transport.Dial
	transport.Dial = func(network, addr string) (net.Conn, error) {
		hr.addHost(addr)
		return dial(network, addr)
	}
	hr.dialWS = func(proto string, u *url.URL, timeout time.Duration) (proto.Conn, error) {
		hr.addHost(u.Host)
		return ablyutil.DialWebsocket(proto, u, timeout)
	}
	return hr
}

func (hr *HostRecorder) Options(opts ably.ClientOptions, host string) ably.ClientOptions {
	return opts.
		RealtimeHost(host).
		AutoConnect(false).
		Dial(hr.dialWS).
		HTTPClient(hr.httpClient)
}

func (hr *HostRecorder) addHost(host string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	if h, _, err := net.SplitHostPort(host); err == nil {
		hr.Hosts[h] = struct{}{}
	} else {
		hr.Hosts[host] = struct{}{}
	}
}

func body(p []byte) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(p))
}

// DialFakeDisconnect wraps a Dial function such that calling the returned
// disconnect function forcibly closes the connection to the server and fakes
// a DISCONNECT message from the server to the client.
//
// Only a single connection gets the disconnect signal, so don't reuse the
// clientOptions.
func DialFakeDisconnect(dial DialFunc) (_ DialFunc, disconnect func() error) {
	if dial == nil {
		dial = func(proto string, url *url.URL, timeout time.Duration) (proto.Conn, error) {
			return ablyutil.DialWebsocket(proto, url, timeout)
		}
	}

	disconnectReq := make(chan chan<- error, 1)

	return func(proto string, url *url.URL, timeout time.Duration) (proto.Conn, error) {
			conn, err := dial(proto, url, timeout)
			if err != nil {
				return nil, err
			}

			return connWithFakeDisconnect{
				conn:          conn,
				disconnectReq: disconnectReq,
				closed:        make(chan struct{}),
			}, nil
		}, func() error {
			err := make(chan error)
			disconnectReq <- err
			return <-err
		}
}

type DialFunc func(proto string, url *url.URL, timeout time.Duration) (proto.Conn, error)

type connWithFakeDisconnect struct {
	conn          proto.Conn
	disconnectReq <-chan chan<- error
	closed        chan struct{}
}

func (c connWithFakeDisconnect) Send(m *proto.ProtocolMessage) error {
	return c.conn.Send(m)
}

func (c connWithFakeDisconnect) Receive(deadline time.Time) (*proto.ProtocolMessage, error) {
	// Call the real Receive while waiting for a fake disconnection request.
	// The first wins. After a disconnection request, the connection is closed,
	// the ongoing real Receive is ignored and subsequent calls to Receive
	// fail.

	select {
	case <-c.closed:
		return nil, errors.New("called Receive on closed connection")
	default:
	}

	type receiveResult struct {
		m   *proto.ProtocolMessage
		err error
	}
	realReceive := make(chan receiveResult, 1)
	go func() {
		m, err := c.conn.Receive(deadline)
		select {
		case <-c.closed:
		case realReceive <- receiveResult{m: m, err: err}:
		}
	}()

	select {
	case r := <-realReceive:
		return r.m, r.err

	case errCh := <-c.disconnectReq:
		err := c.Close()
		errCh <- err
		if err != nil {
			return nil, err
		}

		return &proto.ProtocolMessage{
			Action: proto.ActionDisconnected,
			Error:  &proto.ErrorInfo{Message: "fake disconnection"},
		}, nil
	}
}

func (c connWithFakeDisconnect) Close() error {
	select {
	case <-c.closed:
		// Already closed.
		return nil
	default:
	}

	close(c.closed)
	return c.conn.Close()
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

		return c.c.Connection.ErrorReason()
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
