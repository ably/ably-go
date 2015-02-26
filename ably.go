package ably

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/realtime"
	"github.com/ably/ably-go/rest"
)

// Wrapper around config.Params
//
// This is only a convenience structure to be able to call ably.ClientOptions
// instead of importing Params from the config package.
type ClientOptions struct {
	config.Params
}

func (c *ClientOptions) ToParams() config.Params {
	return c.Params
}

func NewRestClient(clientOptions *ClientOptions) *rest.Client {
	return rest.NewClient(clientOptions.ToParams())
}

func NewRealtimeClient(clientOptions *ClientOptions) *realtime.Client {
	return realtime.NewClient(clientOptions.ToParams())
}
