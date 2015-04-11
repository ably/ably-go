package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/proto"
)

type PaginatedStats struct {
	paginatedResource *proto.PaginatedResource
	Current           []*proto.Stat
}

func NewPaginatedStats(client proto.ResourceReader, path string, params *config.PaginateParams) (*PaginatedStats, error) {
	paginatedStats := &PaginatedStats{}
	paginatedStats.Current = make([]*proto.Stat, 0)

	newPaginatedResource, err := proto.NewPaginatedResource(client, path, params, &paginatedStats.Current)
	if err != nil {
		return nil, err
	}

	paginatedStats.paginatedResource = newPaginatedResource
	return paginatedStats, nil
}

func (p *PaginatedStats) NextPage() (*PaginatedStats, error) {
	path, err := p.paginatedResource.NextPagePath()
	if err != nil {
		return nil, err
	}

	return NewPaginatedStats(p.paginatedResource.ResourceReader, path, nil)
}
