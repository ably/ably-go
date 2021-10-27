package main

// go run publish.go constants.go utils.go

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ably/ably-go/ably"
)

func main() {
	// Connect to Ably using the API key and ClientID
	client, err := ably.NewREST(
		ably.WithKey(os.Getenv(AblyKey)),
		ably.WithClientID(UserName))
	if err != nil {
		panic(err)
	}

	checkRestPublish(client)
	checkRestBulkPublish(client)
}

func checkRestPublish(client *ably.REST) {
	channel := client.Channels.Get(ChannelName)
	realtimeClient := initRealtimeClient()
	unsubscribe := realtimeSubscribeToEvent(realtimeClient)

	restPublish(channel, "Hey there")

	time.Sleep(time.Second)
	unsubscribe()
	realtimeClient.Close()
}

func checkRestBulkPublish(client *ably.REST) {
	channel := client.Channels.Get(ChannelName)
	realtimeClient := initRealtimeClient()
	unsubscribe := realtimeSubscribeToEvent(realtimeClient)

	restPublishBatch(channel, "Hey there", "How are you?")

	time.Sleep(time.Second)
	unsubscribe()
	realtimeClient.Close()
}

func restPublish(channel *ably.RESTChannel, message string) {

	err := channel.Publish(context.Background(), EventName, message)
	if err != nil {
		err := fmt.Errorf("error publishing to channel: %w", err)
		panic(err)
	}
}

func restPublishBatch(channel *ably.RESTChannel, message1 string, message2 string) {

	err := channel.PublishMultiple(context.Background(), []*ably.Message{
		{Name: EventName, Data: message1},
		{Name: EventName, Data: message2},
	})
	if err != nil {
		err := fmt.Errorf("error batch publishing to channel: %w", err)
		panic(err)
	}
}
