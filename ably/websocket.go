package ably

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"nhooyr.io/websocket"
)

type proto int

const (
	jsonProto proto = iota
	msgpackProto
)

type websocketConn struct {
	conn  *websocket.Conn
	proto proto
}

type websocketErr struct {
	err  error
	resp *http.Response
}

// websocketErr implements the builtin error interface.
func (e *websocketErr) Error() string {
	return e.err.Error()
}

// Unwrap implements the implicit interface that errors.Unwrap understands.
func (e *websocketErr) Unwrap() error {
	return e.err
}

func (ws *websocketConn) Send(msg *protocolMessage) error {
	switch ws.proto {
	case jsonProto:
		p, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		return ws.conn.Write(context.Background(), websocket.MessageText, p)
	case msgpackProto:
		p, err := ablyutil.MarshalMsgpack(msg)
		if err != nil {
			return err
		}
		return ws.conn.Write(context.Background(), websocket.MessageBinary, p)
	}
	return nil
}

func (ws *websocketConn) Receive(deadline time.Time) (*protocolMessage, error) {
	msg := &protocolMessage{}
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if deadline.IsZero() {
		ctx = context.Background()
	} else {
		ctx, cancel = context.WithDeadline(context.Background(), deadline)
		defer cancel()
	}
	_, data, err := ws.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	switch ws.proto {
	case jsonProto:
		err := json.Unmarshal(data, msg)
		if err != nil {
			return nil, err
		}
	case msgpackProto:
		err := ablyutil.UnmarshalMsgpack(data, msg)
		if err != nil {
			return nil, err
		}
	}
	return msg, nil
}

func (ws *websocketConn) Close() error {
	return ws.conn.Close(websocket.StatusNormalClosure, "")
}

func dialWebsocket(proto string, u *url.URL, timeout time.Duration, agents map[string]string) (*websocketConn, error) {
	ws := &websocketConn{}
	switch proto {
	case "application/json":
		ws.proto = jsonProto
	case "application/x-msgpack":
		ws.proto = msgpackProto
	default:
		return nil, errors.New(`invalid protocol "` + proto + `"`)
	}
	// Starts a raw websocket connection with server
	conn, resp, err := dialWebsocketTimeout(u.String(), "https://"+u.Host, timeout, agents)
	if err != nil {
		return nil, &websocketErr{err: err, resp: resp}
	}
	ws.conn = conn
	return ws, nil
}

// dialWebsocketTimeout dials the websocket with a timeout.
func dialWebsocketTimeout(uri, origin string, timeout time.Duration, agents map[string]string) (*websocket.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var ops websocket.DialOptions
	ops.HTTPHeader = make(http.Header)
	ops.HTTPHeader.Add(ablyAgentHeader, ablyAgentIdentifier(agents))

	c, resp, err := websocket.Dial(ctx, uri, &ops)

	if err != nil {
		return nil, resp, err
	}

	return c, resp, nil
}

func unwrapConn(c conn) conn {
	u, ok := c.(interface {
		Unwrap() conn
	})
	if !ok {
		return c
	}
	return unwrapConn(u.Unwrap())
}

func extractHttpResponseFromError(err error) *http.Response {
	wsErr, ok := err.(*websocketErr)
	if ok {
		return wsErr.resp
	}
	return nil
}

func setConnectionReadLimit(c conn, readLimit int64) error {
	unwrappedConn := unwrapConn(c)
	websocketConn, ok := unwrappedConn.(*websocketConn)
	if !ok {
		return errors.New("cannot set readlimit for connection, connection does not use nhooyr.io/websocket")
	}
	websocketConn.conn.SetReadLimit(readLimit)
	return nil
}
