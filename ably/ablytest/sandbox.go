package ablytest

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
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
			{},
		},
		Namespaces: []Namespace{
			{ID: "persisted", Persisted: true},
		},
		Channels: []Channel{
			{
				Name: "persisted:presence_fixtures",
				Presence: []Presence{
					{ClientID: "client_bool", Data: "true"},
					{ClientID: "client_int", Data: "true"},
					{ClientID: "client_string", Data: "true"},
					{ClientID: "client_json", Data: `{"test": "This is a JSONObject clientData payload"}`},
				},
			},
		},
	}
}

type Sandbox struct {
	Config      *Config
	Environment string

	client *http.Client
}

func NewRealtime(opts ...ably.ClientOptions) (*Sandbox, *ably.Realtime) {
	app := MustSandbox(nil)
	client, err := ably.NewRealtime(app.Options(opts...))
	if err != nil {
		panic(nonil(err, app.Close()))
	}
	return app, client
}

func NewREST(opts ...ably.ClientOptions) (*Sandbox, *ably.REST) {
	app := MustSandbox(nil)
	client, err := ably.NewREST(app.Options(opts...))
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
	return NewSandboxWIthEnv(config, Environment)
}

func NewSandboxWIthEnv(config *Config, env string) (*Sandbox, error) {
	app := &Sandbox{
		Config:      config,
		Environment: env,
		client:      NewHTTPClient(),
	}
	if app.Config == nil {
		app.Config = DefaultConfig()
	}
	p, err := json.Marshal(app.Config)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", app.URL("apps"), bytes.NewReader(p))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := app.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		err := errors.New(http.StatusText(resp.StatusCode))
		if p, e := ioutil.ReadAll(resp.Body); e == nil && len(p) != 0 {
			err = fmt.Errorf("request error: %s (%q)", err, p)
		}
		return nil, err
	}
	if err := json.NewDecoder(resp.Body).Decode(app.Config); err != nil {
		return nil, err
	}
	return app, nil
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

func (app *Sandbox) NewRealtime(opts ...ably.ClientOptions) *ably.Realtime {
	client, err := ably.NewRealtime(app.Options(opts...))
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

func (app *Sandbox) Options(opts ...ably.ClientOptions) ably.ClientOptions {
	type transportHijacker interface {
		Hijack(http.RoundTripper) http.RoundTripper
	}
	appHTTPClient := NewHTTPClient()
	appOpts := ably.NewClientOptions(app.Key()).
		Environment(app.Environment).
		UseBinaryProtocol(!NoBinaryProtocol).
		LogHandler(DefaultLogger.GetLogger()).
		LogLevel(DefaultLogger.Level).
		HTTPClient(appHTTPClient)

	// If opts want to record round trips inject the recording transport
	// via TransportHijacker interface.
	opt := MergeOptions(opts...)
	if httpClient := ClientOptionsInspector.HTTPClient(opt); httpClient != nil {
		if hijacker, ok := httpClient.Transport.(transportHijacker); ok {
			appHTTPClient.Transport = hijacker.Hijack(appHTTPClient.Transport)
			opt = opt.HTTPClient(appHTTPClient)
		}
	}
	appOpts = MergeOptions(appOpts, opt)

	return appOpts
}

func (app *Sandbox) URL(paths ...string) string {
	return "https://" + app.Environment + "-rest.ably.io/" + path.Join(paths...)
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
