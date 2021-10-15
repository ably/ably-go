package ably_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ablytest"
)

type Result interface {
	Wait(context.Context) error
}

func nonil(err ...error) error {
	for _, err := range err {
		if err != nil {
			return err
		}
	}
	return nil
}

type closeClient struct {
	io.Closer
	skip []int
}

func (c *closeClient) Close() error {
	e := c.Closer.Close()
	if a, ok := e.(*ably.ErrorInfo); ok {
		for _, v := range c.skip {
			if a.StatusCode == v {
				return nil
			}
		}
	}
	return e
}

func safeclose(t *testing.T, closers ...io.Closer) {
	type failed struct {
		i   int
		c   io.Closer
		err error
	}
	var closeErrors []failed
	for i, closer := range closers {
		err := closer.Close()
		if err != nil {
			closeErrors = append(closeErrors, failed{i, closer, err})
		}
	}
	if len(closeErrors) != 0 {
		for _, err := range closeErrors {
			t.Logf("safeclose %d: failed to close %T: %s", err.i, err.c, err.err)
		}
	}
}

type closeFunc func() error

func (f closeFunc) Close() error {
	return f()
}

func checkError(code ably.ErrorCode, err error) error {
	switch e, ok := err.(*ably.ErrorInfo); {
	case !ok:
		return fmt.Errorf("want err to be *ably.ErrorInfo; was %T: %v", err, err)
	case e.Code != code:
		return fmt.Errorf("want e.Code=%d; got %d: %s", code, e.Code, err)
	default:
		return nil
	}
}

func assertEquals(t *testing.T, expected interface{}, actual interface{}) {
	t.Helper()

	if expected != actual {
		t.Errorf("%v is not equal to %v", expected, actual)
	}
}

func assertTrue(t *testing.T, value bool) {
	t.Helper()

	if !value {
		t.Errorf("%v is not true", value)
	}
}

func assertFalse(t *testing.T, value bool) {
	t.Helper()

	if value {
		t.Errorf("%v is not false", value)
	}
}

func assertNil(t *testing.T, object interface{}) {
	t.Helper()

	if object != nil {
		value := reflect.ValueOf(object)
		if !value.IsNil() {
			t.Errorf("%v is not nil", object)
		}
	}
}

func assertDeepEquals(t *testing.T, expected interface{}, actual interface{}) {
	t.Helper()

	areEqual := reflect.DeepEqual(expected, actual)
	if !areEqual {
		t.Errorf("%v is not equal to %v", expected, actual)
	}
}

func init() {
	ablytest.ClientOptionsInspector.UseBinaryProtocol = func(o []ably.ClientOption) bool {
		return !ably.ApplyOptionsWithDefaults(o...).NoBinaryProtocol
	}
	ablytest.ClientOptionsInspector.HTTPClient = func(o []ably.ClientOption) *http.Client {
		return ably.ApplyOptionsWithDefaults(o...).HTTPClient
	}
}

type messages chan *ably.Message

func (ms messages) Receive(m *ably.Message) {
	ms <- m
}

type connMock struct {
	SendFunc    func(*ably.ProtocolMessage) error
	ReceiveFunc func(deadline time.Time) (*ably.ProtocolMessage, error)
	CloseFunc   func() error
}

func (r connMock) Send(a0 *ably.ProtocolMessage) error {
	return r.SendFunc(a0)
}

func (r connMock) Receive(deadline time.Time) (*ably.ProtocolMessage, error) {
	return r.ReceiveFunc(deadline)
}

func (r connMock) Close() error {
	return r.CloseFunc()
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

// MessagePipeWithAfterFunc sets a function to get a timer. This timer
// will be used to determine whether a Receive times out.
//
// If not set, receives won't timeout.
func MessagePipeWithAfterFunc(after func(context.Context, time.Duration) <-chan time.Time) MessagePipeOption {
	return func(pc *pipeConn) {
		pc.after = after
	}
}

func MessagePipe(in <-chan *ably.ProtocolMessage, out chan<- *ably.ProtocolMessage, opts ...MessagePipeOption) func(string, *url.URL, time.Duration) (ably.Conn, error) {
	return func(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
		pc := pipeConn{
			in:    in,
			out:   out,
			after: ablyutil.After,
		}
		for _, opt := range opts {
			opt(&pc)
		}
		return pc, nil
	}
}

type pipeConn struct {
	in    <-chan *ably.ProtocolMessage
	out   chan<- *ably.ProtocolMessage
	now   func() time.Time
	after func(context.Context, time.Duration) <-chan time.Time
}

func (pc pipeConn) Send(msg *ably.ProtocolMessage) error {
	pc.out <- msg
	return nil
}

func (pc pipeConn) Receive(deadline time.Time) (*ably.ProtocolMessage, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var timeout <-chan time.Time
	if pc.now != nil {
		timeout = pc.after(ctx, deadline.Sub(pc.now()))
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
	sent     []*ably.ProtocolMessage
	received []*ably.ProtocolMessage
}

// NewMessageRecorder gives new spy value that records incoming and outgoing
// ProtocolMessages and dialed endpoints.
//
// For use with Dial field of clientOptions.
func NewMessageRecorder() *MessageRecorder {
	return &MessageRecorder{}
}

// Reset resets the recorded urls, sent and received messages
func (rec *MessageRecorder) Reset() {
	rec.mu.Lock()
	rec.url = nil
	rec.sent = nil
	rec.received = nil
	rec.mu.Unlock()
}

// Dial
func (rec *MessageRecorder) Dial(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
	rec.mu.Lock()
	rec.url = append(rec.url, u)
	rec.mu.Unlock()
	conn, err := ably.DialWebsocket(proto, u, timeout)
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
	newUrl := make([]*url.URL, len(rec.url))
	copy(newUrl, rec.url)
	return newUrl
}

// Sent
func (rec *MessageRecorder) Sent() []*ably.ProtocolMessage {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	sent := make([]*ably.ProtocolMessage, len(rec.sent))
	copy(sent, rec.sent)
	return sent
}

func (rec *MessageRecorder) CheckIfSent(action ably.ProtoAction, times int) func() bool {
	return func() bool {
		counter := 0
		for _, m := range rec.Sent() {
			if m.Action == action {
				counter++
				if counter == times {
					return true
				}
			}
		}
		return false
	}
}

func (rec *MessageRecorder) FindFirst(action ably.ProtoAction) *ably.ProtocolMessage {
	for _, m := range rec.Sent() {
		if m.Action == action {
			return m
		}
	}
	return nil
}

// Received
func (rec *MessageRecorder) Received() []*ably.ProtocolMessage {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	received := make([]*ably.ProtocolMessage, len(rec.received))
	copy(received, rec.received)
	return received
}

type recConn struct {
	conn ably.Conn
	rec  *MessageRecorder
}

func (c recConn) Send(msg *ably.ProtocolMessage) error {
	if err := c.conn.Send(msg); err != nil {
		return err
	}
	c.rec.mu.Lock()
	c.rec.sent = append(c.rec.sent, msg)
	c.rec.mu.Unlock()
	return nil
}

func (c recConn) Receive(deadline time.Time) (*ably.ProtocolMessage, error) {
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
	dialWS     func(string, *url.URL, time.Duration) (ably.Conn, error)
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
	hr.dialWS = func(proto string, u *url.URL, timeout time.Duration) (ably.Conn, error) {
		hr.addHost(u.Host)
		return ably.DialWebsocket(proto, u, timeout)
	}
	return hr
}

func (hr *HostRecorder) Options(host string, opts ...ably.ClientOption) []ably.ClientOption {
	return append(opts,
		ably.WithRealtimeHost(host),
		ably.WithAutoConnect(false),
		ably.WithDial(hr.dialWS),
		ably.WithHTTPClient(hr.httpClient),
	)
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

// DialFakeDisconnect wraps a Dial function such that calling the returned
// disconnect function forcibly closes the connection to the server and fakes
// a DISCONNECT message from the server to the client.
//
// Only a single connection gets the disconnect signal, so don't reuse the
// clientOptions.
func DialFakeDisconnect(dial DialFunc) (_ DialFunc, disconnect func() error) {
	if dial == nil {
		dial = func(proto string, url *url.URL, timeout time.Duration) (ably.Conn, error) {
			return ably.DialWebsocket(proto, url, timeout)
		}
	}

	disconnectReq := make(chan chan<- error, 1)

	return func(proto string, url *url.URL, timeout time.Duration) (ably.Conn, error) {
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

type DialFunc func(proto string, url *url.URL, timeout time.Duration) (ably.Conn, error)

type connWithFakeDisconnect struct {
	conn          ably.Conn
	disconnectReq <-chan chan<- error
	closed        chan struct{}
}

func (c connWithFakeDisconnect) Send(m *ably.ProtocolMessage) error {
	return c.conn.Send(m)
}

func (c connWithFakeDisconnect) Receive(deadline time.Time) (*ably.ProtocolMessage, error) {
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
		m   *ably.ProtocolMessage
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

		return &ably.ProtocolMessage{
			Action: ably.ActionDisconnected,
			Error:  &ably.ProtoErrorInfo{Message: "fake disconnection"},
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

// DialIntercept returns a DialFunc and an intercept function that, when called,
// makes the processing of the next received protocol message with any of the given
// actions. The processing remains blocked until the passed context expires. The
// intercepted message is sent to the returned channel.
func DialIntercept(dial DialFunc) (_ DialFunc, intercept func(context.Context, ...ably.ProtoAction) <-chan *ably.ProtocolMessage) {
	active := &activeIntercept{}

	intercept = func(ctx context.Context, actions ...ably.ProtoAction) <-chan *ably.ProtocolMessage {
		msg := make(chan *ably.ProtocolMessage, 1)
		active.Lock()
		defer active.Unlock()
		active.ctx = ctx
		active.actions = actions
		active.msg = msg
		return msg
	}

	return func(proto string, url *url.URL, timeout time.Duration) (ably.Conn, error) {
		conn, err := dial(proto, url, timeout)
		if err != nil {
			return nil, err
		}
		return interceptConn{conn, active}, nil
	}, intercept
}

type activeIntercept struct {
	sync.Mutex
	ctx     context.Context
	actions []ably.ProtoAction
	msg     chan<- *ably.ProtocolMessage
}

type interceptConn struct {
	ably.Conn
	active *activeIntercept
}

func (c interceptConn) Receive(deadline time.Time) (*ably.ProtocolMessage, error) {
	msg, err := c.Conn.Receive(deadline)
	if err != nil {
		return nil, err
	}

	c.active.Lock()
	defer c.active.Unlock()

	if c.active.msg == nil {
		return msg, err
	}

	for _, a := range c.active.actions {
		if msg.Action == a {
			c.active.msg <- msg
			c.active.msg = nil
			<-c.active.ctx.Done()
			break
		}
	}

	return msg, err
}

var canceledCtx context.Context = func() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}()
