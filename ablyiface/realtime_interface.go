package ablyiface

import (
	"context"
	"time"

	ably "github.com/ably/ably-go/ably"
)

//RealtimeClient is an interface that has the same methods as ably.Realtime
type RealtimeClientAPI interface {
	Stats(o ...ably.StatsOption) ably.StatsRequest
	Time(ctx context.Context) (time.Time, error)

	// Auth
	ClientID() string
	CreateTokenRequest(params *ably.TokenParams, opts ...ably.AuthOption) (*ably.TokenRequest, error)
	RequestToken(ctx context.Context, params *ably.TokenParams, opts ...ably.AuthOption) (*ably.TokenDetails, error)
	Authorize(ctx context.Context, params *ably.TokenParams, setOpts ...ably.AuthOption) (*ably.TokenDetails, error)

	// Connection
	Connect()
	Close()
	ConnectionID() string
	ConnectionKey() string
	ConnectionErrorReason() *ably.ErrorInfo
	ConnectionRecoveryKey() string
	ConnectionSerial() *int64
	ConnectionState() ably.ConnectionState

	// Channel
	Release(ctx context.Context, channel string)
	Attach(ctx context.Context, channel string) error
	Detach(ctx context.Context, channel string) error
	Subscribe(ctx context.Context, channel string, name string, handle func(*ably.Message)) (func(), error)
	SubscribeAll(ctx context.Context, channel string, handle func(*ably.Message)) (func(), error)
	Publish(ctx context.Context, channel string, name string, data interface{}) error
	PublishMultiple(ctx context.Context, channel string, messages []*ably.Message) error
	History(channel string, o ...ably.HistoryOption) ably.HistoryRequest
	State(channel string) ably.ChannelState
	ErrorReason(channel string) *ably.ErrorInfo
	Modes(channel string) []ably.ChannelMode
	Params(channel string) map[string]string

	// Presence
	PresenceSyncComplete(channel string) bool
	GetPresence(ctx context.Context, channel string) ([]*ably.PresenceMessage, error)
	GetPresenceWithOptions(ctx context.Context, channel string, options ...ably.PresenceGetOption) ([]*ably.PresenceMessage, error)
	PresenceSubscribe(ctx context.Context, channel string, action ably.PresenceAction, handle func(*ably.PresenceMessage)) (func(), error)
	PresenceSubscribeAll(ctx context.Context, channel string, handle func(*ably.PresenceMessage)) (func(), error)
	PresenceEnter(ctx context.Context, channel string, data interface{}) error
	PresenceUpdate(ctx context.Context, channel string, data interface{}) error
	PresenceLeave(ctx context.Context, channel string, data interface{}) error
	PresenceEnterClient(ctx context.Context, channel string, clientID string, data interface{}) error
	PresenceUpdateClient(ctx context.Context, channel string, clientID string, data interface{}) error
	PresenceLeaveClient(ctx context.Context, channel string, clientID string, data interface{}) error
}

// Ensure that *ably.Realtime satisfies the RealtimeClientAPI interface
var _ RealtimeClientAPI = (*ably.Realtime)(nil)
