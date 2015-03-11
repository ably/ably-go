package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
)

type PaginatedMessages struct {
	paginatedResource *protocol.PaginatedResource
	Current           []*protocol.Message
}

func NewPaginatedMessages(client protocol.ResourceReader, path string, params *config.PaginateParams) (*PaginatedMessages, error) {
	paginatedMessages := &PaginatedMessages{}
	paginatedMessages.Current = make([]*protocol.Message, 0)

	newPaginatedResource, err := protocol.NewPaginatedResource(client, path, params, &paginatedMessages.Current)
	if err != nil {
		return nil, err
	}

	paginatedMessages.paginatedResource = newPaginatedResource
	return paginatedMessages, nil
}

func (p *PaginatedMessages) NextPage() (*PaginatedMessages, error) {
	path, err := p.paginatedResource.NextPagePath()
	if err != nil {
		return nil, err
	}

	return NewPaginatedMessages(p.paginatedResource.ResourceReader, path, nil)
}
