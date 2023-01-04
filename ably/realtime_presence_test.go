//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ably/ably-go/ably"
)

// When a client is created without a ClientID, EnterClient is used to announce the presence of a client.
// This example shows a client without a clientID announcing the presence of "Client A" using EnterClient.
func ExampleRealtimePresence_EnterClient() {

	// A new realtime client is created without providing a ClientID.
	client, err := ably.NewRealtime(
		ably.WithKey("ABLY_PRIVATE_KEY"),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	// A new channel is initialised.
	channel := client.Channels.Get("chat")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// The presence of Client A is announced using EnterClient.
	if err := channel.Presence.EnterClient(ctx, "Client A", nil); err != nil {
		fmt.Println(err)
		return
	}
}
