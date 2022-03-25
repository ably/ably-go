//go:build !integration

package ably_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ably/ably-go/ably"
)

// When publishing a message to a channel, data can be either a single string or
// a struct of type Message. This example shows the different ways to publish a message.
func ExampleRealtimeChannel_Publish() {

	// Create a new realtime client.
	client, err := ably.NewRealtime(
		ably.WithKey("ABLY_PRIVATE_KEY"),
		ably.WithClientID("Client A"),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Initialise a new channel.
	channel := client.Channels.Get("chat")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Publish a string to the channel
	if err := channel.Publish(ctx, "chat_message", "Hello, how are you?"); err != nil {
		fmt.Println(err)
		return
	}

	// Publish a Message to the channel
	newChatMessage := ably.Message{
		Name: "chat_message",
		Data: "Hello, how are you?",
	}

	if err := channel.Publish(ctx, "chat_message", newChatMessage); err != nil {
		fmt.Println(err)
		return
	}
}
