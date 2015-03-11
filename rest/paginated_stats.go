package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
)

type PaginatedStats struct {
	paginatedResource *protocol.PaginatedResource
	Current           []*protocol.Stat
}

func NewPaginatedStats(client protocol.ResourceReader, path string, params *config.PaginateParams) (*PaginatedStats, error) {
	paginatedStats := &PaginatedStats{}
	paginatedStats.Current = make([]*protocol.Stat, 0)

	newPaginatedResource, err := protocol.NewPaginatedResource(client, path, params, &paginatedStats.Current)
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
