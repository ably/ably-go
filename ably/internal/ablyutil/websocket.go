package ablyutil

import (
	"errors"
	"net/url"

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

func (ws *WebsocketConn) Receive() (*proto.ProtocolMessage, error) {
	msg := &proto.ProtocolMessage{}
	err := ws.codec.Receive(ws.conn, &msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (ws *WebsocketConn) Close() error {
	return ws.conn.Close()
}

func DialWebsocket(proto string, u *url.URL) (*WebsocketConn, error) {
	ws := &WebsocketConn{}
	switch proto {
	case "application/json":
		ws.codec = websocket.JSON
	case "application/x-msgpack":
		ws.codec = msgpackCodec
	default:
		return nil, errors.New(`invalid protocol "` + proto + `"`)
	}
	conn, err := websocket.Dial(u.String(), "", "https://"+u.Host)
	if err != nil {
		return nil, err
	}
	ws.conn = conn
	return ws, nil
}

var msgpackCodec = websocket.Codec{
	Marshal: func(v interface{}) ([]byte, byte, error) {
		p, err := Marshal(v)
		return p, websocket.BinaryFrame, err
	},
	Unmarshal: func(p []byte, _ byte, v interface{}) error {
		return Unmarshal(p, v)
	},
}
