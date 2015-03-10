package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
)

type Presence struct {
	client  *Client
	channel *Channel
}

func (p *Presence) Get(params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	return p.paginatedGet("/channels/"+p.channel.Name+"/presence", params)
}

func (p *Presence) History(params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	return p.paginatedGet("/channels/"+p.channel.Name+"/presence/history", params)
}

func (p *Presence) paginatedGet(path string, params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	paginatedPresenceMessages := PaginatedPresenceMessages{
		paginatedResource: &protocol.PaginatedResource{
			ResourceReader: p.client,
		},
	}
	return paginatedPresenceMessages.GetPage(path, params)
}
