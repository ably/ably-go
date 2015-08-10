package testutil

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/ably/ably-go/ably"
)

type Key struct {
	ID         string          `json:"id,omitempty"`
	ScopeID    string          `json:"scopeId,omitempty"`
	Status     int             `json:"status,omitempty"`
	Type       int             `json:"type,omitempty"`
	Value      string          `json:"value,omitempty"`
	Created    int             `json:"created,omitempty"`
	Modified   int             `json:"modified,omitempty"`
	Capability ably.Capability `json:"capability,omitempty"`
	Expires    int             `json:"expired,omitempty"`
	Privileged bool            `json:"privileged,omitempty"`
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

func Provision(cfg *Config) *Sandbox {
	app, err := NewSandbox(cfg)
	if err != nil {
		panic(err)
	}
	return app
}

func ProvisionRealtime(cfg *Config, opts *ably.ClientOptions) (*Sandbox, *ably.RealtimeClient) {
	app := Provision(cfg)
	client, err := ably.NewRealtimeClient(app.Options(opts))
	if err != nil {
		panic(nonil(err, app.Close()))
	}
	return app, client
}

func NewSandbox(config *Config) (*Sandbox, error) {
	app := &Sandbox{
		Config:      config,
		Environment: nonempty(os.Getenv("ABLY_ENV"), "sandbox"),
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
		return nil, errors.New(http.StatusText(resp.StatusCode))
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

func (app *Sandbox) KeyParts() (name, secret string) {
	return app.Config.AppID + "." + app.Config.Keys[0].ID, app.Config.Keys[0].Value
}

func (app *Sandbox) Key() string {
	name, secret := app.KeyParts()
	return name + ":" + secret
}

func (app *Sandbox) Options(extra *ably.ClientOptions) *ably.ClientOptions {
	opts := &ably.ClientOptions{
		Environment: app.Environment,
		HTTPClient:  app.client,
		Key:         app.Key(),
	}
	if extra != nil {
		// Overwrite any non-zero field from ClientOptions passed as argument.
		vorig := reflect.ValueOf(opts).Elem()
		vextr := reflect.ValueOf(extra).Elem()
		for i := 0; i < vextr.NumField(); i++ {
			field := vextr.Field(i)
			typ := reflect.TypeOf(field.Interface())
			// Ignore non-comparable fields.
			isfunc := typ.Kind() == reflect.Func
			if isfunc && !field.IsNil() {
				vorig.Field(i).Set(field)
			}
			if !isfunc && field.Interface() != reflect.Zero(typ).Interface() {
				vorig.Field(i).Set(field)
			}
		}
	}
	return opts
}

func (app *Sandbox) URL(paths ...string) string {
	return "https://" + app.Environment + "-rest.ably.io/" + path.Join(paths...)
}

func nonil(err ...error) error {
	for _, err := range err {
		if err != nil {
			return err
		}
	}
	return nil
}

func nonempty(s ...string) string {
	for _, s := range s {
		if s != "" {
			return s
		}
	}
	return ""
}

func NewHTTPClient() *http.Client {
	const timeout = 10 * time.Second
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
