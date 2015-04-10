package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
)

type Channel struct {
	Name     string
	Presence *Presence

	client *Client
}

func newChannel(name string, client *Client) *Channel {
	c := &Channel{
		Name:   name,
		client: client,
	}

	c.Presence = &Presence{
		client:  client,
		channel: c,
	}

	return c
}

func (c *Channel) Publish(name string, data string) error {
	messages := []*protocol.Message{
		{Name: name, Data: data, Encoding: "utf8"},
	}
	return c.PublishAll(messages)
}

// PublishAll sends multiple messages in the same http call.
// This is the more efficient way of transmitting a batch of messages
// using the Rest API.
func (c *Channel) PublishAll(messages []*protocol.Message) error {
	res, err := c.client.Post("/channels/"+c.Name+"/messages", messages, nil)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	return nil
}

func (c *Channel) History(params *config.PaginateParams) (*PaginatedMessages, error) {
	return c.paginateResults("/channels/"+c.Name+"/history", params)
}

func (p *Channel) paginateResults(path string, params *config.PaginateParams) (*PaginatedMessages, error) {
	return NewPaginatedMessages(p.client, path, params)
}
