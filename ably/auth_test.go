package ably_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

func single() *ably.PaginateParams {
	return &ably.PaginateParams{
		Limit:     1,
		Direction: "forwards",
	}
}

func recorder() (*ablytest.RoundTripRecorder, *ably.ClientOptions) {
	rec := &ablytest.RoundTripRecorder{}
	opts := &ably.ClientOptions{
		HTTPClient: &http.Client{
			Transport: rec,
		},
	}
	return rec, opts
}

func authValue(req *http.Request) (value string, err error) {
	auth := req.Header.Get("Authorization")
	if i := strings.IndexRune(auth, ' '); i != -1 {
		p, err := base64.StdEncoding.DecodeString(auth[i+1:])
		if err != nil {
			return "", errors.New("failed to base64 decode Authorization header value: " + err.Error())
		}
		auth = string(p)
	}
	return auth, nil
}

func TestAuth_BasicAuth(t *testing.T) {
	t.Parallel()
	rec, opts := recorder()
	opts.UseQueryTime = true
	defer rec.Stop()
	app, client := ablytest.NewRestClient(opts)
	defer safeclose(t, app)

	if _, err := client.Time(); err != nil {
		t.Fatalf("client.Time()=%v", err)
	}
	if _, err := client.Stats(single()); err != nil {
		t.Fatalf("client.Stats()=%v", err)
	}
	if n := rec.Len(); n != 2 {
		t.Fatalf("want rec.Len()=2; got %d", n)
	}
	if method := client.Auth.Method(); method != ably.AuthBasic {
		t.Fatalf("want method=1; got %d", method)
	}
	url := rec.Request(1).URL
	if url.Scheme != "https" {
		t.Fatalf("want url.Scheme=https; got %s", url.Scheme)
	}
	auth, err := authValue(rec.Request(1))
	if err != nil {
		t.Fatalf("authValue=%v", err)
	}
	if key := app.Options().Key; auth != key {
		t.Fatalf("want auth=%q; got %q", key, auth)
	}
	// Can't use basic auth over HTTP.
	opts.NoTLS = true
	switch _, err := ably.NewRestClient(app.Options(opts)); {
	case err == nil:
		t.Fatal("want err != nil")
	case ably.ErrorCode(err) != 40103:
		t.Fatalf("want code=40103; got %d", ably.ErrorCode(err))
	}
}

func timeWithin(t, start, end time.Time) error {
	if t.Before(start) || t.After(end) {
		return fmt.Errorf("want t=%v to be within [%v, %v] time span", t, start, end)
	}
	return nil
}

func TestAuth_TokenAuth(t *testing.T) {
	t.Parallel()
	rec, opts := recorder()
	defer rec.Stop()
	opts.NoTLS = true
	opts.UseTokenAuth = true
	opts.UseQueryTime = true
	app, client := ablytest.NewRestClient(opts)
	defer safeclose(t, app)

	beforeAuth := time.Now().Add(-time.Second)
	if _, err := client.Time(); err != nil {
		t.Fatalf("client.Time()=%v", err)
	}
	if _, err := client.Stats(single()); err != nil {
		t.Fatalf("client.Stats()=%v", err)
	}
	// At this points there should be two requests recorded:
	//
	//   - first: explicit call to Time()
	//   - second: implicit call to Time() during token request
	//   - third: token request
	//   - fourth: actual stats request
	//
	if n := rec.Len(); n != 4 {
		t.Fatalf("want rec.Len()=4; got %d", n)
	}
	if method := client.Auth.Method(); method != ably.AuthToken {
		t.Fatalf("want method=2; got %d", method)
	}
	url := rec.Request(3).URL
	if url.Scheme != "http" {
		t.Fatalf("want url.Scheme=http; got %s", url.Scheme)
	}
	auth, err := authValue(rec.Request(3))
	if err != nil {
		t.Fatalf("authValue=%v", err)
	}
	rec.Reset()
	tok, err := client.Auth.Authorize(nil, nil)
	if err != nil {
		t.Fatalf("Authorize()=%v", err)
	}
	// Call to Authorize with force set to false should not refresh the token
	// and return existing one.
	// The following ensures no token request was made.
	if n := rec.Len(); n != 0 {
		t.Fatalf("Authorize() did not return existing token; want rec.Len()=0; %d", n)
	}
	if auth != tok.Token {
		t.Fatalf("want auth=%q; got %q", tok.Token, auth)
	}
	if defaultCap := (ably.Capability{"*": {"*"}}); tok.RawCapability != defaultCap.Encode() {
		t.Fatalf("want tok.Capability=%v; got %v", defaultCap, tok.Capability())
	}
	now := time.Now().Add(time.Second)
	if err := timeWithin(tok.IssueTime(), beforeAuth, now); err != nil {
		t.Fatal(err)
	}
	// Ensure token expires in 60m (default TTL).
	beforeAuth = beforeAuth.Add(60 * time.Minute)
	now = now.Add(60 * time.Minute)
	if err := timeWithin(tok.ExpireTime(), beforeAuth, now); err != nil {
		t.Fatal(err)
	}
	// Can't use token auth with missing token parameters.
	opts = app.Options(opts)
	opts.Key = ""
	switch _, err := ably.NewRestClient(opts); {
	case err == nil:
		t.Fatal("want err != nil")
	case ably.ErrorCode(err) != 40102:
		t.Fatalf("want code=40102; got %d", ably.ErrorCode(err))
	}
}

type bufferLogger struct {
	buf bytes.Buffer
}

func (b *bufferLogger) Print(level ably.LogLevel, v ...interface{}) {
	v = append([]interface{}{level}, v...)
	fmt.Fprint(&b.buf, v...)
}
func (b *bufferLogger) Printf(level ably.LogLevel, str string, v ...interface{}) {}

func TestAuth_Authorise(t *testing.T) {
	t.Parallel()
	rec, opts := recorder()
	defer rec.Stop()
	log := &bufferLogger{}
	opts.UseTokenAuth = true
	opts.Logger.Logger = log
	opts.Logger.Level = ably.LogWarning
	app, client := ablytest.NewRestClient(opts)
	defer safeclose(t, app)

	params := &ably.TokenParams{
		TTL: ably.Duration(time.Second),
	}
	_, err := client.Auth.Authorise(params, &ably.AuthOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	msg := "Auth.Authorise is deprecated"
	txt := log.buf.String()

	// make sure deprecation warning is logged
	//
	// Refers to RSA10L
	if !strings.Contains(txt, msg) {
		t.Errorf("expected %s to contain %s", txt, msg)
	}
}

func TestAuth_TokenAuth_Renew(t *testing.T) {
	t.Parallel()
	rec, opts := recorder()
	defer rec.Stop()
	opts.UseTokenAuth = true
	app, client := ablytest.NewRestClient(opts)
	defer safeclose(t, app)

	params := &ably.TokenParams{
		TTL: ably.Duration(time.Second),
	}
	tok, err := client.Auth.Authorize(params, &ably.AuthOptions{Force: true})
	if err != nil {
		t.Fatalf("Authorize()=%v", err)
	}
	if n := rec.Len(); n != 1 {
		t.Fatalf("want rec.Len()=1; got %d", n)
	}
	if ttl := tok.ExpireTime().Sub(tok.IssueTime()); ttl > 2*time.Second {
		t.Fatalf("want ttl=1s; got %v", ttl)
	}
	time.Sleep(2 * time.Second) // wait till expires
	_, err = client.Stats(single())
	if err != nil {
		t.Fatalf("Stats()=%v", err)
	}
	// Recorded responses:
	//
	//   - 0: response for explicit Authorize()
	//   - 1: response for implicit Authorize() (token renewal)
	//   - 2: response for Stats()
	//
	if n := rec.Len(); n != 3 {
		t.Fatalf("token not renewed; want rec.Len()=3; got %d", n)
	}
	var newTok ably.TokenDetails
	if err := ably.DecodeResp(rec.Response(1), &newTok); err != nil {
		t.Fatalf("token decode error: %v", err)
	}
	if tok.Token == newTok.Token {
		t.Fatalf("token not renewed; new token equals old: %s", tok.Token)
	}
	// Ensure token was renewed with original params.
	if ttl := newTok.ExpireTime().Sub(newTok.IssueTime()); ttl > 2*time.Second {
		t.Fatalf("want ttl=1s; got %v", ttl)
	}
	time.Sleep(2 * time.Second) // wait for token to expire
	// Ensure request fails when Token or *TokenDetails is provided, but no
	// means to renew the token
	rec.Reset()
	opts = app.Options(opts)
	opts.Key = ""
	opts.TokenDetails = tok
	client, err = ably.NewRestClient(opts)
	if err != nil {
		t.Fatalf("NewRestClient()=%v", err)
	}
	if _, err := client.Stats(single()); err == nil {
		t.Fatal("want err!=nil")
	}
	// Ensure no requests were made to Ably servers.
	if n := rec.Len(); n != 0 {
		t.Fatalf("want rec.Len()=0; got %d", n)
	}
}

func TestAuth_RequestToken(t *testing.T) {
	t.Parallel()
	rec, opts := recorder()
	opts.UseTokenAuth = true
	opts.AuthParams = url.Values{"param_1": []string{"this", "should", "get", "overwritten"}}
	defer rec.Stop()
	app, client := ablytest.NewRestClient(opts)
	defer safeclose(t, app)
	server := ablytest.MustAuthReverseProxy(app.Options(opts))
	defer safeclose(t, server)

	if n := rec.Len(); n != 0 {
		t.Fatalf("want rec.Len()=0; got %d", n)
	}
	token, err := client.Auth.RequestToken(nil, nil)
	if err != nil {
		t.Fatalf("RequestToken()=%v", err)
	}
	if n := rec.Len(); n != 1 {
		t.Fatalf("want rec.Len()=1; got %d", n)
	}
	// Enqueue token in the auth reverse proxy - expect it'd be received in response
	// to AuthURL request.
	server.TokenQueue = append(server.TokenQueue, token)
	authOpts := &ably.AuthOptions{
		AuthURL: server.URL("details"),
	}
	token2, err := client.Auth.RequestToken(nil, authOpts)
	if err != nil {
		t.Fatalf("RequestToken()=%v", err)
	}
	// Ensure token was requested from AuthURL.
	if n := rec.Len(); n != 2 {
		t.Fatalf("want rec.Len()=2; got %d", n)
	}
	if got, want := rec.Request(1).URL.Host, server.Listener.Addr().String(); got != want {
		t.Fatalf("want request.URL.Host=%s; got %s", want, got)
	}
	// Again enqueue received token in the auth reverse proxy - expect it'd be returned
	// by the AuthCallback.
	//
	// For "token" and "details" callback the TokenDetails value is obtained from
	// token2, thus token2 and tokCallback are the same.
	rec.Reset()
	for _, callback := range []string{"token", "details"} {
		server.TokenQueue = append(server.TokenQueue, token2)
		authOpts := &ably.AuthOptions{
			AuthCallback: server.Callback(callback),
		}
		tokCallback, err := client.Auth.RequestToken(nil, authOpts)
		if err != nil {
			t.Fatalf("RequestToken()=%v (callback=%s)", err, callback)
		}
		// Ensure no requests to Ably servers were made.
		if n := rec.Len(); n != 0 {
			t.Fatalf("want rec.Len()=0; got %d (callback=%s)", n, callback)
		}
		// Ensure all tokens received from RequestToken are equal.
		if !reflect.DeepEqual(token, token2) {
			t.Fatalf("want token=%v == token2=%v (callback=%s)", token, token2, callback)
		}
		if token2.Token != tokCallback.Token {
			t.Fatalf("want token2.Token=%s == tokCallback.Token=%s (callback=%s)",
				token2.Token, tokCallback.Token, callback)
		}
	}
	// For "request" callback, a TokenRequest value is created from the token2,
	// then it's used to request TokenDetails from the Ably servers.
	server.TokenQueue = append(server.TokenQueue, token2)
	authOpts = &ably.AuthOptions{
		AuthCallback: server.Callback("request"),
	}
	tokCallback, err := client.Auth.RequestToken(nil, authOpts)
	if err != nil {
		t.Fatalf("RequestToken()=%v", err)
	}
	if n := rec.Len(); n != 1 {
		t.Fatalf("want rec.Len()=1; got %d", n)
	}
	if token2.Token == tokCallback.Token {
		t.Fatalf("want token2.Token2=%s != tokCallback.Token=%s", token2.Token, tokCallback.Token)
	}
	// Ensure all headers and params are sent with request to AuthURL.
	for _, method := range []string{"GET", "POST"} {
		// Each iteration records the requests:
		//
		//  0 - RequestToken: request to AuthURL
		//  1 - RequestToken: proxied Auth request to Ably servers
		//  2 - Stats request to Ably API
		//
		// Responses are analogously ordered.
		rec.Reset()
		authOpts = &ably.AuthOptions{
			AuthMethod:  method,
			AuthURL:     server.URL("request"),
			AuthHeaders: http.Header{"X-Header-1": {"header"}, "X-Header-2": {"header"}},
			AuthParams: url.Values{
				"param_1":  {"value"},
				"param_2":  {"value"},
				"clientId": {"should not be overwritten"},
			},
		}
		params := &ably.TokenParams{
			ClientID: "test",
		}
		tokURL, err := client.Auth.RequestToken(params, authOpts)
		if err != nil {
			t.Fatalf("RequestToken()=%v (method=%s)", err, method)
		}
		if tokURL.Token == token2.Token {
			t.Fatalf("want tokURL.Token != token2.Token: %s (method=%s)", tokURL.Token, method)
		}
		req := rec.Request(0)
		if req.Method != method {
			t.Fatalf("want req.Method=%s; got %s", method, req.Method)
		}
		for k := range authOpts.AuthHeaders {
			if got, want := req.Header.Get(k), authOpts.AuthHeaders.Get(k); got != want {
				t.Errorf("want %s; got %s (method=%s)", want, got, method)
			}
		}
		query := ablytest.MustQuery(req)
		for k := range authOpts.AuthParams {
			if k == "clientId" {
				if got := query.Get(k); got != params.ClientID {
					t.Errorf("want client_id=%q to be not overwritten; it was: %q (method=%s)",
						params.ClientID, got, method)
				}
				continue
			}
			if got, want := query.Get(k), authOpts.AuthParams.Get(k); got != want {
				t.Errorf("want %s; got %s (method=%s)", want, got, method)
			}
		}
		var tokReq ably.TokenRequest
		if err := ably.DecodeResp(rec.Response(1), &tokReq); err != nil {
			t.Errorf("token request decode error: %v (method=%s)", err, method)
		}
		if tokReq.ClientID != "test" {
			t.Errorf("want clientID=test; got %v (method=%s)", tokReq.ClientID, method)
		}
		// Call the API with the token obtained via AuthURL.
		optsURL := app.Options(opts)
		optsURL.AuthOptions = ably.AuthOptions{Token: tokURL.Token}
		c, err := ably.NewRestClient(optsURL)
		if err != nil {
			t.Errorf("NewRealtimeClient()=%v", err)
			continue
		}
		if _, err = c.Stats(single()); err != nil {
			t.Errorf("c.Stats()=%v (method=%s)", err, method)
		}
	}
}

func TestAuth_ClientID_Error(t *testing.T) {
	t.Parallel()
	opts := &ably.ClientOptions{
		ClientID: "*",
		AuthOptions: ably.AuthOptions{
			Key:          "abc:abc",
			UseTokenAuth: true,
		},
	}
	_, err := ably.NewRealtimeClient(opts)
	if err := checkError(40102, err); err != nil {
		t.Fatal(err)
	}
}

func TestAuth_ReuseClientID(t *testing.T) {
	t.Parallel()
	opts := &ably.ClientOptions{
		AuthOptions: ably.AuthOptions{
			UseTokenAuth: true,
		},
	}
	app, client := ablytest.NewRestClient(opts)
	defer safeclose(t, app)

	params := &ably.TokenParams{
		ClientID: "reuse-me",
	}
	tok, err := client.Auth.Authorize(params, nil)
	if err != nil {
		t.Fatalf("Authorize()=%v", err)
	}
	if tok.ClientID != params.ClientID {
		t.Fatalf("want ClientID=%q; got %q", params.ClientID, tok.ClientID)
	}
	if clientID := client.Auth.ClientID(); clientID != params.ClientID {
		t.Fatalf("want ClientID=%q; got %q", params.ClientID, tok.ClientID)
	}
	force := &ably.AuthOptions{
		Force: true,
	}
	tok2, err := client.Auth.Authorize(nil, force)
	if err != nil {
		t.Fatalf("Authorize()=%v", err)
	}
	if tok2.ClientID != params.ClientID {
		t.Fatalf("want ClientID=%q; got %q", params.ClientID, tok2.ClientID)
	}
}

func TestAuth_RequestToken_PublishClientID(t *testing.T) {
	t.Parallel()
	app := ablytest.MustSandbox(nil)
	defer safeclose(t, app)
	cases := []struct {
		authAs    string
		publishAs string
		clientID  string
		rejected  bool
	}{
		{"", "", "", false},                         // i=0
		{"", "explicit", "", true},                  // i=1
		{"*", "", "", false},                        // i=2
		{"*", "explicit", "", false},                // i=3
		{"explicit", "different", "explicit", true}, // i=4
	}
	for i, cas := range cases {
		client := app.NewRealtimeClient()
		defer safeclose(t, client)
		params := &ably.TokenParams{
			ClientID: cas.authAs,
		}
		tok, err := client.Auth.RequestToken(params, nil)
		if err != nil {
			t.Errorf("%d: CreateTokenRequest()=%v", i, err)
			continue
		}
		opts := &ably.AuthOptions{
			TokenDetails: tok,
		}
		if _, err = client.Auth.Authorize(params, opts); err != nil {
			t.Errorf("%d: Authorize()=%v", i, err)
			continue
		}
		if id := client.Auth.ClientID(); id != cas.clientID {
			t.Errorf("%d: want ClientID to be %q; got %s", i, cas.clientID, id)
			continue
		}
		channel := client.Channels.GetAndAttach("publish")
		sub, err := channel.Subscribe("test")
		if err != nil {
			t.Errorf("%d: Subscribe()=%v", i, err)
			continue
		}
		msg := []*proto.Message{{
			ClientID: cas.publishAs,
			Name:     "test",
			Data:     &proto.DataValue{Value: "payload"},
		}}
		err = ablytest.Wait(channel.PublishAll(msg))
		if cas.rejected {
			if err == nil {
				t.Errorf("%d: expected message to be rejected", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("%d: PublishAll()=%v", i, err)
			continue
		}
		select {
		case msg := <-sub.MessageChannel():
			if msg.ClientID != cas.publishAs {
				t.Errorf("%d: want ClientID=%q; got %q", i, cas.publishAs, msg.ClientID)
			}
		case <-time.After(ablytest.Timeout):
			t.Errorf("%d: waiting for message timed out after %v", i, ablytest.Timeout)
		}
	}
}

func TestAuth_ClientID(t *testing.T) {
	t.Parallel()
	in := make(chan *proto.ProtocolMessage, 16)
	out := make(chan *proto.ProtocolMessage, 16)
	app := ablytest.MustSandbox(nil)
	defer safeclose(t, app)
	opts := &ably.ClientOptions{
		AuthOptions: ably.AuthOptions{
			UseTokenAuth: true,
		},
	}
	proxy := ablytest.MustAuthReverseProxy(app.Options(opts))
	defer safeclose(t, proxy)
	params := &ably.TokenParams{
		TTL: ably.Duration(time.Second),
	}
	opts = &ably.ClientOptions{
		AuthOptions: ably.AuthOptions{
			AuthURL:      proxy.URL("details"),
			UseTokenAuth: true,
			Force:        true,
		},
		Dial:      ablytest.MessagePipe(in, out),
		NoConnect: true,
	}
	client := app.NewRealtimeClient(opts) // no client.Close as the connection is mocked

	tok, err := client.Auth.RequestToken(params, nil)
	if err != nil {
		t.Fatalf("RequestToken()=%v", err)
	}
	proxy.TokenQueue = append(proxy.TokenQueue, tok)

	tok, err = client.Auth.Authorize(nil, nil)
	if err != nil {
		t.Fatalf("Authorize()=%v", err)
	}
	connected := &proto.ProtocolMessage{
		Action:       proto.ActionConnected,
		ConnectionID: "connection-id",
		ConnectionDetails: &proto.ConnectionDetails{
			ClientID: "client-id",
		},
	}
	// Ensure CONNECTED message changes the empty Auth.ClientID.
	in <- connected
	if id := client.Auth.ClientID(); id != "" {
		t.Fatalf("want clientID to be empty; got %q", id)
	}
	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	if id := client.Auth.ClientID(); id != connected.ConnectionDetails.ClientID {
		t.Fatalf("want clientID=%q; got %q", connected.ConnectionDetails.ClientID, id)
	}
	// Mock the auth reverse proxy to return a token with non-matching ClientID
	// via AuthURL.
	tok.ClientID = "non-matching"
	proxy.TokenQueue = append(proxy.TokenQueue, tok)

	force := &ably.AuthOptions{
		Force: true,
	}
	_, err = client.Auth.Authorize(nil, force)
	if err := checkError(40012, err); err != nil {
		t.Fatal(err)
	}
	closed := &proto.ProtocolMessage{
		Action: proto.ActionClosed,
	}
	in <- closed
	if err := client.Close(); err != nil {
		t.Fatalf("Close()=%v", err)
	}
	time.Sleep(2 * time.Second) // wait for token to expire
	in <- connected
	proxy.TokenQueue = append(proxy.TokenQueue, tok)
	failed := make(chan ably.State, 1)
	client.Connection.On(failed, ably.StateConnFailed)
	err = ablytest.Wait(client.Connection.Connect())
	if err = checkError(40012, err); err != nil {
		t.Fatal(err)
	}
	if state := client.Connection.State(); state != ably.StateConnFailed {
		t.Fatalf("want state=%q; got %q", ably.StateConnFailed, state)
	}
	select {
	case state := <-failed:
		if state.State != ably.StateConnFailed {
			t.Fatalf("want state=%q; got %q", ably.StateConnFailed, state.State)
		}
	default:
		t.Fatal("wanted to have StateConnFailed emitted; it wasn't")
	}
}

func TestAuth_CreateTokenRequest(t *testing.T) {
	t.Parallel()
	app, client := ablytest.NewRestClient(nil)
	defer safeclose(t, app)

	opts := &ably.AuthOptions{
		UseQueryTime: true,
		Key:          app.Key(),
	}
	params := &ably.TokenParams{
		TTL:           ably.Duration(5 * time.Second),
		RawCapability: (ably.Capability{"presence": {"read", "write"}}).Encode(),
	}
	t.Run("RSA9h", func(ts *testing.T) {
		ts.Run("parameters are optional", func(ts *testing.T) {
			_, err := client.Auth.CreateTokenRequest(params, nil)
			if err != nil {
				ts.Fatalf("expected no error to occur got %v instead", err)
			}
			_, err = client.Auth.CreateTokenRequest(nil, opts)
			if err != nil {
				ts.Fatalf("expected no error to occur got %v instead", err)
			}
			_, err = client.Auth.CreateTokenRequest(nil, nil)
			if err != nil {
				ts.Fatalf("expected no error to occur got %v instead", err)
			}
		})
		ts.Run("AuthOptions must not be merged", func(ts *testing.T) {
			opts := &ably.AuthOptions{
				UseQueryTime: true,
			}
			_, err := client.Auth.CreateTokenRequest(params, opts)
			if err == nil {
				ts.Fatal("expected an error")
			}
			e := err.(*ably.Error)
			if e.Code != ably.ErrInvalidCredentials {
				ts.Errorf("expected error code %d got %d", ably.ErrInvalidCredentials, e.Code)
			}

			// overide with bad key
			opts.Key = "some bad key"
			_, err = client.Auth.CreateTokenRequest(params, opts)
			if err == nil {
				ts.Fatal("expected an error")
			}
			e = err.(*ably.Error)
			if e.Code != ably.ErrIncompatibleCredentials {
				ts.Errorf("expected error code %d got %d", ably.ErrIncompatibleCredentials, e.Code)
			}
		})
	})
	t.Run("RSA9c must generate a unique 16+ character nonce", func(ts *testing.T) {
		req, err := client.Auth.CreateTokenRequest(params, opts)
		if err != nil {
			ts.Fatalf("CreateTokenRequest()=%v", err)
		}
		if len(req.Nonce) < 16 {
			ts.Fatalf("want len(nonce)>=16; got %d", len(req.Nonce))
		}
	})
	t.Run("RSA9g generate a signed request", func(ts *testing.T) {
		req, err := client.Auth.CreateTokenRequest(nil, nil)
		if err != nil {
			ts.Fatalf("CreateTokenRequest()=%v", err)
		}
		if req.Mac == "" {
			ts.Fatalf("want mac to be not empty")
		}
	})
}

func TestAuth_RealtimeAccessToken(t *testing.T) {
	t.Parallel()
	rec := ablytest.NewMessageRecorder()
	opts := &ably.ClientOptions{
		ClientID:  "explicit",
		NoConnect: true,
		Dial:      rec.Dial,
	}
	app, client := ablytest.NewRealtimeClient(opts)
	defer safeclose(t, app)

	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatalf("Connect()=%v", err)
	}
	if err := ablytest.Wait(client.Channels.Get("test").Publish("name", "value")); err != nil {
		t.Fatalf("Publish()=%v", err)
	}
	if clientID := client.Auth.ClientID(); clientID != opts.ClientID {
		t.Fatalf("want ClientID=%q; got %q", opts.ClientID, clientID)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close()=%v", err)
	}
	urls := rec.URL()
	if len(urls) == 0 {
		t.Fatal("want urls to be non-empty")
	}
	for _, url := range urls {
		if s := url.Query().Get("access_token"); s == "" {
			t.Errorf("missing access_token param in %q", url)
		}
	}
	for _, msg := range rec.Sent() {
		for _, msg := range msg.Messages {
			if msg.ClientID != "" {
				t.Fatalf("want ClientID to be empty; got %q", msg.ClientID)
			}
		}
	}
}
