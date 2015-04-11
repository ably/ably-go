package rest

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/proto"
)

type Presence struct {
	client  *Client
	channel *Channel
}

// Get gives the channel's presence messages according to the given parameters.
// The returned resource can be inspected for the presence messages via
// the PresenceMessages() method.
func (p *Presence) Get(params *config.PaginateParams) (*proto.PaginatedResource, error) {
	path := "/channels/" + p.channel.Name + "/presence"
	return proto.NewPaginatedResource(presMsgType, path, params, query(p.client.Get))
}

// History gives the channel's presence messages history according to the given
// parameters. The returned resource can be inspected for the presence messages
// via the PresenceMessages() method.
func (p *Presence) History(params *config.PaginateParams) (*proto.PaginatedResource, error) {
	path := "/channels/" + p.channel.Name + "/presence/history"
	return proto.NewPaginatedResource(presMsgType, path, params, query(p.client.Get))
}
