package ablytest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
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

// StateRecorder provides:
//
//   * send ably.State channel for recording state transitions
//   * goroutine-safe access to recorded state enums
//
type StateRecorder struct {
	Timeout time.Duration // times out waiting for states after this duration; 15s by default

	mtx    sync.Mutex
	wg     sync.WaitGroup
	states []ably.State
	ch     chan ably.State
	done   chan struct{}
	typ    ably.StateType
}

// NewStateRecorder gives new recorder which purpose is to record states via
// (*clientOptions).Listener channel.
//
// If buffer is > 0, the recorder will use it as a buffer to ensure all states
// transitions are received.
// If buffer is <= 0, the recorder will not buffer any states, which can
// result in some of them being dropped.
func NewStateRecorder(buffer int) *StateRecorder {
	return newStateRecorder(buffer, ably.StateChan|ably.StateConn)
}

// NewStateChanRecorder gives new recorder which records channel-related
// state transitions only.
func NewStateChanRecorder(buffer int) *StateRecorder {
	return newStateRecorder(buffer, ably.StateChan)
}

// NewStateConnRecorder gives new recorder which records connection-related
// state transitions only.
func NewStateConnRecorder(buffer int) *StateRecorder {
	return newStateRecorder(buffer, ably.StateConn)
}

func newStateRecorder(buffer int, typ ably.StateType) *StateRecorder {
	if buffer < 0 {
		buffer = 0
	}
	rec := &StateRecorder{
		ch:     make(chan ably.State, buffer),
		done:   make(chan struct{}),
		states: make([]ably.State, 0, buffer),
		typ:    typ,
	}
	rec.wg.Add(1)
	go rec.processIncomingStates()
	return rec
}

func (rec *StateRecorder) processIncomingStates() {
	defer rec.wg.Done()
	for {
		select {
		case state, ok := <-rec.ch:
			if !ok {
				return
			}
			if state.Type != 0 && state.Type&rec.typ == 0 {
				continue
			}
			rec.add(state)
		case <-rec.done:
			return
		}
	}
}

// Add appends state to the list of recorded ones, used to ensure ordering
// of the states by injecting values at certain points of the test.
func (rec *StateRecorder) Add(state ably.StateEnum) {
	rec.ch <- ably.State{State: state}
}

func (rec *StateRecorder) add(state ably.State) {
	rec.mtx.Lock()
	rec.states = append(rec.states, state)
	rec.mtx.Unlock()
}

func (rec *StateRecorder) Channel() chan<- ably.State {
	return rec.ch
}

// Stop stops separate recording goroutine and waits until it terminates.
func (rec *StateRecorder) Stop() {
	close(rec.done)
	rec.wg.Wait()
	// Drain listener channel.
	for {
		select {
		case <-rec.ch:
		default:
			return
		}
	}
}

// States gives copy of the recorded states, safe for use while the recorder
// is still running.
func (rec *StateRecorder) States() []ably.StateEnum {
	rec.mtx.Lock()
	defer rec.mtx.Unlock()
	states := make([]ably.StateEnum, 0, len(rec.states))
	for _, state := range rec.states {
		states = append(states, state.State)
	}
	return states
}

// Errors gives copy of the error that recorded events hold. It returns only
// non-nil errors. If none of the recorded states contained an error, the
// method returns nil.
func (rec *StateRecorder) Errors() []*ably.ErrorInfo {
	var errors []*ably.ErrorInfo
	rec.mtx.Lock()
	defer rec.mtx.Unlock()
	for _, state := range rec.states {
		if state.Err != nil {
			errors = append(errors, state.Err)
		}
	}
	return errors
}

// WaitFor blocks until we observe the given exact states were recorded.
func (rec *StateRecorder) WaitFor(states []ably.StateEnum) error {
	done := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				if recorded := rec.States(); reflect.DeepEqual(states, recorded) {
					close(done)
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
		}
	}()
	select {
	case <-done:
		return nil
	case <-time.After(rec.timeout()):
		close(stop)
		return fmt.Errorf("WaitFor(%v) has timed out after %v: recorded states were %v",
			states, rec.timeout(), rec.States())
	}
}

func (rec *StateRecorder) timeout() time.Duration {
	if rec.Timeout != 0 {
		return rec.Timeout
	}
	return 15 * time.Second
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
