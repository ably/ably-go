package ably

import (
	"strings"

	"github.com/ably/ably-go/ably/proto"
)

// based on HttpUtils::encodeURIComponent from ably-java library
var encodeURIComponent = strings.NewReplacer(
	" ", "%20",
	"!", "%21",
	"'", "%27",
	"(", "%28",
	")", "%29",
	"+", "%2B",
	":", "%3A",
	"~", "%7E",
	"/", "%2F",
	"?", "%3F",
	"#", "%23",
)

type RestChannel struct {
	Name     string
	Presence *RestPresence

	client  *RestClient
	uriName string
	options *proto.ChannelOptions
}

func newRestChannel(name string, client *RestClient) *RestChannel {
	c := &RestChannel{
		Name:    name,
		client:  client,
		uriName: encodeURIComponent.Replace(name),
	}
	c.Presence = &RestPresence{
		client:  client,
		channel: c,
	}
	return c
}

func (c *RestChannel) Publish(name string, data string) error {
	messages := []*proto.Message{
		{Name: name, Data: data, Encoding: proto.UTF8},
	}
	return c.PublishAll(messages)
}

// PublishAll sends multiple messages in the same http call.
// This is the more efficient way of transmitting a batch of messages
// using the Rest API.
func (c *RestChannel) PublishAll(messages []*proto.Message) error {
	res, err := c.client.post("/channels/"+c.uriName+"/messages", messages, nil)
	if err != nil {
		return err
	}
	return res.Body.Close()
}

// History gives the channel's message history according to the given parameters.
// The returned result can be inspected for the messages via the Messages()
// method.
func (c *RestChannel) History(params *PaginateParams) (*PaginatedResult, error) {
	path := "/channels/" + c.uriName + "/history"
	return newPaginatedResult(msgType, path, params, query(c.client.get), c.logger())
}

func (c *RestChannel) logger() *LoggerOptions {
	return c.client.logger()
}
