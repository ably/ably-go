package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
)

type Presence struct {
	client  *Client
	channel *Channel
}

func (p *Presence) Get(params *config.PaginateParams) ([]*protocol.PresenceMessage, error) {
	return p.clientGet("/channels/"+p.channel.Name+"/presence", params)
}

func (p *Presence) History(params *config.PaginateParams) ([]*protocol.PresenceMessage, error) {
	return p.clientGet("/channels/"+p.channel.Name+"/presence/history", params)
}

func (p *Presence) clientGet(url string, params *config.PaginateParams) ([]*protocol.PresenceMessage, error) {
	msgs := []*protocol.PresenceMessage{}

	builtURL, err := p.client.buildPaginatedURL(url, params)
	if err != nil {
		return nil, err
	}

	_, err = p.client.Get(builtURL, &msgs)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}
