package ablyutil

import (
	"errors"
	"net"
	"net/url"
	"time"

	"github.com/ably/ably-go/ably/proto"

	"golang.org/x/net/websocket"
)

type WebsocketConn struct {
	conn  *websocket.Conn
	codec websocket.Codec
}

func (ws *WebsocketConn) Send(msg *proto.ProtocolMessage) error {
	return ws.codec.Send(ws.conn, msg)
}

func (ws *WebsocketConn) Receive(deadline time.Time) (*proto.ProtocolMessage, error) {
	msg := &proto.ProtocolMessage{}
	if !deadline.IsZero() {
		err := ws.conn.SetReadDeadline(deadline)
		if err != nil {
			return nil, err
		}
	}
	err := ws.codec.Receive(ws.conn, &msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (ws *WebsocketConn) Close() error {
	return ws.conn.Close()
}

func DialWebsocket(proto string, u *url.URL, timeout time.Duration) (*WebsocketConn, error) {
	ws := &WebsocketConn{}
	switch proto {
	case "application/json":
		ws.codec = websocket.JSON
	case "application/x-msgpack":
		ws.codec = msgpackCodec
	default:
		return nil, errors.New(`invalid protocol "` + proto + `"`)
	}
	// Starts a raw websocket connection with server
	conn, err := dialWebsocketTimeout(u.String(), "", "https://"+u.Host, timeout)
	if err != nil {
		return nil, err
	}
	ws.conn = conn
	return ws, nil
}

// dialWebsocketTimeout dials the websocket with a timeout.
func dialWebsocketTimeout(uri, protocol, origin string, timeout time.Duration) (*websocket.Conn, error) {
	config, err := websocket.NewConfig(uri, origin)
	if err != nil {
		return nil, err
	}
	if protocol != "" {
		config.Protocol = []string{protocol}
	}
	config.Dialer = &net.Dialer{
		Timeout: timeout,
	}
	return websocket.DialConfig(config)
}

var msgpackCodec = websocket.Codec{
	Marshal: func(v interface{}) ([]byte, byte, error) {
		p, err := MarshalMsgpack(v)
		return p, websocket.BinaryFrame, err
	},
	Unmarshal: func(p []byte, _ byte, v interface{}) error {
		return UnmarshalMsgpack(p, v)
	},
}
