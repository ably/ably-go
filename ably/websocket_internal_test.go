//go:build !integration
// +build !integration

package ably

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"nhooyr.io/websocket"
	"strings"
	"testing"
	"time"
)

var timeout = time.Second * 30

// handleWebsocketHandshake is a test server that can handle a websocket connection from a client.
func handleWebsocketHandshake(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		log.Printf("test server: error accepting websocket handshake: %+v\n", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
}

func TestWebsocketDial(t *testing.T) {
	// Create test server with a handler that will accept a websocket handshake.
	ts := httptest.NewServer(http.HandlerFunc(handleWebsocketHandshake))
	defer ts.Close()

	// Convert http:// to ws:// and parse the test server url
	websocketUrl := fmt.Sprintf("ws%s", strings.TrimPrefix(ts.URL, "http"))
	testServerURL, _ := url.Parse(websocketUrl)

	// Calculate the expected Ably-Agent header used by tests.
	expectedAgentHeader := AblySDKIdentifier + " " + GoRuntimeIdentifier + " " + GoOSIdentifier()

	tests := map[string]struct {
		dialProtocol         string
		expectedPayloadType  byte
		expectedConnProtocol []string
		expectedResult       *websocketConn
		expectedErr          error
	}{
		"Can dial for protocol application/json": {
			dialProtocol:         "application/json",
			expectedPayloadType:  uint8(1),
			expectedConnProtocol: []string(nil),
			expectedResult:       &websocketConn{},
			expectedErr:          nil,
		},
		"Can dial for protocol application/x-msgpack": {
			dialProtocol:         "application/x-msgpack",
			expectedPayloadType:  uint8(1),
			expectedConnProtocol: []string(nil),
			expectedResult:       &websocketConn{},
			expectedErr:          nil,
		},
		"Can handle an error when dialing for an invalid protocol": {
			dialProtocol:   "aProtocol",
			expectedResult: nil,
			expectedErr:    errors.New(`invalid protocol "aProtocol"`),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {

			result, err := dialWebsocket(test.dialProtocol, testServerURL, timeout)
			assert.Equal(t, test.expectedErr, err)

			if result != nil {
				assert.NotNil(t, result.codec)
				assert.Equal(t, expectedAgentHeader, result.conn.Config().Header.Get("Ably-Agent"))
				assert.Equal(t, test.expectedPayloadType, result.conn.PayloadType)
				assert.Equal(t, test.expectedConnProtocol, result.conn.Config().Protocol)
				assert.True(t, result.conn.IsClientConn())
			}
		})
	}
}
