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

func (p *PaginatedPresenceMessages) decode() error {
	p.Current = make([]*protocol.PresenceMessage, 0)
	reader := bytes.NewReader(p.paginatedResource.Body)
	return json.NewDecoder(reader).Decode(&p.Current)
}

func NewPaginatedPresenceMessages(client protocol.ResourceReader, path string, params *config.PaginateParams) (*PaginatedPresenceMessages, error) {
	newPaginatedResource, err := protocol.NewPaginatedResource(client, path, params)
	if err != nil {
		return nil, err
	}

	paginatedPresenceMessages := &PaginatedPresenceMessages{paginatedResource: newPaginatedResource}
	return paginatedPresenceMessages, paginatedPresenceMessages.decode()
}

func (p *PaginatedPresenceMessages) NextPage() (*PaginatedPresenceMessages, error) {
	path, err := p.paginatedResource.NextPage()
	if err != nil {
		return nil, err
	}

	return NewPaginatedPresenceMessages(p.paginatedResource.ResourceReader, path, nil)
}
