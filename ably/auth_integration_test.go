//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func single() *ably.PaginateParams {
	return &ably.PaginateParams{
		Limit:     1,
		Direction: "forwards",
	}
}

func recorder() (*ablytest.RoundTripRecorder, []ably.ClientOption) {
	rec := &ablytest.RoundTripRecorder{}
	return rec, []ably.ClientOption{ably.WithHTTPClient(&http.Client{
		Transport: rec,
	})}
}

func TestAuth_BasicAuth(t *testing.T) {
	rec, extraOpt := recorder()
	defer rec.Stop()
	opts := []ably.ClientOption{ably.WithQueryTime(true)}
	app, client := ablytest.NewREST(append(opts, extraOpt...)...)
	defer safeclose(t, app)

	_, err := client.Time(context.Background())
	assert.NoError(t, err,
		"Expected client.Time to return a nil error, got %v", err)

	_, err = client.Stats().Pages(context.Background())
	assert.NoError(t, err,
		"Expected client.Stats().Pages to return a nil error, got %v", err)
	assert.Equal(t, 2, rec.Len(),
		"Expected rec.Len to return 2, got %d", rec.Len())

	t.Run("RSA2: Should use basic auth as default authentication if an API key exists", func(t *testing.T) {
		assert.Greater(t, len(app.Key()), 0,
			"Expected key length to be > 0, got %d", len(app.Key()))
		assert.Equal(t, ably.AuthBasic, client.Auth.Method(),
			"Expected client.Auth.Method to be AuthBasic, got %d", client.Auth.Method())
	})

	t.Run("RSA1: Should connect to HTTPS by default, trying to connect with non-TLS should result in error", func(t *testing.T) {
		assert.Equal(t, "https", rec.Request(1).URL.Scheme,
			"Expected rec.Request(1).URL.Scheme to be https, got %s", rec.Request(1).URL.Scheme)

		// Can't use basic auth over HTTP.
		_, err := ably.NewREST(app.Options(ably.WithTLS(false))...)
		assert.Error(t, err)
		assert.Equal(t, ably.ErrorCode(40103), ably.UnwrapErrorCode(err),
			"want code=40103; got %d", ably.UnwrapErrorCode(err))
	})

	t.Run("RSA11: API key should follow format KEY_NAME:KEY_SECRET in auth header", func(t *testing.T) {
		decodeAuthHeader := func(req *http.Request) (value string, err error) {
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
		appDecodedAuthHeaderValue, err := decodeAuthHeader(rec.Request(1))
		assert.NoError(t, err,
			"Expected decodeAuthHeader to return a nil error, got %v", err)
		keyFieldCount := len(strings.Split(app.Key(), ":"))
		assert.Equal(t, 2, keyFieldCount,
			"Expected app.Key to have 2 fields, got %d", keyFieldCount)
		assert.Equal(t, app.Key(), appDecodedAuthHeaderValue,
			"Expected app.Key to be the decoded auth header value, got %s", appDecodedAuthHeaderValue)
	})
}

func timeWithin(t, start, end time.Time) error {
	if t.Before(start) || t.After(end) {
		return fmt.Errorf("want t=%v to be within [%v, %v] time span", t, start, end)
	}
	return nil
}

func TestAuth_TokenAuth(t *testing.T) {
	rec, extraOpt := recorder()
	defer rec.Stop()
	opts := []ably.ClientOption{
		ably.WithTLS(false),
		ably.WithUseTokenAuth(true),
		ably.WithQueryTime(true),
	}
	app, client := ablytest.NewREST(append(opts, extraOpt...)...)
	defer safeclose(t, app)

	beforeAuth := time.Now().Add(-time.Second)
	_, err := client.Time(context.Background())
	assert.NoError(t, err,
		"client.Time()=%v", err)
	_, err = client.Stats().Pages(context.Background())
	assert.NoError(t, err,
		"client.Stats()=%v", err)
	// At this points there should be two requests recorded:
	//
	//   - first: explicit call to Time()
	//   - second: implicit call to Time() during token request
	//   - third: token request
	//   - fourth: actual stats request
	//
	assert.Equal(t, 4, rec.Len(),
		"want rec.Len()=4; got %d", rec.Len())
	assert.Equal(t, ably.AuthToken, client.Auth.Method(),
		"want method=2; got %d", client.Auth.Method())
	requestUrl := rec.Request(3).URL
	assert.Equal(t, "http", requestUrl.Scheme,
		"want url.Scheme=http; got %s", requestUrl.Scheme)
	rec.Reset()
	tok, err := client.Auth.Authorize(context.Background(), nil)
	assert.NoError(t, err,
		"Authorize()=%v", err)
	// Call to Authorize should always refresh the token.
	assert.Equal(t, 1, rec.Len(),
		"Authorize() did not return new token; want rec.Len()=1; got %d", rec.Len())
	assert.Equal(t, `{"*":["*"]}`, tok.Capability,
		"want tok.Capability={\"*\":[\"*\"]}; got %v", tok.Capability)
	now := time.Now().Add(time.Second)
	err = timeWithin(tok.IssueTime(), beforeAuth, now)
	assert.NoError(t, err)

	// Ensure token expires in 60m (default TTL).
	beforeAuth = beforeAuth.Add(60 * time.Minute)
	now = now.Add(60 * time.Minute)
	err = timeWithin(tok.ExpireTime(), beforeAuth, now)
	assert.NoError(t, err)
}

func TestAuth_TokenAuth_Renew(t *testing.T) {
	rec, extraOpt := recorder()
	defer rec.Stop()
	opts := []ably.ClientOption{ably.WithUseTokenAuth(true)}
	app, client := ablytest.NewREST(append(opts, extraOpt...)...)
	defer safeclose(t, app)

	params := &ably.TokenParams{
		TTL: time.Second.Milliseconds(),
	}
	tok, err := client.Auth.Authorize(context.Background(), params)
	assert.NoError(t, err)
	assert.Equal(t, 1, rec.Len(),
		"want rec.Len()=1; got %d", rec.Len())
	ttl := tok.ExpireTime().Sub(tok.IssueTime())
	assert.Equal(t, 1.0, ttl.Seconds(),
		"want ttl=1s; got %v", ttl)
	time.Sleep(2 * time.Second) // wait till expires
	_, err = client.Stats().Pages(context.Background())
	assert.NoError(t, err,
		"Stats()=%v", err)
	// Recorded responses:
	//
	//   - 0: response for explicit Authorize()
	//   - 1: response for implicit Authorize() (token renewal)
	//   - 2: response for Stats()
	//
	assert.Equal(t, 3, rec.Len(),
		"token not renewed; want rec.Len()=3; got %d", rec.Len())
	var newTok ably.TokenDetails
	err = ably.DecodeResp(rec.Response(1), &newTok)
	assert.NoError(t, err,
		"token decode error: %v", err)
	assert.NotEqual(t, newTok.Token, tok.Token,
		"token not renewed; new token equals old: %s", tok.Token)
	// Ensure token was renewed with original params.
	ttl = newTok.ExpireTime().Sub(newTok.IssueTime())
	assert.Equal(t, 1.0, ttl.Seconds(),
		"want ttl=1s; got %v", ttl)
	time.Sleep(2 * time.Second) // wait for token to expire
	// Ensure request fails when Token or *TokenDetails is provided, but no
	// means to renew the token
	rec.Reset()
	opts = app.Options(opts...)
	opts = append(opts, ably.WithKey(""), ably.WithTokenDetails(tok))
	client, err = ably.NewREST(opts...)
	assert.NoError(t, err,
		"NewREST()=%v", err)
	_, err = client.Stats().Pages(context.Background())
	assert.Error(t, err)
	// Ensure no requests were made to Ably servers.
	assert.Equal(t, 0, rec.Len(),
		"want rec.Len()=0; got %d", rec.Len())
}

func TestAuth_RequestToken(t *testing.T) {
	rec, extraOpt := recorder()
	opts := []ably.ClientOption{
		ably.WithUseTokenAuth(true),
		ably.WithAuthParams(url.Values{"param_1": []string{"this", "should", "get", "overwritten"}}),
	}
	defer rec.Stop()
	app, client := ablytest.NewREST(append(opts, extraOpt...)...)
	defer safeclose(t, app)
	server := ablytest.MustAuthReverseProxy(app.Options(append(opts, extraOpt...)...)...)
	defer safeclose(t, server)

	assert.Equal(t, 0, rec.Len(),
		"want rec.Len()=0; got %d", rec.Len())
	token, err := client.Auth.RequestToken(context.Background(), nil)
	assert.NoError(t, err,
		"RequestToken()=%v", err)
	assert.Equal(t, 1, rec.Len(),
		"want rec.Len()=1; got %d", rec.Len())
	// Enqueue token in the auth reverse proxy - expect it'd be received in response
	// to AuthURL request.
	server.TokenQueue = append(server.TokenQueue, token)
	authOpts := []ably.AuthOption{
		ably.AuthWithURL(server.URL("details")),
	}
	token2, err := client.Auth.RequestToken(context.Background(), nil, authOpts...)
	assert.NoError(t, err,
		"RequestToken()=%v", err)
	// Ensure token was requested from AuthURL.
	assert.Equal(t, 2, rec.Len(),
		"want rec.Len()=2; got %d", rec.Len())
	assert.Equal(t, server.Listener.Addr().String(), rec.Request(1).URL.Host,
		"want request.URL.Host=%s; got %s", server.Listener.Addr().String(), rec.Request(1).URL.Host)
	// Again enqueue received token in the auth reverse proxy - expect it'd be returned
	// by the AuthCallback.
	//
	// For "token" and "details" callback the TokenDetails value is obtained from
	// token2, thus token2 and tokCallback are the same.
	rec.Reset()
	for _, callback := range []string{"token", "details"} {
		server.TokenQueue = append(server.TokenQueue, token2)
		authOpts := []ably.AuthOption{
			ably.AuthWithCallback(server.Callback(callback)),
		}
		tokCallback, err := client.Auth.RequestToken(context.Background(), nil, authOpts...)
		assert.NoError(t, err,
			"RequestToken()=%v (callback=%s)", err, callback)
		// Ensure no requests to Ably servers were made.
		assert.Equal(t, 0, rec.Len(),
			"want rec.Len()=0; got %d (callback=%s)", rec.Len(), callback)
		// Ensure all tokens received from RequestToken are equal.
		assert.Equal(t, token, token2,
			"want token=%v == token2=%v (callback=%s)", token, token2, callback)
		assert.Equal(t, token2.Token, tokCallback.Token,
			"want token2.Token=%s == tokCallback.Token=%s (callback=%s)", token2.Token, tokCallback.Token, callback)
	}
	// For "request" callback, a TokenRequest value is created from the token2,
	// then it's used to request TokenDetails from the Ably servers.
	server.TokenQueue = append(server.TokenQueue, token2)
	authOpts = []ably.AuthOption{
		ably.AuthWithCallback(server.Callback("request")),
	}
	tokCallback, err := client.Auth.RequestToken(context.Background(), nil, authOpts...)
	assert.NoError(t, err,
		"RequestToken()=%v", err)
	assert.Equal(t, 1, rec.Len(),
		"want rec.Len()=1; got %d", rec.Len())
	assert.NotEqual(t, token2.Token, tokCallback.Token,
		"want token2.Token2=%s != tokCallback.Token=%s", token2.Token, tokCallback.Token)
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
		authHeaders := http.Header{"X-Header-1": {"header"}, "X-Header-2": {"header"}}
		authParams := url.Values{
			"param_1":  {"value"},
			"param_2":  {"value"},
			"clientId": {"should not be overwritten"},
		}
		authOpts = []ably.AuthOption{
			ably.AuthWithMethod(method),
			ably.AuthWithURL(server.URL("request")),
			ably.AuthWithHeaders(authHeaders),
			ably.AuthWithParams(authParams),
		}
		params := &ably.TokenParams{
			ClientID: "test",
		}

		tokURL, err := client.Auth.RequestToken(context.Background(), params, authOpts...)
		assert.NoError(t, err,
			"RequestToken()=%v (method=%s)", err, method)
		assert.NotEqual(t, tokURL.Token, token2.Token,
			"want tokURL.Token != token2.Token: %s (method=%s)", tokURL.Token, method)
		req := rec.Request(0)
		assert.Equal(t, req.Method, method,
			"want req.Method=%s; got %s", method, req.Method)
		for k := range authHeaders {
			assert.Equal(t, authHeaders.Get(k), req.Header.Get(k),
				"(method=%s)", method)
		}
		query := ablytest.MustQuery(req)
		for k := range authParams {
			if k == "clientId" {
				assert.Equal(t, params.ClientID, query.Get(k),
					"want client_id=%q to be not overwritten; it was: %q (method=%s)", params.ClientID, query.Get(k), method)
				continue
			}
			assert.Equal(t, authParams.Get(k), query.Get(k),
				"param:%s; want %q; got %q (method=%s)", k, authParams.Get(k), query.Get(k), method)
		}
		var tokReq ably.TokenRequest
		err = ably.DecodeResp(rec.Response(1), &tokReq)
		assert.NoError(t, err,
			"token request decode error: %v (method=%s)", err, method)
		assert.Equal(t, "test", tokReq.ClientID,
			"want clientID=test; got %v (method=%s)", tokReq.ClientID, method)
		// Call the API with the token obtained via AuthURL.
		optsURL := append(app.Options(opts...),
			ably.WithToken(tokURL.Token),
		)
		c, err := ably.NewREST(optsURL...)
		assert.NoError(t, err,
			"NewREST()=%v", err)
		_, err = c.Stats().Pages(context.Background())
		assert.NoError(t, err,
			"c.Stats()=%v (method=%s)", err, method)
	}
}

func TestAuth_ReuseClientID(t *testing.T) {
	opts := []ably.ClientOption{ably.WithUseTokenAuth(true)}
	app, client := ablytest.NewREST(opts...)
	defer safeclose(t, app)

	params := &ably.TokenParams{
		ClientID: "reuse-me",
	}
	tok, err := client.Auth.Authorize(context.Background(), params)
	assert.NoError(t, err,
		"Authorize()=%v", err)
	assert.Equal(t, params.ClientID, tok.ClientID,
		"want ClientID=%q; got %q", params.ClientID, tok.ClientID)
	assert.Equal(t, params.ClientID, client.Auth.ClientID(),
		"want ClientID=%q; got %q", params.ClientID, client.Auth.ClientID())
	tok2, err := client.Auth.Authorize(context.Background(), nil)
	assert.NoError(t, err,
		"Authorize()=%v", err)
	assert.Equal(t, params.ClientID, tok2.ClientID,
		"want ClientID=%q; got %q", params.ClientID, tok2.ClientID)
}

func TestAuth_RequestToken_PublishClientID(t *testing.T) {
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
		rclient, err := ably.NewREST(app.Options()...)
		assert.NoError(t, err)
		params := &ably.TokenParams{
			ClientID: cas.authAs,
		}
		tok, err := rclient.Auth.RequestToken(context.Background(), params)
		assert.NoError(t, err,
			"%d: CreateTokenRequest()=%v", i, err)
		opts := []ably.ClientOption{
			ably.WithTokenDetails(tok),
			ably.WithUseTokenAuth(true),
		}
		if i == 4 {
			opts = append(opts, ably.WithClientID(cas.clientID))
		}
		client := app.NewRealtime(opts...)
		defer safeclose(t, ablytest.FullRealtimeCloser(client))
		err = ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
		assert.NoError(t, err,
			"Connect(): want err == nil got err=%v", err)
		assert.Equal(t, cas.clientID, client.Auth.ClientID(),
			"%d: want ClientID to be %q; got %s", i, cas.clientID, client.Auth.ClientID())
		channel := client.Channels.Get("publish")
		err = channel.Attach(context.Background())
		assert.NoError(t, err)
		messages, unsub, err := ablytest.ReceiveMessages(channel, "test")
		defer unsub()
		assert.NoError(t, err,
			"%d:.Subscribe(context.Background())=%v", i, err)
		msg := []*ably.Message{{
			ClientID: cas.publishAs,
			Name:     "test",
			Data:     "payload",
		}}
		err = channel.PublishMultiple(context.Background(), msg)
		if cas.rejected {
			assert.Error(t, err,
				"%d: expected message to be rejected %#v", i, cas)
			continue
		}
		assert.NoError(t, err,
			"%d: PublishMultiple()=%v", i, err)
		select {
		case msg := <-messages:
			assert.Equal(t, cas.publishAs, msg.ClientID,
				"%d: want ClientID=%q; got %q", i, cas.publishAs, msg.ClientID)
		case <-time.After(ablytest.Timeout):
			t.Errorf("%d: waiting for message timed out after %v", i, ablytest.Timeout)
		}
	}
}

func TestAuth_ClientID(t *testing.T) {
	in := make(chan *ably.ProtocolMessage, 16)
	out := make(chan *ably.ProtocolMessage, 16)
	app := ablytest.MustSandbox(nil)
	defer safeclose(t, app)
	opts := []ably.ClientOption{
		ably.WithUseTokenAuth(true),
	}
	proxy := ablytest.MustAuthReverseProxy(app.Options(opts...)...)
	defer safeclose(t, proxy)
	params := &ably.TokenParams{
		TTL: time.Second.Milliseconds(),
	}
	opts = []ably.ClientOption{
		ably.WithAuthURL(proxy.URL("details")),
		ably.WithUseTokenAuth(true),
		ably.WithDial(MessagePipe(in, out)),
		ably.WithAutoConnect(false),
	}
	client := app.NewRealtime(opts...) // no client.Close as the connection is mocked

	t.Run("Auth_ClientID", func(t *testing.T) {

		tok, err := client.Auth.RequestToken(context.Background(), params)
		assert.NoError(t, err,
			"RequestToken()=%v", err)
		proxy.TokenQueue = append(proxy.TokenQueue, tok)

		tok, err = client.Auth.Authorize(context.Background(), nil)
		assert.NoError(t, err,
			"Authorize()=%v", err)
		connected := &ably.ProtocolMessage{
			Action:       ably.ActionConnected,
			ConnectionID: "connection-id",
			ConnectionDetails: &ably.ConnectionDetails{
				ClientID: "client-id",
			},
		}
		// Ensure CONNECTED message changes the empty Auth.ClientID.
		in <- connected
		assert.Equal(t, "", client.Auth.ClientID(),
			"want clientID to be empty; got %q", client.Auth.ClientID())
		err = ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
		assert.NoError(t, err,
			"Connect()=%v", err)
		assert.Equal(t, connected.ConnectionDetails.ClientID, client.Auth.ClientID(),
			"want clientID=%q; got %q", connected.ConnectionDetails.ClientID, client.Auth.ClientID())
		// Mock the auth reverse proxy to return a token with non-matching ClientID
		// via AuthURL.
		tok.ClientID = "non-matching"
		proxy.TokenQueue = append(proxy.TokenQueue, tok)

		_, err = client.Auth.Authorize(context.Background(), nil)
		err = checkError(40012, err)
		assert.NoError(t, err)
		// After the current token expires, reconnecting should request a new token
		// from authURL. Make it return the token with the mismatched client ID, and
		// expect a transition to FAILED.

		time.Sleep(time.Duration(params.TTL) * time.Millisecond)
		tokenExpiredAt := time.Now()

		// Allow some extra time for the server to reject our token.
		tokenErrorDeadline := tokenExpiredAt.Add(5 * time.Second)

		// Close and reconnect with the expired token, until we get an error.
		// Expiration deadlines are inexact, so this may take a few tries until the
		// token is effectively expired.
		err = nil
		for err == nil && time.Now().Before(tokenErrorDeadline) {
			time.Sleep(100 * time.Millisecond)

			err = ablytest.Wait(ablytest.ConnWaiter(client, client.Close, ably.ConnectionEventClosing), nil)
			assert.NoError(t, err,
				"Close()=%v", err)
			err = ablytest.Wait(ablytest.ConnWaiter(client, func() {
				closed := &ably.ProtocolMessage{
					Action: ably.ActionClosed,
				}
				in <- closed
			}, ably.ConnectionEventClosed), nil)
			assert.NoError(t, err,
				"waiting for close: %v", err)
			in <- connected
			proxy.TokenQueue = append(proxy.TokenQueue, tok)
			err = ablytest.Wait(ablytest.ConnWaiter(client, client.Connect,
				ably.ConnectionEventConnected,
				ably.ConnectionEventFailed,
			), nil)
		}
		err = checkError(40012, err)
		assert.NoError(t, err)

		assert.Equal(t, ably.ConnectionStateFailed, client.Connection.State(),
			"want state=%q; got %q", ably.ConnectionStateFailed, client.Connection.State())
	})
}

func TestAuth_CreateTokenRequest(t *testing.T) {
	app, client := ablytest.NewREST()
	defer safeclose(t, app)

	opts := []ably.AuthOption{
		ably.AuthWithQueryTime(true),
		ably.AuthWithKey(app.Key()),
	}
	params := &ably.TokenParams{
		TTL:        (5 * time.Second).Milliseconds(),
		Capability: `{"presence":["read", "write"]}`,
	}
	t.Run("RSA9h", func(t *testing.T) {
		t.Run("parameters are optional", func(t *testing.T) {
			_, err := client.Auth.CreateTokenRequest(params)
			assert.NoError(t, err)
			_, err = client.Auth.CreateTokenRequest(nil, opts...)
			assert.NoError(t, err)
			_, err = client.Auth.CreateTokenRequest(nil)
			assert.NoError(t, err)
		})
		t.Run("authOptions must not be merged", func(t *testing.T) {
			opts := []ably.AuthOption{ably.AuthWithQueryTime(true)}
			_, err := client.Auth.CreateTokenRequest(params, opts...)
			assert.Error(t, err)
			e := err.(*ably.ErrorInfo)
			assert.Equal(t, ably.ErrInvalidCredentials, e.Code,
				"expected error code %d got %d", ably.ErrInvalidCredentials, e.Code)
			// override with bad key
			opts = append(opts, ably.AuthWithKey("some bad key"))
			_, err = client.Auth.CreateTokenRequest(params, opts...)
			assert.Error(t, err)
			e = err.(*ably.ErrorInfo)
			assert.Equal(t, ably.ErrIncompatibleCredentials, e.Code,
				"expected error code %d got %d", ably.ErrIncompatibleCredentials, e.Code)
		})
	})
	t.Run("RSA9c must generate a unique 16+ character nonce", func(t *testing.T) {
		req, err := client.Auth.CreateTokenRequest(params, opts...)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(req.Nonce), 16,
			"want len(nonce)>=16; got %d", len(req.Nonce))
	})
	t.Run("RSA9g generate a signed request", func(t *testing.T) {
		req, err := client.Auth.CreateTokenRequest(nil)
		assert.NoError(t, err,
			"CreateTokenRequest()=%v", err)
		assert.NotEqual(t, "", req.MAC,
			"want mac to be not empty")
	})
}

func TestAuth_RealtimeAccessToken(t *testing.T) {
	rec := NewMessageRecorder()
	const explicitClientID = "explicit"
	opts := []ably.ClientOption{
		ably.WithClientID(explicitClientID),
		ably.WithAutoConnect(false),
		ably.WithDial(rec.Dial),
		ably.WithUseTokenAuth(true),
	}
	app, client := ablytest.NewRealtime(opts...)
	defer safeclose(t, app)

	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err,
		"Connect()=%v", err)
	err = client.Channels.Get("test").Publish(context.Background(), "name", "value")
	assert.NoError(t, err,
		"Publish()=%v", err)
	assert.Equal(t, explicitClientID, client.Auth.ClientID(),
		"want ClientID=%q; got %q", explicitClientID, client.Auth.ClientID())
	err = ablytest.FullRealtimeCloser(client).Close()
	assert.NoError(t, err,
		"Close()=%v", err)
	recUrls := rec.URLs()
	assert.NotEqual(t, 0, len(recUrls),
		"want urls to be non-empty")
	for _, recUrl := range recUrls {
		assert.NotEqual(t, "", recUrl.Query().Get("access_token"),
			"missing access_token param in %q", recUrl)
	}
	for _, msg := range rec.Sent() {
		for _, msg := range msg.Messages {
			assert.Equal(t, "", msg.ClientID,
				"want ClientID to be empty; got %q", msg.ClientID)
		}
	}
}

func TestAuth_IgnoreTimestamp_QueryTime(t *testing.T) {
	rec, extraOpt := recorder()
	defer rec.Stop()

	tests := map[string]struct {
		opt                 []ably.ClientOption
		initialUseQueryTime bool
		newUseQueryTime     bool
		tokenParams         *ably.TokenParams
	}{
		"Should not save query time and timestamp when WithQueryTime is false and token params has no timestamp": {
			opt: []ably.ClientOption{
				ably.WithTLS(true),
				ably.WithUseTokenAuth(true),
				ably.WithQueryTime(false),
			},
			tokenParams: &ably.TokenParams{
				TTL:        123890,
				Capability: `{"foo":["publish"]}`,
				ClientID:   "abcd1234",
			},
			initialUseQueryTime: false,
			newUseQueryTime:     true,
		},
		"Should not save query time and timestamp when WithQueryTime is true and token params has no timestamp": {
			opt: []ably.ClientOption{
				ably.WithTLS(true),
				ably.WithUseTokenAuth(true),
				ably.WithQueryTime(true),
			},
			tokenParams: &ably.TokenParams{
				TTL:        123890,
				Capability: `{"foo":["publish"]}`,
				ClientID:   "abcd1234",
			},
			initialUseQueryTime: true,
			newUseQueryTime:     false,
		},
		"Should not save query time and timestamp when WithQueryTime is true and token params has a timestamp": {
			opt: []ably.ClientOption{
				ably.WithTLS(true),
				ably.WithUseTokenAuth(true),
				ably.WithQueryTime(true),
			},
			tokenParams: &ably.TokenParams{
				TTL:        123890,
				Capability: `{"foo":["publish"]}`,
				ClientID:   "abcd1234",
				Timestamp:  time.Now().UnixNano() / int64(time.Millisecond),
			},
			initialUseQueryTime: true,
		},
		"Should not save query time and timestamp when WithQueryTime is false and token params has a timestamp": {
			opt: []ably.ClientOption{
				ably.WithTLS(true),
				ably.WithUseTokenAuth(true),
				ably.WithQueryTime(false),
			},
			tokenParams: &ably.TokenParams{
				TTL:        123890,
				Capability: `{"foo":["publish"]}`,
				ClientID:   "abcd1234",
				Timestamp:  time.Now().UnixNano() / int64(time.Millisecond),
			},
			initialUseQueryTime: false,
			newUseQueryTime:     true,
		},
	}

	for _, test := range tests {
		app, client := ablytest.NewREST(append(test.opt, extraOpt...)...)
		defer safeclose(t, app)
		prevTokenParams := client.Auth.Params()
		prevAuthOptions := client.Auth.AuthOptions()
		prevUseQueryTime := prevAuthOptions.UseQueryTime

		assert.Equal(t, test.initialUseQueryTime, prevUseQueryTime)

		newAuthOptions := []ably.AuthOption{
			ably.AuthWithMethod("POST"),
			ably.AuthWithKey(app.Key()),
			ably.AuthWithQueryTime(test.newUseQueryTime),
		}

		_, err := client.Auth.Authorize(context.Background(), test.tokenParams, newAuthOptions...) // Call to Authorize should always refresh the token.
		assert.NoError(t, err)
		updatedParams := client.Auth.Params()
		updatedAuthOptions := client.Auth.AuthOptions()
		updatedUseQueryTime := updatedAuthOptions.UseQueryTime

		assert.Nil(t, prevTokenParams)
		assert.Equal(t, test.tokenParams, updatedParams) // RSA10J
		assert.Zero(t, updatedParams.Timestamp)          // RSA10g

		assert.Equal(t, prevAuthOptions, updatedAuthOptions)      // RSA10J
		assert.NotEqual(t, prevUseQueryTime, updatedUseQueryTime) // RSA10g
	}
}

func TestAuth_RSA7c(t *testing.T) {
	app := ablytest.MustSandbox(nil)
	defer safeclose(t, app)
	opts := app.Options()
	opts = append(opts, ably.WithClientID("*"))
	_, err := ably.NewREST(opts...)
	assert.Error(t, err)
}
