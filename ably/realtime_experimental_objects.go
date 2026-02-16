package ably

import (
	"context"
	"errors"

	"github.com/ably/ably-go/ably/objects"
)

// RealtimeExperimentalObjects exposes a method to publish LiveObject messages on a Realtime channel.
// It requires the LiveObjects plugin to be configured see ably-go/ably/objects/liveobjects.go.
//
// NOTE: this option is experimental, the LiveObjects plugin API may change in a
// backwards incompatible way between minor/patch versions. Once the API has been finalised,
// a new non-experimental option will be added, and this one will be removed.
//
// [LiveObjects]: https://ably.com/docs/liveobjects
type RealtimeExperimentalObjects struct {
	channel channel
}

func newRealtimeExperimentalObjects(channel channel) *RealtimeExperimentalObjects {
	return &RealtimeExperimentalObjects{channel: channel}
}

// PublishObjects publishes the given object messages to the channel.
//
// An objects plugin must be configured on the realtime client, which is used
// to prepare the message to be published (e.g. setting the objectId and
// initialValue for a COUNTER_CREATE op).
func (o *RealtimeExperimentalObjects) PublishObjects(ctx context.Context, msgs ...*objects.Message) error {
	plugin := o.channel.getClientOptions().ExperimentalObjectsPlugin
	if plugin == nil {
		return newError(ErrMissingPlugin, errors.New("missing objects plugin"))
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
		Channel: o.channel.getName(),
		State:   msgs,
	}
	if err := o.channel.send(msg, &ackCallback{onAck: onAck}); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-listen:
		return err
	}
}

type channel interface {
	send(msg *protocolMessage, callback *ackCallback) error
	getClientOptions() *clientOptions
	getName() string
}
