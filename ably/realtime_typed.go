package ably

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func (r *RealtimeChannelOf[T]) Publish(ctx context.Context, name string, o T) error {
	return r.RealtimeChannel.Publish(ctx, name, &o)
}

func (r *RealtimeChannelOf[T]) Subscribe(ctx context.Context, name string, handle func(*MessageOf[T], error)) (func(), error) {
	return r.RealtimeChannel.Subscribe(ctx, name, func(msg *Message) {
		var mo MessageOf[T]
		mo.Name = msg.Name
		mo.ID = msg.ID
		mo.Timestamp = msg.Timestamp
		mo.ClientID = msg.ClientID
		mo.Extras = msg.Extras

		// this switch statement does not work
		switch any(mo.Data).(type) {
		case string:
		case []byte:
			var ok bool
			mo.Data, ok = msg.Data.(T)
			if !ok {
				handle(nil, fmt.Errorf("can not decode message of type %T into %T", msg.Data, mo.Data))
				return
			}
			handle(&mo, nil)
			return
		}

		var r io.Reader
		switch d := msg.Data.(type) {
		case string:
			r = strings.NewReader(d)
		case []byte:
			r = bytes.NewReader(d)
		default:
			err := fmt.Errorf("Could not decode message data of type %T into %T", mo.Data, msg.Data)
			handle(nil, err)
		}
		err := json.NewDecoder(r).Decode(&mo.Data)
		if err != nil {
			handle(nil, fmt.Errorf("could not decode message value into %T, %w", mo.Data, err))
			return
		}

		handle(&mo, nil)
	})
}

func GetChannelOf[T any](client *Realtime, name string) *RealtimeChannelOf[T] {
	ch := client.Channels.Get(name)
	return &RealtimeChannelOf[T]{*ch}
}
