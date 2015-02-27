package ably

import (
	"github.com/ably/ably-go/realtime"
	"github.com/ably/ably-go/rest"
)

func NewRestClient(clientOptions *ClientOptions) *rest.Client {
	return rest.NewClient(clientOptions.ToParams())
}

func NewRealtimeClient(clientOptions *ClientOptions) *realtime.Client {
	return realtime.NewClient(clientOptions.ToParams())
}
