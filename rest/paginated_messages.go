package rest

import "github.com/ably/ably-go/protocol"

// PaginatedMessages represents a single page of messages.
type PaginatedMessages struct {
	paginatedResource *protocol.PaginatedResource
	channel           *Channel

	Current []*protocol.Message
}

func (p *PaginatedMessages) NextPage() (*PaginatedMessages, error) {
	path, err := p.paginatedResource.NextPage()
	if err != nil {
		return nil, err
	}

	return p.channel.paginatedGet(path, nil)
}
