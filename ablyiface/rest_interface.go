package ablyiface

import (
	"context"
	"time"

	ably "github.com/ably/ably-go/ably"
)

// RESTChannels
type RESTChannel interface{
	Iterate() []*RESTChannel
}

//RESTClient is an interface that has the same methods as ably.REST
type RESTClientAPI interface {
	Time(ctx context.Context) (time.Time, error)
	Stats(o ...ably.StatsOption) ably.StatsRequest
	StatsByItems(ctx context.Context, o ...ably.StatsOption) (*ably.StatsPaginatedItems, error)
	Request(method string, path string, o ...ably.RequestOption) ably.RESTRequest
	
	// Auth 
	GetClientID() string
	CreateTokenRequest(params *ably.TokenParams, opts ...ably.AuthOption) (*ably.TokenRequest, error)
	RequestToken(ctx context.Context, params *ably.TokenParams, opts ...ably.AuthOption) (*ably.TokenDetails, error)
	Authorize(ctx context.Context, params *ably.TokenParams, setOpts ...ably.AuthOption) (*ably.TokenDetails, error)
	
	// Channel
	Release(channel string)
	Publish(ctx context.Context, channel string, name string, data interface{}, o ...ably.PublishMultipleOption) error
	PublishMultiple(ctx context.Context, channel string, messages []*ably.Message, o ...ably.PublishMultipleOption) error
	History(channel string, o ...ably.HistoryOption) ably.HistoryRequest
	
	//Presence
	GetPresence(channel string, o ...ably.GetPresenceOption) ably.PresenceRequest
	PresenceHistory(channel string, o ...ably.PresenceHistoryOption) ably.PresenceRequest
}

// Ensure that *ably.Rest satisfies the RESTClientAPI interface
var _ RESTClientAPI = (*ably.REST)(nil)
