package ably

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/ably/ably-go/ably/internal/ablyutil"
	"golang.org/x/net/websocket"
)

type websocketConn struct {
	conn  *websocket.Conn
	codec websocket.Codec
}

func (ws *websocketConn) Send(msg *protocolMessage) error {
	return ws.codec.Send(ws.conn, msg)
}

func (ws *websocketConn) Receive(deadline time.Time) (*protocolMessage, error) {
	msg := &protocolMessage{}
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

func (ws *websocketConn) Close() error {
	return ws.conn.Close()
}

func dialWebsocket(proto string, u *url.URL, timeout time.Duration) (*websocketConn, error) {
	ws := &websocketConn{}
	switch proto {
	case "application/json":
		ws.codec = websocket.JSON
	case "application/x-msgpack":
		ws.codec = msgpackCodec
	default:
		return nil, errors.New(`invalid protocol "` + proto + `"`)
	}
	// Starts a raw websocket connection with server
	fmt.Printf("1st arg u.String is:%s\n", u.String())
	fmt.Println(`2nd arg is ""`)
	fmt.Printf("3rd arg is origin: %s\n",  "https://"+u.Host)
	fmt.Printf("4th arg is timeout: %v\n", timeout)

	conn, err := dialWebsocketTimeout(u.String(), "", "https://"+u.Host, timeout)
	if err != nil {
		return nil, err
	}
	ws.conn = conn
	return ws, nil
}

// dialWebsocketTimeout dials the websocket with a timeout.
func dialWebsocketTimeout(uri, protocol, origin string, timeout time.Duration) (*websocket.Conn, error) {
	fmt.Println("about to create new websocket config")
	config, err := websocket.NewConfig(uri, origin)
	if err != nil {
		return nil, err
	}
	fmt.Printf("config is: %+v\n", config)
	
	config.Header.Set(ablyAgentHeader, ablyAgentIdentifier)
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
		p, err := ablyutil.MarshalMsgpack(v)
		return p, websocket.BinaryFrame, err
	},
	Unmarshal: func(p []byte, _ byte, v interface{}) error {
		return ablyutil.UnmarshalMsgpack(p, v)
	},
}
