package ably

import (
	"context"
	"errors"

	"github.com/ably/ably-go/ably/objects"
)

type RealtimeExperimentalObjects struct {
	channel *RealtimeChannel
}

func newRealtimeExperimentalObjects(channel *RealtimeChannel) *RealtimeExperimentalObjects {
	return &RealtimeExperimentalObjects{channel: channel}
}

// PublishObjects publishes the given object messages to the channel.
//
// An objects plugin must be configured on the realtime client, which is used
// to prepare the message to be published (e.g. setting the objectId and
// initialValue for a COUNTER_CREATE op).
func (o *RealtimeExperimentalObjects) PublishObjects(ctx context.Context, msgs ...*objects.Message) error {
	plugin := o.channel.client.opts().ObjectsPlugin
	if plugin == nil {
		return errors.New("missing objects plugin")
	}
	for _, msg := range msgs {
		if err := plugin.PrepareObject(msg); err != nil {
			return err
		}
	}

	listen := make(chan error, 1)
	onAck := func(err error) {
		listen <- err
	}

	msg := &protocolMessage{
		Action:  actionObject,
		Channel: o.channel.Name,
		State:   msgs,
	}
	if err := o.channel.send(msg, onAck); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-listen:
		return err
	}
}
