package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/examples"
)

func main() {
	// Connect to Ably using the API key and ClientID
	client, err := ably.NewREST(
		ably.WithKey(os.Getenv(examples.AblyKey)),
		ably.WithClientID(examples.UserName))
	if err != nil {
		panic(err)
	}

	checkRestPublish(client)
	checkRestBulkPublish(client)
}

func checkRestPublish(client *ably.REST) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	unsubscribe := examples.RealtimeSubscribeToEvent(realtimeClient)

	restPublish(channel, "Hey there")

	time.Sleep(time.Second)
	unsubscribe()
	realtimeClient.Close()
}

func checkRestBulkPublish(client *ably.REST) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	unsubscribe := examples.RealtimeSubscribeToEvent(realtimeClient)

	restPublishBatch(channel, "Hey there", "How are you?")

	time.Sleep(time.Second)
	unsubscribe()
	realtimeClient.Close()
}

func restPublish(channel *ably.RESTChannel, message string) {

	err := channel.Publish(context.Background(), examples.EventName, message)
	if err != nil {
		err := fmt.Errorf("error publishing to channel: %w", err)
		panic(err)
	}
}

func restPublishBatch(channel *ably.RESTChannel, message1 string, message2 string) {

	err := channel.PublishMultiple(context.Background(), []*ably.Message{
		{Name: examples.EventName, Data: message1},
		{Name: examples.EventName, Data: message2},
	})
	if err != nil {
		err := fmt.Errorf("error batch publishing to channel: %w", err)
		panic(err)
	}
}
