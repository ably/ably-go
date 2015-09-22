package ably

type RestPresence struct {
	client  *RestClient
	channel *RestChannel
}

// Get gives the channel's presence messages according to the given parameters.
// The returned result can be inspected for the presence messages via
// the PresenceMessages() method.
func (p *RestPresence) Get(params *PaginateParams) (*PaginatedResult, error) {
	path := "/channels/" + p.channel.Name + "/presence"
	return newPaginatedResult(presMsgType, path, params, query(p.client.get))
}

// History gives the channel's presence messages history according to the given
// parameters. The returned result can be inspected for the presence messages
// via the PresenceMessages() method.
func (p *RestPresence) History(params *PaginateParams) (*PaginatedResult, error) {
	path := "/channels/" + p.channel.Name + "/presence/history"
	return newPaginatedResult(presMsgType, path, params, query(p.client.get))
}
