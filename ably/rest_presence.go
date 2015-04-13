package ably

type RestPresence struct {
	client  *RestClient
	channel *RestChannel
}

// Get gives the channel's presence messages according to the given parameters.
// The returned resource can be inspected for the presence messages via
// the PresenceMessages() method.
func (p *RestPresence) Get(params *PaginateParams) (*PaginatedResource, error) {
	path := "/channels/" + p.channel.Name + "/presence"
	return newPaginatedResource(presMsgType, path, params, query(p.client.Get))
}

// History gives the channel's presence messages history according to the given
// parameters. The returned resource can be inspected for the presence messages
// via the PresenceMessages() method.
func (p *RestPresence) History(params *PaginateParams) (*PaginatedResource, error) {
	path := "/channels/" + p.channel.Name + "/presence/history"
	return newPaginatedResource(presMsgType, path, params, query(p.client.Get))
}
