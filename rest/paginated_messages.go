package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/proto"
)

type PaginatedMessages struct {
	paginatedResource *proto.PaginatedResource
	Current           []*proto.Message
}

func NewPaginatedMessages(client proto.ResourceReader, path string, params *config.PaginateParams) (*PaginatedMessages, error) {
	paginatedMessages := &PaginatedMessages{}
	paginatedMessages.Current = make([]*proto.Message, 0)

	newPaginatedResource, err := proto.NewPaginatedResource(client, path, params, &paginatedMessages.Current)
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
