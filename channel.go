package ably

import (
	"fmt"
	"net/http"
)

type Channel struct {
	Name string

	client *Client
}

func (c *Channel) Publish(msg *Message) error {
	res, err := c.client.Post("/channels/"+c.Name+"/messages", msg, nil)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("Expected status %d, got %d", http.StatusCreated, res.StatusCode)
	}
	return nil
}
