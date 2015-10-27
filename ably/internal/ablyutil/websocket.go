package ablyutil

import (
	"errors"
	"net/url"

	"github.com/ably/ably-go/Godeps/_workspace/src/golang.org/x/net/websocket"
	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
)

type WebsocketConn struct {
	conn  *websocket.Conn
	codec websocket.Codec
}

func (ws *WebsocketConn) Send(v interface{}) error {
	return ws.codec.Send(ws.conn, v)
}

func (ws *WebsocketConn) Receive(v interface{}) error {
	return ws.codec.Receive(ws.conn, v)
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
		p, err := msgpack.Marshal(v)
		return p, websocket.BinaryFrame, err
	},
	Unmarshal: func(p []byte, _ byte, v interface{}) error {
		return msgpack.Unmarshal(p, v)
	},
}
