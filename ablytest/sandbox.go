package ablytest

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/ably/ably-go/ably"
)

type Key struct {
	ID         string `json:"id,omitempty"`
	ScopeID    string `json:"scopeId,omitempty"`
	Status     int    `json:"status,omitempty"`
	Type       int    `json:"type,omitempty"`
	Value      string `json:"value,omitempty"`
	Created    int    `json:"created,omitempty"`
	Modified   int    `json:"modified,omitempty"`
	Capability string `json:"capability,omitempty"`
	Expires    int    `json:"expired,omitempty"`
	Privileged bool   `json:"privileged,omitempty"`
}

type Namespace struct {
	ID        string `json:"id"`
	Created   int    `json:"created,omitempty"`
	Modified  int    `json:"modified,omitempty"`
	Persisted bool   `json:"persisted,omitempty"`
}

type Presence struct {
	ClientID string `json:"clientId"`
	Data     string `json:"data"`
	Encoding string `json:"encoding,omitempty"`
}

type Channel struct {
	Name     string     `json:"name"`
	Presence []Presence `json:"presence,omitempty"`
}

type Connection struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type Config struct {
	ID          string       `json:"id,omitempty"`
	AppID       string       `json:"appId,omitempty"`
	AccountID   string       `json:"accountId,omitempty"`
	Status      int          `json:"status,omitempty"`
	Created     int          `json:"created,omitempty"`
	Modified    int          `json:"modified,omitempty"`
	TLSOnly     bool         `json:"tlsOnly,omitempty"`
	Labels      string       `json:"labels,omitempty"`
	Keys        []Key        `json:"keys"`
	Namespaces  []Namespace  `json:"namespaces"`
	Channels    []Channel    `json:"channels"`
	Connections []Connection `json:"connections,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Keys: []Key{
			{
				Capability: `{"[*]*":["*"]}`,
			},
		},
		Namespaces: []Namespace{
			{ID: "persisted", Persisted: true},
		},
		Channels: []Channel{
			{
				Name:     "persisted:presence_fixtures",
				Presence: PresenceFixtures(),
			},
		},
	}
}

var PresenceFixtures = func() []Presence {
	return []Presence{
		{ClientID: "client_bool", Data: "true"},
		{ClientID: "client_int", Data: "true"},
		{ClientID: "client_string", Data: "true"},
		{ClientID: "client_json", Data: `{"test": "This is a JSONObject clientData payload"}`},
	}
}

type Sandbox struct {
	Config      *Config
	Environment string

	client *http.Client
}

func NewRealtime(opts ...ably.ClientOption) (*Sandbox, *ably.Realtime) {
	app := MustSandbox(nil)
	client, err := ably.NewRealtime(app.Options(opts...)...)
	if err != nil {
		panic(nonil(err, app.Close()))
	}
	return app, client
}

func NewREST(opts ...ably.ClientOption) (*Sandbox, *ably.REST) {
	app := MustSandbox(nil)
	client, err := ably.NewREST(app.Options(opts...)...)
	if err != nil {
		panic(nonil(err, app.Close()))
	}
	return app, client
}

func MustSandbox(config *Config) *Sandbox {
	app, err := NewSandbox(nil)
	if err != nil {
		panic(err)
	}
	return app
}

func NewSandbox(config *Config) (*Sandbox, error) {
	return NewSandboxWithEnv(config, Environment)
}

// This code is copied from options.go
func hasActiveInternetConnection(httpClient *http.Client) bool {
	res, err := httpClient.Get("https://internet-up.ably-realtime.com/is-the-internet-up.txt")
	if err != nil || res.StatusCode != 200 {
		return false
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte("yes"))
}

func NewSandboxWithEnv(config *Config, env string) (*Sandbox, error) {
	httpClient := NewHTTPClient()
	if !hasActiveInternetConnection(httpClient) {
		return nil, errors.New("internet is not available, cannot setup ably sandbox")
	}
	app := &Sandbox{
		Config:      config,
		Environment: env,
		client:      httpClient,
	}
	if app.Config == nil {
		app.Config = DefaultConfig()
	}
	p, err := json.Marshal(app.Config)
	if err != nil {
		return nil, err
	}

	const RetryCount = 4
	retryInterval := time.Second
	for requestAttempt := 0; requestAttempt < RetryCount; requestAttempt++ {
		req, err := http.NewRequest("POST", app.URL("apps"), bytes.NewReader(p))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := app.client.Do(req)
		if err != nil {
			if !errors.Is(err, syscall.ECONNRESET) { // if not connection reset by peer
				// return error if it wasn't due to a timeout
				if err, ok := err.(*url.Error); ok && !err.Timeout() {
					return nil, err
				}
			}
		}

		if err != nil || (resp != nil && resp.StatusCode == 504) { // gateway timeout
			// Timeout. Back off before allowing another attempt.
			log.Println("warn: request timeout, attempting retry")
			time.Sleep(retryInterval)
			retryInterval *= 2
		} else {
			defer resp.Body.Close()
			if resp.StatusCode > 299 {
				err := errors.New(http.StatusText(resp.StatusCode))
				if p, e := io.ReadAll(resp.Body); e == nil && len(p) != 0 {
					err = fmt.Errorf("request error: %s (%q)", err, p)
				}
				return nil, err
			}
			if err := json.NewDecoder(resp.Body).Decode(app.Config); err != nil {
				return nil, err
			}
			return app, nil
		}
	}

	return nil, fmt.Errorf("Failed to request sandbox app after %d attempts.", RetryCount)
}

func (app *Sandbox) Close() error {
	req, err := http.NewRequest("DELETE", app.URL("apps", app.Config.AppID), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(app.KeyParts())
	resp, err := app.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return errors.New(http.StatusText(resp.StatusCode))
	}
	return nil
}

func (app *Sandbox) NewRealtime(opts ...ably.ClientOption) *ably.Realtime {
	client, err := ably.NewRealtime(app.Options(opts...)...)
	if err != nil {
		panic("ably.NewRealtime failed: " + err.Error())
	}
	return client
}

func (app *Sandbox) KeyParts() (name, secret string) {
	return app.Config.AppID + "." + app.Config.Keys[0].ID, app.Config.Keys[0].Value
}

func (app *Sandbox) Key() string {
	name, secret := app.KeyParts()
	return name + ":" + secret
}

func (app *Sandbox) Options(opts ...ably.ClientOption) []ably.ClientOption {
	type transportHijacker interface {
		Hijack(http.RoundTripper) http.RoundTripper
	}
	appHTTPClient := NewHTTPClient()
	appOpts := []ably.ClientOption{
		ably.WithKey(app.Key()),
		ably.WithEnvironment(app.Environment),
		ably.WithUseBinaryProtocol(!NoBinaryProtocol),
		ably.WithHTTPClient(appHTTPClient),
		ably.WithLogLevel(DefaultLogLevel),
	}

	// If opts want to record round trips inject the recording transport
	// via TransportHijacker interface.
	if httpClient := ClientOptionsInspector.HTTPClient(opts); httpClient != nil {
		if hijacker, ok := httpClient.Transport.(transportHijacker); ok {
			appHTTPClient.Transport = hijacker.Hijack(appHTTPClient.Transport)
			opts = append(opts, ably.WithHTTPClient(appHTTPClient))
		}
	}
	appOpts = MergeOptions(appOpts, opts)

	return appOpts
}

func (app *Sandbox) URL(paths ...string) string {
	return "https://" + app.Environment + "-rest.ably.io/" + path.Join(paths...)
}

// Source code for the same => https://github.com/ably/echoserver/blob/main/app.js
var CREATE_JWT_URL string = "https://echo.ably.io/createJWT"

// GetJwtAuthParams constructs the authentication parameters required for JWT creation.
// Required when authUrl is chosen as a mode of auth
//
// Parameters:
// - expiresIn: The duration until the JWT expires.
// - invalid: A boolean flag indicating whether to use an invalid key secret.
//
// Returns: A url.Values object containing the authentication parameters.
func (app *Sandbox) GetJwtAuthParams(expiresIn time.Duration, invalid bool) url.Values {
	key, secret := app.KeyParts()
	authParams := url.Values{}
	authParams.Add("environment", app.Environment)
	authParams.Add("returnType", "jwt")
	authParams.Add("keyName", key)
	if invalid {
		authParams.Add("keySecret", "invalid")
	} else {
		authParams.Add("keySecret", secret)
	}
	authParams.Add("expiresIn", fmt.Sprint(expiresIn.Seconds()))
	return authParams
}

// CreateJwt generates a JWT with the specified expiration time.
//
// Parameters:
// - expiresIn: The duration until the JWT expires.
// - invalid: A boolean flag indicating whether to use an invalid key secret.
//
// Returns:
// - A string containing the generated JWT.
// - An error if the JWT creation fails.
func (app *Sandbox) CreateJwt(expiresIn time.Duration, invalid bool) (string, error) {
	u, err := url.Parse(CREATE_JWT_URL)
	if err != nil {
		return "", err
	}
	u.RawQuery = app.GetJwtAuthParams(expiresIn, invalid).Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("client: could not create request: %s", err)
	}
	res, err := app.client.Do(req)
	if err != nil {
		res.Body.Close()
		return "", fmt.Errorf("client: error making http request: %s", err)
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("client: could not read response body: %s", err)
	}
	if res.StatusCode != 200 {
		return "", fmt.Errorf("non-success response received: %v:%s", res.StatusCode, resBody)
	}
	return string(resBody), nil
}

func NewHTTPClient() *http.Client {
	const timeout = time.Minute
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: timeout,
			}).Dial,
			TLSHandshakeTimeout: timeout,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: os.Getenv("HTTP_PROXY") != "",
			},
		},
	}
}

func NewHTTPClientNoKeepAlive() *http.Client {
	const timeout = time.Minute
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout: timeout,
			}).Dial,
			TLSHandshakeTimeout: timeout,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: os.Getenv("HTTP_PROXY") != "",
			},
		},
	}
}
