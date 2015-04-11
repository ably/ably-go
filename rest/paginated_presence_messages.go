package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/proto"
)

type PaginatedPresenceMessages struct {
	paginatedResource *proto.PaginatedResource
	Current           []*proto.PresenceMessage
}

func NewPaginatedPresenceMessages(client proto.ResourceReader, path string, params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	paginatedPresenceMessages := &PaginatedPresenceMessages{}
	paginatedPresenceMessages.Current = make([]*proto.PresenceMessage, 0)

	newPaginatedResource, err := proto.NewPaginatedResource(client, path, params, &paginatedPresenceMessages.Current)
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
