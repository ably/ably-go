//go:build !unit
// +build !unit

package ably_test

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/internal/ablytest"
)

func localPlaintextRealtimeOption(config *ablytest.Config) ably.ClientOption {
	allowedHosts := make(map[string]struct{})
	for _, host := range strings.Split(os.Getenv("ABLY_LOCAL_FALLBACK_HOSTS"), ",") {
		if host = strings.TrimSpace(host); host != "" {
			allowedHosts[host] = struct{}{}
		}
	}
	endpoint := net.JoinHostPort(config.LocalEndpoint, strconv.Itoa(config.LocalPort))

	ably.SetWebsocketURLTransform(func(u *url.URL) (*url.URL, error) {
		hostname := u.Hostname()
		ip := net.ParseIP(hostname)
		loopback := hostname == "localhost" || (ip != nil && ip.IsLoopback())
		if !loopback {
			if _, allowed := allowedHosts[hostname]; !allowed {
				return nil, fmt.Errorf("refusing plaintext realtime test transport for non-loopback host %q", hostname)
			}
		}

		localURL := *u
		localURL.Scheme = "ws"
		if !loopback {
			localURL.Host = endpoint
		}
		return &localURL, nil
	})
	return ably.WithDial(ably.DialWebsocket)
}

// TestMain tears down the shared sandbox app once after all tests in this
// package have run. The app itself is provisioned lazily on first use (see
// ablytest.NewSandbox), so there is no setup here; if no test provisions it,
// CloseSharedApp is a no-op.
func TestMain(m *testing.M) {
	if os.Getenv("ABLY_LOCAL_PLAINTEXT_REALTIME") == "1" {
		ablytest.LocalRealtimeOption = localPlaintextRealtimeOption
	}
	code := m.Run()
	ably.SetWebsocketURLTransform(nil)
	if err := ablytest.CloseSharedApp(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to tear down shared sandbox app: %v\n", err)
	}
	os.Exit(code)
}
