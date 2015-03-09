package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
)

type Presence struct {
	client  *Client
	channel *Channel
}

type PaginatedPresenceMessages struct {
	paginatedResource *protocol.PaginatedResource
	presence          *Presence

	Current []*protocol.PresenceMessage
}

func (pm *PaginatedPresenceMessages) NextPage() (*PaginatedPresenceMessages, error) {
	path, err := pm.paginatedResource.NextPage()
	if err != nil {
		return nil, err
	}

	return pm.presence.clientGet(path, nil)
}

func (p *Presence) Get(params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	return p.clientGet("/channels/"+p.channel.Name+"/presence", params)
}

func (p *Presence) History(params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	return p.clientGet("/channels/"+p.channel.Name+"/presence/history", params)
}

func (p *Presence) clientGet(url string, params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	msgs := []*protocol.PresenceMessage{}

	builtURL, err := p.client.buildPaginatedURL(url, params)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Get(builtURL, &msgs)
	if err != nil {
		return nil, err
	}

	paginatedMessages := &PaginatedPresenceMessages{
		paginatedResource: &protocol.PaginatedResource{
			Response: resp,
			Path:     builtURL,
		},
		presence: p,
		Current:  msgs,
	}

	return paginatedMessages, nil
}
