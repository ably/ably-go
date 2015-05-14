package ably

import "github.com/ably/ably-go/ably/proto"

type RestChannel struct {
	Name     string
	Presence *RestPresence

	client *RestClient
}

func newRestChannel(name string, client *RestClient) *RestChannel {
	c := &RestChannel{
		Name:   name,
		client: client,
	}
	c.Presence = &RestPresence{
		client:  client,
		channel: c,
	}
	return c
}

func (c *RestChannel) Publish(name string, data string) error {
	messages := []*proto.Message{
		{Name: name, Data: data, Encoding: "utf8"},
	}
	return c.PublishAll(messages)
}

// PublishAll sends multiple messages in the same http call.
// This is the more efficient way of transmitting a batch of messages
// using the Rest API.
func (c *RestChannel) PublishAll(messages []*proto.Message) error {
	resp, err := c.client.Post("/channels/"+c.Name+"/messages", messages, nil)
	if err != nil {
		return err
	}
	if err = checkError(resp); err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// History gives the channel's message history according to the given parameters.
// The returned result can be inspected for the messages via the Messages()
// method.
func (c *RestChannel) History(params *PaginateParams) (*PaginatedResult, error) {
	path := "/channels/" + c.Name + "/history"
	return newPaginatedResult(msgType, path, params, query(c.client.Get))
}
