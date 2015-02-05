package realtime

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/ably/ably-go"
	"github.com/ably/ably-go/protocol"
	"github.com/ably/ably-go/rest"
)

func NewClient(params ably.Params) *Client {
	c := &Client{
		Params:   params,
		Err:      make(chan error),
		rest:     rest.NewClient(params),
		channels: make(map[string]*Channel),
	}
	c.connCond = sync.NewCond(&c.connMtx)
	go c.connect()
	return c
}

type Client struct {
	ably.Params

	Err chan error

	rest *rest.Client

	conn     *Conn
	connCond *sync.Cond
	connMtx  sync.Mutex

	channels map[string]*Channel
	chanMtx  sync.RWMutex
}

func (c *Client) Close() {
	c.getConn().close()
}

func (c *Client) Channel(name string) *Channel {
	c.chanMtx.Lock()
	defer c.chanMtx.Unlock()

	if ch, ok := c.channels[name]; ok {
		return ch
	}

	ch := NewChannel(name, c)
	c.channels[name] = ch
	return ch
}

func (c *Client) getConn() *Conn {
	c.connMtx.Lock()
	defer c.connMtx.Unlock()
	if c.conn == nil {
		c.connCond.Wait()
	}
	return c.conn
}

func (c *Client) connect() {
	log.Println("requesting token")
	token, err := c.rest.RequestToken(60*60, &ably.Capability{"*": []string{"*"}})
	if err != nil {
		c.Err <- fmt.Errorf("Error fetching token: %s", err)
		return
	}

	c.connMtx.Lock()
	c.conn, err = Dial(c.RealtimeEndpoint + "?access_token=" + token.ID + "&binary=false&timestamp=" + strconv.Itoa(int(time.Now().Unix())))
	c.connCond.Broadcast()
	c.connMtx.Unlock()
	if err != nil {
		c.Err <- fmt.Errorf("Websocket dial error: %s", err)
		return
	}

	for {
		select {
		case msg := <-c.conn.Ch:
			c.Channel(msg.Channel).notify(msg)
		case err := <-c.conn.Err:
			c.Close()
			c.Err <- err
			return
		}
	}
}

func (c *Client) send(msg *protocol.ProtocolMessage) error {
	return c.getConn().send(msg)
}

func (c *Client) isActive() bool {
	return c.getConn().isActive()
}
