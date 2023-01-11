package ably

import (
	"context"
	"encoding/json"
	"strings"
)

type MessageOf[T any] struct {

	// Data is the message payload, if provided (TM2d).
	Data T `json:"data,omitempty" codec:"data,omitempty"`
	Message
}

type RealtimeChannelOf[T any] struct {
	RealtimeChannel
}

func (r *RealtimeChannelOf[T]) PublishOf(ctx context.Context, name string, o T) error {
	return r.Publish(ctx, name, &o)
}

func (r *RealtimeChannelOf[T]) SubscribeOf(ctx context.Context, name string, handle func(*MessageOf[T])) (func(), error) {
	return r.Subscribe(ctx, name, func(msg *Message) {
		var val T
		err := json.NewDecoder(strings.NewReader(msg.Data.(string))).Decode(&val)
		if err != nil {
			panic(err)
		}
		var mo MessageOf[T]
		mo.Name = msg.Name
		mo.ID = msg.ID
		mo.Timestamp = msg.Timestamp
		mo.Data = val

		handle(&mo)
	})
}

func GetChannelOf[T any](client *Realtime, name string) *RealtimeChannelOf[T] {
	ch := client.Channels.Get(name)
	return &RealtimeChannelOf[T]{*ch}
}
