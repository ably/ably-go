package ably_test

import (
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
	"github.com/ably/ably-go/ably/testutil"
)

func recorder() (*ably.RoundTripRecorder, *ably.ClientOptions) {
	rec := &ably.RoundTripRecorder{}
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
	defer rec.Stop()
	app, client := testutil.ProvisionRest(opts)
	defer safeclose(t, app)

	if _, err := client.Time(); err != nil {
		t.Fatalf("client.Time()=%v", err)
	}
	if rec.Len() != 1 {
		t.Fatalf("want rec.Len()=1; got %d", rec.Len())
	}
	if method := client.Auth.Method; method != ably.AuthBasic {
		t.Fatalf("want method=basic; got %s", method)
	}
	url := rec.Request(0).URL
	if url.Scheme != "https" {
		t.Fatalf("want url.Scheme=https; got %s", url.Scheme)
	}
	auth, err := authValue(rec.Request(0))
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
	app, client := testutil.ProvisionRest(opts)
	defer safeclose(t, app)

	beforeAuth := time.Now()
	if _, err := client.Time(); err != nil {
		t.Fatalf("client.Time()=%v", err)
	}
	// At this points there should be two requests recorded:
	//
	//   - first: the one requesting token
	//   - second: actual time request
	//
	if rec.Len() != 2 {
		t.Fatalf("want rec.Len()=2; got %d", rec.Len())
	}
	if client.Auth.Method != ably.AuthToken {
		t.Fatalf("want method=token; got %s", client.Auth.Method)
	}
	url := rec.Request(1).URL
	if url.Scheme != "http" {
		t.Fatalf("want url.Scheme=http; got %s", url.Scheme)
	}
	auth, err := authValue(rec.Request(1))
	if err != nil {
		t.Fatalf("authValue=%v", err)
	}
	tok, err := client.Auth.Authorise(nil, nil, false)
	if err != nil {
		t.Fatalf("Authorise()=%v", err)
	}
	// Call to Authorise with force set to false should not refresh the token
	// and return existing one.
	// The following ensures no token request was made.
	if rec.Len() != 2 {
		t.Fatal("Authorise() did not return existing token")
	}
	if auth != tok.Token {
		t.Fatalf("want auth=%q; got %q", tok.Token, auth)
	}
	if defaultCap := (ably.Capability{"*": {"*"}}); tok.RawCapability != defaultCap.Encode() {
		t.Fatalf("want tok.Capability=%v; got %v", defaultCap, tok.Capability())
	}
	now := time.Now()
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

func TestAuth_RequestToken(t *testing.T) {
	t.Parallel()
	rec, opts := recorder()
	opts.UseTokenAuth = true
	defer rec.Stop()
	app, client := testutil.ProvisionRest(opts)
	defer safeclose(t, app)
	server := ably.MustAuthReverseProxy(app.Options(opts))
	defer safeclose(t, server)

	if rec.Len() != 0 {
		t.Fatalf("want rec.Len()=0; got %d", rec.Len())
	}
	token, err := client.Auth.RequestToken(nil, nil)
	if err != nil {
		t.Fatalf("RequestToken()=%v", err)
	}
	if rec.Len() != 1 {
		t.Fatalf("want rec.Len()=1; got %d", rec.Len())
	}
	// Enqueue token in the auth reverse proxy - expect it'd be received in response
	// to AuthURL request.
	server.TokenQueue = append(server.TokenQueue, token)
	authOpts := &ably.AuthOptions{
		AuthURL: server.URL("details"),
	}
	token2, err := client.Auth.RequestToken(authOpts, nil)
	if err != nil {
		t.Fatalf("RequestToken()=%v", err)
	}
	// Ensure token was requested from AuthURL.
	if rec.Len() != 2 {
		t.Fatalf("want rec.Len()=2; got %d", rec.Len())
	}
	if got, want := rec.Request(1).URL.Host, server.Listener.Addr().String(); got != want {
		t.Fatalf("want request.URL.Host=%s; got %s", want, got)
	}
	// Again enqueue received token in the auth reverse proxy - expect it'd be returned
	// by the AuthCallback.
	server.TokenQueue = append(server.TokenQueue, token2)
	authOpts = &ably.AuthOptions{
		AuthCallback: server.Callback("token"),
	}
	token3, err := client.Auth.RequestToken(authOpts, nil)
	if err != nil {
		t.Fatalf("RequestToken()=%v", err)
	}
	// token2 and token3 were obtained from the original token value, thus
	// ensure no request to Ably servers were made.
	if rec.Len() != 2 {
		t.Fatalf("want rec.Len()=2; got %d", rec.Len())
	}
	// Ensure all tokens received from RequestToken are equal.
	if !reflect.DeepEqual(token, token2) {
		t.Fatalf("want token=%v == token2=%v", token, token2)
	}
	if token2.Token != token3.Token {
		t.Fatalf("want token2.Token=%s == token3.Token=%s", token2.Token, token3.Token)
	}
	// Ensure all headers and params are sent with request to AuthURL.
	authOpts = &ably.AuthOptions{
		AuthMethod:  "POST",
		AuthURL:     server.URL("request"),
		AuthHeaders: http.Header{"X-Header-1": {"header"}, "X-Header-2": {"header"}},
		AuthParams:  url.Values{"param_1": {"value"}, "param_2": {"value"}},
	}
	if _, err = client.Auth.RequestToken(authOpts, nil); err != nil {
		t.Fatalf("RequestToken()=%v", err)
	}
	if rec.Len() != 3 {
		t.Fatalf("want rec.Len()=3; got %d", rec.Len())
	}
	req := rec.Request(2)
	if req.Method != "POST" {
		t.Fatalf("want req.Method=POST; got %s", req.Method)
	}
	for k := range authOpts.AuthHeaders {
		if got, want := req.Header.Get(k), authOpts.AuthHeaders.Get(k); got != want {
			t.Errorf("want %s; got %s", want, got)
		}
	}
	query := req.URL.Query()
	for k := range authOpts.AuthParams {
		if got, want := query.Get(k), authOpts.AuthParams.Get(k); got != want {
			t.Errorf("want %s; got %s", want, got)
		}
	}
}
