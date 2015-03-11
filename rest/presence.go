package rest

import "github.com/ably/ably-go/config"

type Presence struct {
	client  *Client
	channel *Channel
}

func (p *Presence) Get(params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	return p.paginateResult("/channels/"+p.channel.Name+"/presence", params)
}

func (p *Presence) History(params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	return p.paginateResult("/channels/"+p.channel.Name+"/presence/history", params)
}

func (p *Presence) paginateResult(path string, params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	return NewPaginatedPresenceMessages(p.client, path, params)
}
