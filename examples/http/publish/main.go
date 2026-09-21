package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ably/ably-pubsub-go/examples"
	"github.com/ably/ably-pubsub-go/server"
)

func main() {
	// Connect to Ably using the API key and ClientID
	client, err := pubsub.NewHTTPClient(
		pubsub.WithKey(os.Getenv(examples.AblyKey)),
		pubsub.WithClientID(examples.UserName))
	if err != nil {
		panic(err)
	}

	checkHTTPPublish(client)
	checkHTTPBulkPublish(client)
}

func checkHTTPPublish(client *pubsub.HTTPClient) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	unsubscribe := examples.RealtimeSubscribeToEvent(realtimeClient)

	httpPublish(channel, "Hey there")

	time.Sleep(time.Second)
	unsubscribe()
	realtimeClient.Close()
}

func checkHTTPBulkPublish(client *pubsub.HTTPClient) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	unsubscribe := examples.RealtimeSubscribeToEvent(realtimeClient)

	httpPublishBatch(channel, "Hey there", "How are you?")

	time.Sleep(time.Second)
	unsubscribe()
	realtimeClient.Close()
}

func httpPublish(channel *pubsub.HTTPChannel, message string) {

	err := channel.Publish(context.Background(), examples.EventName, message)
	if err != nil {
		err := fmt.Errorf("error publishing to channel: %w", err)
		panic(err)
	}
}

func httpPublishBatch(channel *pubsub.HTTPChannel, message1 string, message2 string) {

	err := channel.PublishMultiple(context.Background(), []*pubsub.Message{
		{Name: examples.EventName, Data: message1},
		{Name: examples.EventName, Data: message2},
	})
	if err != nil {
		err := fmt.Errorf("error batch publishing to channel: %w", err)
		panic(err)
	}
}
