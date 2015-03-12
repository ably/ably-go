package ably

import (
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/realtime"
	"github.com/ably/ably-go/rest"
)

func NewRestClient(params config.Params) *rest.Client {
	params.Prepare()
	return rest.NewClient(params)
}

func NewRealtimeClient(params config.Params) *realtime.Client {
	params.Prepare()
	return realtime.NewClient(params)
}
