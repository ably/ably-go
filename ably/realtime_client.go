package ably

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// The Realtime libraries establish and maintain a persistent connection
// to Ably enabling extremely low latency broadcasting of messages and presence
// state.
type Realtime struct {
	Auth        *Auth
	Channels    *RealtimeChannels           // ChannelsMap will eventually take the name Channels can change via expand and contract pattern.
	ChannelsMap map[string]*RealtimeChannel // Moved up to top level from Realtime.Channels.chans
	channelsMu  sync.Mutex                  // Moved up to top level from Realtime.Channels.mtx
	Connection  *Connection
	rest        *REST
}

// NewRealtime constructs a new Realtime.
func NewRealtime(options ...ClientOption) (*Realtime, error) {
	c := &Realtime{}
	rest, err := NewREST(options...) //options validated in NewREST
	if err != nil {
		return nil, err
	}
	c.rest = rest
	c.Auth = rest.Auth
	c.Channels = newChannels(c)
	conn := newConn(c.opts(), rest.Auth, connCallbacks{
		c.onChannelMsg,
		c.onReconnected,
		c.onReconnectionFailed,
	})
	conn.internalEmitter.OnAll(func(change ConnectionStateChange) {
		c.Channels.broadcastConnStateChange(change)
	})
	c.Connection = conn
	return c, nil
}

// Connect is the same as Connection.Connect.
func (c *Realtime) Connect() {
	c.Connection.Connect()
}

// Close is the same as Connection.Close.
func (c *Realtime) Close() {
	c.Connection.Close()
}

// Stats is the same as REST.Stats.
func (c *Realtime) Stats(o ...StatsOption) StatsRequest {
	return c.rest.Stats(o...)
}

// Time
func (c *Realtime) Time(ctx context.Context) (time.Time, error) {
	return c.rest.Time(ctx)
}

// ClientID, CreateTokenRequest, RequestToken, Authorize are wrappers that duplicate functionality from Realtime.Auth.xxx to Realtime.xxx
func (c *Realtime) ClientID() string {
	return c.Auth.ClientID()
}

func (c *Realtime) CreateTokenRequest(params *TokenParams, opts ...AuthOption) (*TokenRequest, error) {
	return c.Auth.CreateTokenRequest(params, opts...)
}

func (c *Realtime) RequestToken(ctx context.Context, params *TokenParams, opts ...AuthOption) (*TokenDetails, error) {
	return c.Auth.RequestToken(ctx, params, opts...)
}

func (c *Realtime) Authorize(ctx context.Context, params *TokenParams, setOpts ...AuthOption) (*TokenDetails, error) {
	return c.Auth.Authorize(ctx, params, setOpts...)
}

func (c *Realtime) onChannelMsg(msg *protocolMessage) {
	c.Channels.Get(msg.Channel).notify(msg)
}

// GetConnectionID, GetConnectionKey, ConnectionErrorReason, ConnectionRecoveryKey, ConnectionSerial, ConnectionState
// are wrappers that duplicate functionality from Realtime.Connection.xxx
func (c *Realtime) ConnectionID() string {
	return c.Connection.ID()
}

func (c *Realtime) ConnectionKey() string {
	return c.Connection.Key()
}

func (c *Realtime) ConnectionErrorReason() *ErrorInfo {
	return c.Connection.ErrorReason()
}

func (c *Realtime) ConnectionRecoveryKey() string {
	return c.Connection.RecoveryKey()
}

func (c *Realtime) ConnectionSerial() *int64 {
	return c.Connection.Serial()
}

func (c *Realtime) ConnectionState() ConnectionState {
	return c.Connection.State()
}

// ReleaseChannel is a wrapper that duplicates functionality from Realtime.Channels.Release
func (c *Realtime) Release(ctx context.Context, channel string) {
	c.Channels.Release(ctx, channel)
}

//Attach, Detach, Subscribe, SubscribeAll, Publish, PublishMultiple, History, State, ErrorReason, Modes, Params
// are wrappers that duplicate functionality from RealtimeChannel.xxx
func (c *Realtime) Attach(ctx context.Context, channel string) error {
	ch := c.ChannelsMap[channel]
	return ch.Attach(ctx)
}

func (c *Realtime) Detach(ctx context.Context, channel string) error {
	ch := c.ChannelsMap[channel]
	return ch.Detach(ctx)
}

func (c *Realtime) Subscribe(ctx context.Context, channel string, name string, handle func(*Message)) (func(), error) {
	ch := c.ChannelsMap[channel]
	return ch.Subscribe(ctx, name, handle)
}

func (c *Realtime) SubscribeAll(ctx context.Context, channel string, handle func(*Message)) (func(), error) {
	ch := c.ChannelsMap[channel]
	return ch.SubscribeAll(ctx, handle)
}

func (c *Realtime) Publish(ctx context.Context, channel string, name string, data interface{}) error {
	ch := c.ChannelsMap[channel]
	return ch.Publish(ctx, name, data)
}

func (c *Realtime) PublishMultiple(ctx context.Context, channel string, messages []*Message) error {
	ch := c.ChannelsMap[channel]
	return ch.PublishMultiple(ctx, messages)
}

func (c *Realtime) History(channel string, o ...HistoryOption) HistoryRequest {
	ch := c.ChannelsMap[channel]
	return ch.History()
}

func (c *Realtime) State(channel string) ChannelState {
	ch := c.ChannelsMap[channel]
	return ch.State()
}

func (c *Realtime) ErrorReason(channel string) *ErrorInfo {
	ch := c.ChannelsMap[channel]
	return ch.ErrorReason()
}

func (c *Realtime) Modes(channel string) []ChannelMode {
	ch := c.ChannelsMap[channel]
	return ch.Modes()
}

func (c *Realtime) Params(channel string) map[string]string {
	ch := c.ChannelsMap[channel]
	return ch.Params()
}

// PresenceSyncComplete, GetPresence, GetPresenceWithOptions, PresenceSubscribe, PresenceSubscribeAll, PresenceEnter
// PresenceUpdate, PresenceLeave, PresenceEnterClient, PresenceUpdateClient, PresenceLeaveClient
// are wrappers that duplicate functionality from RealtimeClient.Presence.xxx

func (c *Realtime) PresenceSyncComplete(channel string) bool {
	ch := c.ChannelsMap[channel]
	return ch.Presence.SyncComplete()
}

func (c *Realtime) GetPresence(ctx context.Context, channel string) ([]*PresenceMessage, error) {
	ch := c.ChannelsMap[channel]
	return ch.Presence.Get(ctx)
}
func (c *Realtime) GetPresenceWithOptions(ctx context.Context, channel string, options ...PresenceGetOption) ([]*PresenceMessage, error) {
	ch := c.ChannelsMap[channel]
	return ch.Presence.GetWithOptions(ctx, options...)
}

func (c *Realtime) PresenceSubscribe(ctx context.Context, channel string, action PresenceAction, handle func(*PresenceMessage)) (func(), error) {
	ch := c.ChannelsMap[channel]
	return ch.Presence.Subscribe(ctx, action, handle)
}

func (c *Realtime) PresenceSubscribeAll(ctx context.Context, channel string, handle func(*PresenceMessage)) (func(), error) {
	ch := c.ChannelsMap[channel]
	return ch.Presence.SubscribeAll(ctx, handle)
}

func (c *Realtime) PresenceEnter(ctx context.Context, channel string, data interface{}) error {
	ch := c.ChannelsMap[channel]
	return ch.Presence.Enter(ctx, data)
}
func (c *Realtime) PresenceUpdate(ctx context.Context, channel string, data interface{}) error {
	ch := c.ChannelsMap[channel]
	return ch.Presence.Update(ctx, data)
}

func (c *Realtime) PresenceLeave(ctx context.Context, channel string, data interface{}) error {
	ch := c.ChannelsMap[channel]
	return ch.Presence.Leave(ctx, data)
}

func (c *Realtime) PresenceEnterClient(ctx context.Context, channel string, clientID string, data interface{}) error {
	ch := c.ChannelsMap[channel]
	return ch.Presence.EnterClient(ctx, clientID, data)
}
func (c *Realtime) PresenceUpdateClient(ctx context.Context, channel string, clientID string, data interface{}) error {
	ch := c.ChannelsMap[channel]
	return ch.Presence.UpdateClient(ctx, clientID, data)
}

func (c *Realtime) PresenceLeaveClient(ctx context.Context, channel string, clientID string, data interface{}) error {
	ch := c.ChannelsMap[channel]
	return ch.Presence.LeaveClient(ctx, clientID, data)
}

func (c *Realtime) onReconnected(isNewID bool) {
	if !isNewID /* RTN15c3, RTN15g3 */ {
		// No need to reattach: state is preserved. We just need to flush the
		// queue of pending messages.
		for _, ch := range c.Channels.Iterate() {
			ch.queue.Flush()
		}
		//RTN19a
		c.Connection.resendPending()
		return
	}

	for _, ch := range c.Channels.Iterate() {
		switch ch.State() {
		// TODO: SUSPENDED
		case ChannelStateAttaching, ChannelStateAttached: //RTN19b
			ch.mayAttach(false)
		case ChannelStateDetaching: //RTN19b
			ch.detachSkipVerifyActive()
		}
	}
	//RTN19a
	c.Connection.resendPending()
}

func (c *Realtime) onReconnectionFailed(err *errorInfo) {
	for _, ch := range c.Channels.Iterate() {
		ch.setState(ChannelStateFailed, newErrorFromProto(err), false)
	}
}

func isTokenError(err *errorInfo) bool {
	return err != nil && err.StatusCode == http.StatusUnauthorized && (40140 <= err.Code && err.Code < 40150)
}

func (c *Realtime) opts() *clientOptions {
	return c.rest.opts
}

func (c *Realtime) log() logger {
	return c.rest.log
}
