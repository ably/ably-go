package ablytest

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ably/ably-go/ably"
)

// Key is a single API key as returned in the /apps response.
type Key struct {
	ID         string `json:"id,omitempty"`
	Value      string `json:"value,omitempty"`
	Capability string `json:"capability,omitempty"`
}

// Config holds the fields of the /apps response that the test helpers need: the
// app ID and the provisioned API keys. The request body is no longer built from
// this struct — it is the static appspec JSON (see loadAppSetup).
type Config struct {
	AppID string `json:"appId,omitempty"`
	Keys  []Key  `json:"keys"`
}

// Presence describes a presence fixture member provisioned on the
// persisted:presence_fixtures channel by the appspec. It is decoded directly
// from the appspec JSON, so it is the single source of truth for what the
// presence tests expect the channel to return.
type Presence struct {
	ClientID string `json:"clientId"`
	Data     string `json:"data"`
	Encoding string `json:"encoding,omitempty"`
}

// PresenceFixtures returns the presence members provisioned on the
// persisted:presence_fixtures channel, read from the appspec JSON. client_encoded
// is the cipher-encrypted form of client_decoded's data; tests that read it back
// must configure the channel cipher (see PresenceFixturesCipher) to decode it.
func PresenceFixtures() []Presence {
	for _, ch := range loadAppSetup().channels() {
		if ch.Name == presenceFixturesChannel {
			return ch.Presence
		}
	}
	panic(fmt.Sprintf("appspec has no %q channel", presenceFixturesChannel))
}

const presenceFixturesChannel = "persisted:presence_fixtures"

// appSetup mirrors the structure of the shared appspec JSON
// (common/test-resources/test-app-setup.json). PostApps is kept as raw bytes so
// the /apps request body can be forwarded to the server verbatim; the parts the
// helpers need to read back (presence fixtures) are decoded separately from
// those same bytes by the typed accessors below.
type appSetup struct {
	PostApps json.RawMessage `json:"post_apps"`
	Cipher   struct {
		Algorithm string `json:"algorithm"`
		Mode      string `json:"mode"`
		KeyLength int    `json:"keylength"`
		Key       string `json:"key"`
		IV        string `json:"iv"`
	} `json:"cipher"`
}

type appSetupChannel struct {
	Name     string     `json:"name"`
	Presence []Presence `json:"presence"`
}

// channels decodes the channel fixtures from the post_apps body.
func (s appSetup) channels() []appSetupChannel {
	var body struct {
		Channels []appSetupChannel `json:"channels"`
	}
	if err := json.Unmarshal(s.PostApps, &body); err != nil {
		panic(err)
	}
	return body.Channels
}

var loadAppSetup = sync.OnceValue(func() appSetup {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic(errors.New("could not determine ablytest source location"))
	}
	p := filepath.Join(filepath.Dir(thisFile), "..", "common", "test-resources", "test-app-setup.json")
	data, err := os.ReadFile(p)
	if err != nil {
		panic(fmt.Errorf("reading appspec %q (is the ably-common submodule checked out?): %w", p, err))
	}
	var setup appSetup
	if err := json.Unmarshal(data, &setup); err != nil {
		panic(err)
	}
	return setup
})

// PresenceFixturesCipher returns the cipher params used to encrypt the
// client_encoded presence fixture, so tests can decode it on read.
func PresenceFixturesCipher() ably.CipherParams {
	key, err := base64.StdEncoding.DecodeString(loadAppSetup().Cipher.Key)
	if err != nil {
		panic(err)
	}
	return ably.Crypto.GetDefaultParams(ably.CipherParams{
		Algorithm: ably.CipherAES,
		Key:       key,
	})
}

type Sandbox struct {
	Config   *Config
	Endpoint string
	client   *http.Client
}

func NewRealtime(opts ...ably.ClientOption) (*Sandbox, *ably.Realtime) {
	app := MustSandbox()
	client, err := ably.NewRealtime(app.Options(opts...)...)
	if err != nil {
		panic(nonil(err, app.Close()))
	}
	return app, client
}

func NewREST(opts ...ably.ClientOption) (*Sandbox, *ably.REST) {
	app := MustSandbox()
	client, err := ably.NewREST(app.Options(opts...)...)
	if err != nil {
		panic(nonil(err, app.Close()))
	}
	return app, client
}

// MustSandbox returns the shared sandbox app (see NewSandbox) and panics if
// provisioning fails.
func MustSandbox() *Sandbox {
	app, err := NewSandbox()
	if err != nil {
		panic(err)
	}
	return app
}

// NewSandbox returns the process-wide shared sandbox app, provisioning it on
// first call. Every test uses this single app, provisioned from the shared
// appspec; isolation between tests is by channel name rather than by app. Tests
// that call Close on it are no-ops — the shared app is torn down once via
// CloseSharedApp (see TestMain).
func NewSandbox() (*Sandbox, error) {
	return sharedApp()
}

var sharedApp = sync.OnceValues(func() (*Sandbox, error) {
	return provisionSandbox(Endpoint)
})

// CloseSharedApp deletes the shared app if it was provisioned. It is intended to
// be called once from TestMain after all tests have run.
func CloseSharedApp() error {
	app, err := sharedApp()
	if err != nil {
		return nil // never provisioned successfully; nothing to delete
	}
	return app.delete()
}

func provisionSandbox(endpoint string) (*Sandbox, error) {
	app := &Sandbox{
		Config:   &Config{},
		Endpoint: endpoint,
		client:   NewHTTPClient(),
	}
	p := []byte(loadAppSetup().PostApps)

	const RetryCount = 4
	retryInterval := time.Second
	for requestAttempt := 0; requestAttempt < RetryCount; requestAttempt++ {
		req, err := http.NewRequest("POST", app.URL("apps"), bytes.NewReader(p))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set(ably.AblyAgentHeaderName, ably.AgentIdentifier(nil))
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

// Close is a no-op. The single shared app is owned by the test run, not by
// individual tests, so the many per-test Close calls must not delete it;
// teardown happens once via CloseSharedApp. Close is kept because tests call it
// (often via defer) on the value returned by NewSandbox/NewREST/NewRealtime.
func (app *Sandbox) Close() error {
	return nil
}

func (app *Sandbox) delete() error {
	req, err := http.NewRequest("DELETE", app.URL("apps", app.Config.AppID), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(app.KeyParts())
	req.Header.Set(ably.AblyAgentHeaderName, ably.AgentIdentifier(nil))
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

// wildcardCapability is the all-resources, all-operations capability. The shared
// appspec provisions several keys with differing capabilities; the tests expect
// a single all-powerful key, so KeyParts selects the one carrying this
// capability (in particular [*]* is required for qualified/derived channels,
// which the default capability does not grant).
const wildcardCapability = `{"[*]*":["*"]}`

func (app *Sandbox) KeyParts() (name, secret string) {
	key := app.wildcardKey()
	return app.Config.AppID + "." + key.ID, key.Value
}

func (app *Sandbox) wildcardKey() Key {
	for _, k := range app.Config.Keys {
		if k.Capability == wildcardCapability {
			return k
		}
	}
	// Fall back to the first key if no wildcard key is present, so behaviour is
	// well-defined even if the appspec changes.
	return app.Config.Keys[0]
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
		ably.WithEndpoint(app.Endpoint),
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
	if strings.HasPrefix(app.Endpoint, "nonprod:") {
		namespace := strings.TrimPrefix(app.Endpoint, "nonprod:")
		return fmt.Sprintf("https://%s.realtime.ably-nonprod.net/%s", namespace, path.Join(paths...))
	}

	return fmt.Sprintf("https://%s.realtime.ably.net/%s", app.Endpoint, path.Join(paths...))
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
	authParams.Add("endpoint", app.Endpoint)
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
