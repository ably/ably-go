package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
)

type PaginatedPresenceMessages struct {
	paginatedResource *protocol.PaginatedResource
	Current           []*protocol.PresenceMessage
}

func NewPaginatedPresenceMessages(client protocol.ResourceReader, path string, params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	paginatedPresenceMessages := &PaginatedPresenceMessages{}
	paginatedPresenceMessages.Current = make([]*protocol.PresenceMessage, 0)

	newPaginatedResource, err := protocol.NewPaginatedResource(client, path, params, &paginatedPresenceMessages.Current)
	if err != nil {
		return nil, err
	}

	paginatedPresenceMessages.paginatedResource = newPaginatedResource
	return paginatedPresenceMessages, nil
}

func (p *PaginatedPresenceMessages) NextPage() (*PaginatedPresenceMessages, error) {
	path, err := p.paginatedResource.NextPagePath()
	if err != nil {
		return nil, err
	}

	return NewPaginatedPresenceMessages(p.paginatedResource.ResourceReader, path, nil)
}
