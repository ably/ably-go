//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ably/ably-go/ably"
)

// When publishing a message to a channel, data can be either a single string or
// a struct of type Message. This example shows the different ways to publish a message.
func ExampleRESTChannel_Publish() {

	// Create a new REST client.
	client, err := ably.NewREST(
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

	// Publish a string to the channel.
	if err := channel.Publish(ctx, "chat_message", "Hello, how are you?"); err != nil {
		fmt.Println(err)
		return
	}

	// Publish a single message to the channel.
	newChatMessage := ably.Message{
		Name: "chat_message",
		Data: "Hello, how are you?",
	}

	if err := channel.Publish(ctx, "chat_message", newChatMessage); err != nil {
		fmt.Println(err)
		return
	}

	// Publish multiple messages in a single request.
	if err := channel.PublishMultiple(ctx, []*ably.Message{
		{Name: "HelloEvent", Data: "Hello!"},
		{Name: "ByeEvent", Data: "Bye!"},
	}); err != nil {
		fmt.Println(err)
		return
	}

	// Publish a message on behalf of a different client.
	if err := channel.Publish(ctx, "temperature", "12.7",
		ably.PublishWithConnectionKey("connectionKeyOfAnotherClient"),
	); err != nil {
		fmt.Println(err)
		return
	}
}
