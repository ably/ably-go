package rest

import "github.com/ably/ably-go/protocol"

type Presence struct {
	client  *Client
	channel *Channel
}

func (p *Presence) Get() ([]*protocol.PresenceMessage, error) {
	msgs := []*protocol.PresenceMessage{}
	_, err := p.client.Get("/channels/"+p.channel.Name+"/presence", &msgs)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

func (p *Presence) History() ([]*protocol.PresenceMessage, error) {
	msgs := []*protocol.PresenceMessage{}
	_, err := p.client.Get("/channels/"+p.channel.Name+"/presence/history", &msgs)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}
