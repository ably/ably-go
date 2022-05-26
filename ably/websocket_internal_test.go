//go:build !integration
// +build !integration

package ably

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	//	"golang.org/x/net/websocket"
)

func echo(w http.ResponseWriter, r *http.Request) {

	//code snippet
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(body))

	// c, err := upgrader.Upgrade(w, r, nil)
	// if err != nil {
	//     return
	// }
	// defer c.Close()
	// for {
	//     mt, message, err := c.ReadMessage()
	//     if err != nil {
	//         break
	//     }
	//     err = c.WriteMessage(mt, message)
	//     if err != nil {
	//         break
	//     }
	// }
}

func TestWebsocketSend(t *testing.T) {
	// // Create test server with the echo handler.
	server := httptest.NewServer(http.HandlerFunc(echo))
	defer server.Close()

	// Convert http://127.0.0.1 to wss://127.0.0.
	wssURL := "ws" + strings.TrimPrefix(server.URL, "http")
	u, err := url.Parse(wssURL)
	if err != nil {
		fmt.Println(err)
	}

	timeout := time.Second * 60
	// // Connect to the server

	conn, err := dialWebsocket("application/json", u, timeout)
	if err != nil {
		fmt.Println("dial error")
		fmt.Println(err)
	}

	fmt.Println(conn)

	//   ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	// if err != nil {
	//     t.Fatalf("%v", err)
	// }
	// defer ws.Close()

	// // Send message to server, read response and check to see if it's what we expect.
	// for i := 0; i < 10; i++ {
	//     if err := ws.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
	//         t.Fatalf("%v", err)
	//     }
	//     _, p, err := ws.ReadMessage()
	//     if err != nil {
	//         t.Fatalf("%v", err)
	//     }
	//     if string(p) != "hello" {
	//         t.Fatalf("bad message")
	//     }
	// }
}
