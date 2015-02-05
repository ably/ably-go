package rest

import (
	"fmt"
	"net/http"

	"github.com/ably/ably-go/protocol"
)

type Channel struct {
	Name string

	client *Client
}

func (c *Channel) Publish(name string, data interface{}) error {
	msg := &protocol.Message{Name: name, Data: data}
	res, err := c.client.Post("/channels/"+c.Name+"/messages", msg, nil)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("Expected status %d, got %d", http.StatusCreated, res.StatusCode)
	}
	return nil
}

func (c *Channel) History() ([]*protocol.Message, error) {
	msgs := []*protocol.Message{}
	_, err := c.client.Get("/channels/"+c.Name+"/history", &msgs)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}
