package ably

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func (p *PaginatedResult) BuildPath(base, rel string) (string, error) {
	return p.buildPath(base, rel)
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

func DecodeResp(resp *http.Response, out interface{}) error {
	return decodeResp(resp, out)
}

func ErrorCode(err error) int {
	return code(err)
}

func NewTokenParams(query url.Values) *TokenParams {
	params := &TokenParams{}
	if n, err := strconv.ParseInt(query.Get("ttl"), 10, 64); err == nil {
		params.TTL = n
	}
	if s := query.Get("capability"); s != "" {
		params.RawCapability = s
	}
	if s := query.Get("clientId"); s != "" {
		params.ClientID = s
	}
	if n, err := strconv.ParseInt(query.Get("timestamp"), 10, 64); err == nil {
		params.Timestamp = n
	}
	return params
}

// AuthReverseProxy serves token requests by reverse proxying them to
// the Ably servers. Use URL method for creating values for AuthURL
// option and Callback method - for AuthCallback ones.
type AuthReverseProxy struct {
	TokenQueue []*TokenDetails // when non-nil pops the token from the queue instead querying Ably servers
	Listener   net.Listener    // listener which accepts token request connections

	auth *Auth
}

// NewAuthReverseProxy creates new auth reverse proxy. The given opts
// are used to create a Auth client, used to reverse proxying token requests.
func NewAuthReverseProxy(opts *ClientOptions) (*AuthReverseProxy, error) {
	opts.UseTokenAuth = true
	client, err := NewRestClient(opts)
	if err != nil {
		return nil, err
	}
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}
	srv := &AuthReverseProxy{
		Listener: lis,
		auth:     client.Auth,
	}
	go http.Serve(lis, srv)
	return srv, nil
}

// MustAuthReverseProxy panics when creating the proxy fails.
func MustAuthReverseProxy(opts *ClientOptions) *AuthReverseProxy {
	srv, err := NewAuthReverseProxy(opts)
	if err != nil {
		panic(err)
	}
	return srv
}

// URL gives new AuthURL for the requested responseType. Available response
// types are:
//
//   - "token", which responds with (ably.TokenDetails).Token as a string
//   - "details", which responds with ably.TokenDetails
//   - "request", which responds with ably.TokenRequest
//
func (srv *AuthReverseProxy) URL(responseType string) string {
	return "http://" + srv.Listener.Addr().String() + "/" + responseType
}

// Callback gives new AuthCallback. Available response types are the same
// as for URL method.
func (srv *AuthReverseProxy) Callback(responseType string) func(*TokenParams) (interface{}, error) {
	return func(params *TokenParams) (interface{}, error) {
		token, _, err := srv.handleAuth(responseType, params)
		return token, err
	}
}

// Close makes the proxy server stop accepting connections.
func (srv *AuthReverseProxy) Close() error {
	return srv.Listener.Close()
}

// ServeHTTP implements the http.Handler interface.
func (srv *AuthReverseProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	token, contentType, err := srv.handleAuth(req.URL.Path[1:], NewTokenParams(req.URL.Query()))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	p, err := encode(contentType, token)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(p)))
	w.WriteHeader(200)
	if _, err = io.Copy(w, bytes.NewReader(p)); err != nil {
		panic(err)
	}
}

func (srv *AuthReverseProxy) handleAuth(responseType string, params *TokenParams) (token interface{}, typ string, err error) {
	switch responseType {
	case "token", "details":
		var tok *TokenDetails
		if len(srv.TokenQueue) != 0 {
			tok, srv.TokenQueue = srv.TokenQueue[0], srv.TokenQueue[1:]
		} else {
			tok, err = srv.auth.Authorise(nil, params, true)
			if err != nil {
				return nil, "", err
			}
		}
		if responseType == "token" {
			return tok.Token, "text/plain", nil
		}
		return tok, srv.auth.opts().protocol(), nil
	case "request":
		tokReq, err := srv.auth.CreateTokenRequest(nil, params)
		if err != nil {
			return nil, "", err
		}
		return tokReq, srv.auth.opts().protocol(), nil
	default:
		return nil, "", errors.New("unexpected token value type: " + typ)
	}
}

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

func body(p []byte) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(p))
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
	ch     chan State
	states []StateEnum
	wg     sync.WaitGroup
	done   chan struct{}
	mtx    sync.Mutex
}

// NewStateRecorder gives new recorder which purpose is to record states via
// (*ClientOptions).Listener channel.
//
// If buffer is > 0, the recorder will use it as a buffer to ensure all states
// transitions are received.
// If buffer is <= 0, the recorder will not buffer any states, which can
// result in some of them being dropped.
func NewStateRecorder(buffer int) *StateRecorder {
	if buffer < 0 {
		buffer = 0
	}
	rec := &StateRecorder{
		ch:     make(chan State, buffer),
		done:   make(chan struct{}),
		states: make([]StateEnum, 0, buffer),
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
			rec.add(state.State)
		case <-rec.done:
			return
		}
	}
}

// Add appends state to the list of recorded ones, used to ensure ordering
// of the states by injecting values at certain points of the test.
func (rec *StateRecorder) Add(state StateEnum) {
	rec.ch <- State{State: state}
}

func (rec *StateRecorder) add(state StateEnum) {
	rec.mtx.Lock()
	rec.states = append(rec.states, state)
	rec.mtx.Unlock()
}

func (rec *StateRecorder) Channel() chan<- State {
	return rec.ch
}

// Stop stops separate recording gorouting and waits until it terminates.
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
func (rec *StateRecorder) States() []StateEnum {
	rec.mtx.Lock()
	defer rec.mtx.Unlock()
	states := make([]StateEnum, len(rec.states))
	copy(states, rec.states)
	return states
}

// WaitFor blocks until we observe the given exact states were recorded.
func (rec *StateRecorder) WaitFor(states []StateEnum, timeout time.Duration) error {
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
	case <-time.After(timeout):
		close(stop)
		return fmt.Errorf("WaitFor(%v) has timed out after %v: recorded states were %v",
			states, timeout, rec.States())
	}
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

// ResultGroup is like sync.WaitGroup, but for ably.Result values.
// ResultGroup blocks till last added ably.Result has completed successfully.
// If at least ably.Result value failed, ResultGroup returns first encountered
// error immadiately.
type ResultGroup struct {
	wg    sync.WaitGroup
	err   error
	errch chan error
}

func (rg *ResultGroup) init() {
	if rg.errch == nil {
		rg.errch = make(chan error, 1)
	}
}

func (rg *ResultGroup) Add(res Result, err error) {
	rg.init()
	if rg.err != nil {
		return
	}
	if err != nil {
		rg.err = err
		return
	}
	rg.wg.Add(1)
	go func() {
		err := res.Wait()
		if err != nil {
			select {
			case rg.errch <- err:
			default:
			}
		}
		rg.wg.Done()
	}()
}

func (rg *ResultGroup) Wait() error {
	if rg.err != nil {
		return rg.err
	}
	done := make(chan struct{})
	go func() {
		rg.wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
		return nil
	case err := <-rg.errch:
		return err
	}
}

type HostRecorder struct {
	Hosts map[string]struct{}

	mu         sync.Mutex
	httpClient *http.Client
	dialWS     func(string, *url.URL) (MsgConn, error)
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
	hr.dialWS = func(proto string, u *url.URL) (MsgConn, error) {
		hr.addHost(u.Host)
		return dialWebsocket(proto, u)
	}
	return hr
}

func (hr *HostRecorder) Options(host string) *ClientOptions {
	return &ClientOptions{
		RealtimeHost: host,
		NoConnect:    true,
		HTTPClient:   hr.httpClient,
		Dial:         hr.dialWS,
	}
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
