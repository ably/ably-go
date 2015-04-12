package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/proto"
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
	messages := []*proto.Message{
		{Name: name, Data: data, Encoding: "utf8"},
	}
	return c.PublishAll(messages)
}

// PublishAll sends multiple messages in the same http call.
// This is the more efficient way of transmitting a batch of messages
// using the Rest API.
func (c *Channel) PublishAll(messages []*proto.Message) error {
	res, err := c.client.Post("/channels/"+c.Name+"/messages", messages, nil)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	return nil
}

// History gives the channel's message history according to the given parameters.
// The returned resource can be inspected for the messages via the Messages()
// method.
func (c *Channel) History(params *config.PaginateParams) (*proto.PaginatedResource, error) {
	path := "/channels/" + c.Name + "/history"
	return proto.NewPaginatedResource(msgType, path, params, query(c.client.Get))
}
