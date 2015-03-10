package rest

import (
	"bytes"
	"encoding/json"

	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/protocol"
)

type PaginatedPresenceMessages struct {
	paginatedResource *protocol.PaginatedResource
	Current           []*protocol.PresenceMessage
}

func (p *PaginatedPresenceMessages) GetPage(path string, params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	newPaginatedResource, err := p.paginatedResource.PaginatedGet(path, params)
	if err != nil {
		return nil, err
	}

	paginatedPresenceMessages := &PaginatedPresenceMessages{
		paginatedResource: newPaginatedResource,
		Current:           make([]*protocol.PresenceMessage, 0),
	}
	reader := bytes.NewReader(paginatedPresenceMessages.paginatedResource.Body)
	err = json.NewDecoder(reader).Decode(&paginatedPresenceMessages.Current)

	return paginatedPresenceMessages, err
}

func (p *PaginatedPresenceMessages) NextPage() (*PaginatedPresenceMessages, error) {
	path, err := p.paginatedResource.NextPage()
	if err != nil {
		return nil, err
	}

	return p.GetPage(path, nil)
}
