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

type PaginatedMessages struct {
	paginatedResource *protocol.PaginatedResource
	channel           *Channel

	Current []*protocol.Message
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

func (c *Channel) Publish(name string, data interface{}) error {
	msg := &protocol.Message{Name: name, Data: data}
	res, err := c.client.Post("/channels/"+c.Name+"/messages", msg, nil)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	return nil
}

func (c *Channel) History(params *config.PaginateParams) (*PaginatedMessages, error) {
	return c.paginatedGet("/channels/"+c.Name+"/history", params)
}

func (c *Channel) paginatedGet(path string, params *config.PaginateParams) (*PaginatedMessages, error) {
	msgs := []*protocol.Message{}

	builtPath, err := c.client.buildPaginatedPath(path, params)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Get(builtPath, &msgs)
	if err != nil {
		return nil, err
	}

	paginatedMessages := &PaginatedMessages{
		paginatedResource: &protocol.PaginatedResource{
			Response: resp,
			Path:     builtPath,
		},
		channel: c,
		Current: msgs,
	}

	return paginatedMessages, nil
}
