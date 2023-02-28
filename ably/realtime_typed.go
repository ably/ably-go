package ably

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MessageOf is a strongly typed varient of Message, where the Data value is constrained to be a of type T.
type MessageOf[T any] struct {
	// Data is the message payload, if provided (TM2d).
	Data T `json:"data,omitempty" codec:"data,omitempty"`
	Message
}

type RealtimeChannelOf[T any] struct {
	RealtimeChannel
}

// Publish publishes a message of type T.
func (r *RealtimeChannelOf[T]) Publish(ctx context.Context, name string, o T) error {
	return r.RealtimeChannel.Publish(ctx, name, &o)
}

type DecodeError struct {
	Want any
	Got  any
}

func (e DecodeError) Error() string {
	return fmt.Sprintf("can not decode message of type %T into %T", e.Got, e.Want)
}

// Subscribe subscribes to messages. When a message arrives, it decoded as a MessageOf[T] and handle is called.
// If the message can not be decoded, then handle is called with a DecodeError.
func (r *RealtimeChannelOf[T]) Subscribe(ctx context.Context, name string, handle func(*MessageOf[T], error)) (func(), error) {
	return r.RealtimeChannel.Subscribe(ctx, name, func(msg *Message) {
		var mo MessageOf[T]
		mo.Name = msg.Name
		mo.ID = msg.ID
		mo.Timestamp = msg.Timestamp
		mo.ClientID = msg.ClientID
		mo.Extras = msg.Extras

		// TODO: this switch statement does not work.
		switch msg.Data.(type) {
		case string:
		case []byte:
			var ok bool
			mo.Data, ok = msg.Data.(T)
			if !ok {
				handle(nil, DecodeError{msg.Data, mo.Data})
				return
			}
			handle(&mo, nil)
			return
		}

		var r io.Reader
		fmt.Printf("%T", msg.Data)
		switch d := msg.Data.(type) {
		case string:
			r = strings.NewReader(d)
		case []byte:
			r = bytes.NewReader(d)
		default:
			handle(nil, DecodeError{msg.Data, mo.Data})
		}
		err := json.NewDecoder(r).Decode(&mo.Data)
		if err != nil {
			handle(nil, fmt.Errorf("could not decode message value into %T, %w", mo.Data, err))
			return
		}

		handle(&mo, nil)
	})
}

// GetChannelOf[T] returns a channel of messages of type MessageOf[T].
func GetChannelOf[T any](client *Realtime, name string) *RealtimeChannelOf[T] {
	ch := client.Channels.Get(name)
	return &RealtimeChannelOf[T]{*ch}
}
