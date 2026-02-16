package ably_test

import (
	"context"
	"fmt"

	"github.com/ably/ably-go/ably"
)

// Example demonstrating how to publish a message and get its serial
func ExampleRESTChannel_PublishWithResult() {
	client, err := ably.NewREST(ably.WithKey("xxx:xxx"))
	if err != nil {
		panic(err)
	}

	channel := client.Channels.Get("example-channel")

	// Publish a message and get its serial
	result, err := channel.PublishWithResult(context.Background(), "event-name", "message data")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Message published with serial: %s\n", result.Serial)
}

// Example demonstrating how to update a message
func ExampleRESTChannel_UpdateMessage() {
	client, err := ably.NewREST(ably.WithKey("xxx:xxx"))
	if err != nil {
		panic(err)
	}

	channel := client.Channels.Get("example-channel")

	// First publish a message to get its serial
	result, err := channel.PublishWithResult(context.Background(), "event", "initial data")
	if err != nil {
		panic(err)
	}

	// Update the message
	msg := &ably.Message{
		Serial: result.Serial,
		Data:   "updated data",
	}

	updateResult, err := channel.UpdateMessage(
		context.Background(),
		msg,
		ably.UpdateWithDescription("Fixed typo"),
		ably.UpdateWithMetadata(map[string]string{"editor": "alice"}),
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Message updated with version serial: %s\n", updateResult.VersionSerial)
}

// Example demonstrating async message append for AI streaming
func ExampleRealtimeChannel_AppendMessageAsync() {
	client, err := ably.NewRealtime(ably.WithKey("xxx:xxx"))
	if err != nil {
		panic(err)
	}

	channel := client.Channels.Get("chat-channel")

	// Publish initial message
	result, err := channel.PublishWithResult(context.Background(), "ai-response", "The answer is")
	if err != nil {
		panic(err)
	}

	// Stream tokens asynchronously without blocking
	tokens := []string{" 42", ".", " This", " is", " the", " answer."}
	for _, token := range tokens {
		msg := &ably.Message{
			Serial: result.Serial,
			Data:   token,
		}
		// Non-blocking append - critical for AI streaming
		err := channel.AppendMessageAsync(msg, func(r *ably.UpdateResult, err error) {
			if err != nil {
				fmt.Printf("Append failed: %v\n", err)
			}
		})
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("All tokens queued for append")
}
