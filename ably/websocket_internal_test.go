//go:build !integration
// +build !integration

package ably

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ugorji/go/codec"
	"nhooyr.io/websocket"
)

var timeout = time.Second * 30

// handleWebsocketHandshake is a handler that can handle a websocket handshake and connection from a client.
func handleWebsocketHandshake(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		log.Printf("test server: error accepting websocket handshake: %+v\n", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
}

// handleWebsocketMessages is a handler that receives a protocol message over a websocket connection then
// responds by sending a protocol message back to the client including the message type and original message.
func handleWebsocketMessage(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		log.Printf("test server: error accepting websocket handshake: %+v\n", err)
	}
	defer conn.Close(websocket.StatusInternalError, "connection closed")

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	// Read message received from client.
	msgType, received, err := conn.Read(ctx)
	if err != nil {
		log.Printf("test server: error reading received message: %+v\n", err)
	}

	// Reply to message with the type of message received and the message string.
	reply := protocolMessage{
		Action:   actionMessage,
		Messages: []*Message{{Name: msgType.String(), Data: string(received)}},
	}

	var payloadType websocket.MessageType
	var payload []byte

	switch msgType.String() {
	case "MessageText":
		payloadType = websocket.MessageText

		payload, err = json.Marshal(reply)
		if err != nil {
			log.Printf("test server: error marshalling json: %+v\n", err)
		}

	case "MessageBinary":
		payloadType = websocket.MessageBinary

		var handle codec.MsgpackHandle
		var buf bytes.Buffer
		enc := codec.NewEncoder(&buf, &handle)
		if err := enc.Encode(reply); err != nil {
			log.Printf("test server: error encoding msg: %+v\n", err)
		}
		payload = buf.Bytes()
	}

	if err := conn.Write(ctx, payloadType, payload); err != nil {
		log.Println("test server: error sending message\n", err)
	}
}

func TestWebsocketDial(t *testing.T) {
	// Create test server with a handler that will accept a websocket handshake.
	ts := httptest.NewServer(http.HandlerFunc(handleWebsocketHandshake))
	defer ts.Close()

	// Convert http:// to ws:// and parse the test server url
	websocketUrl := fmt.Sprintf("ws%s", strings.TrimPrefix(ts.URL, "http"))
	testServerURL, _ := url.Parse(websocketUrl)

	tests := map[string]struct {
		dialProtocol  string
		expectedErr   error
		expectedProto proto
	}{
		"Can dial for protocol application/json": {
			dialProtocol:  "application/json",
			expectedErr:   nil,
			expectedProto: jsonProto,
		},
		"Can dial for protocol application/x-msgpack": {
			dialProtocol:  "application/x-msgpack",
			expectedErr:   nil,
			expectedProto: 1,
		},
		"Can handle an error when dialing for an invalid protocol": {
			dialProtocol: "aProtocol",
			expectedErr:  errors.New(`invalid protocol "aProtocol"`),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {

			result, err := dialWebsocket(test.dialProtocol, testServerURL, timeout)
			assert.Equal(t, test.expectedErr, err)

			if result != nil {
				assert.Equal(t, test.expectedProto, result.proto)
			}
		})
	}
}

func TestWebsocketSendAndReceive(t *testing.T) {
	// Create test server with a handler that can receive a message.
	ts := httptest.NewServer(http.HandlerFunc(handleWebsocketMessage))
	defer ts.Close()

	// Convert http:// to ws:// and parse the test server url
	websocketUrl := fmt.Sprintf("ws%s", strings.TrimPrefix(ts.URL, "http"))
	testServerURL, _ := url.Parse(websocketUrl)

	tests := map[string]struct {
		dialProtocol        string
		expectedMessageType string
		expectedPayloadType byte
		expectedResult      *websocketConn
		expectedErr         error
	}{
		"Can send and receive a message using protocol application/json": {
			dialProtocol:        "application/json",
			expectedMessageType: "MessageText",
			expectedErr:         nil,
		},
		"Can send and receive a message using protocol application/x-msgpack": {
			dialProtocol:        "application/x-msgpack",
			expectedMessageType: "MessageBinary",
			expectedErr:         nil,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {

			ws, err := dialWebsocket(test.dialProtocol, testServerURL, timeout)
			assert.NoError(t, err)
			msg := protocolMessage{
				Messages: []*Message{{Name: "temperature", Data: "22.7"}},
			}

			ws.Send(&msg)
			result, err := ws.Receive(time.Now().Add(timeout))

			assert.Equal(t, test.expectedErr, err)
			assert.Equal(t, test.expectedMessageType, result.Messages[0].Name)
			assert.Equal(t, 1, len(result.Messages))
			assert.Equal(t, actionMessage, result.Action)
			assert.Contains(t, result.Messages[0].Data, "temperature")
			assert.Contains(t, result.Messages[0].Data, "22.7")
		})
	}
}
