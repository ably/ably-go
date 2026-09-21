package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ably/ably-go/examples"
	"github.com/ably/ably-go/pubsub/server"
)

func main() {
	// Connect to Ably using the API key and ClientID specified
	client, err := server.NewRealtimeClient(
		server.WithKey(os.Getenv(examples.AblyKey)),
		server.WithClientID(examples.UserName))
	if err != nil {
		panic(err)
	}

	checkSubscribeAll(client)
	checkSubscribeToEvent(client)
}

func checkSubscribeAll(client *server.RealtimeClient) {

	channel := client.Channels.Get(examples.ChannelName)

	unsubscribeAll := subscribeAll(channel)

	publish(channel, "Hey there !!")

	time.Sleep(time.Second)

	unsubscribeAll()
}

func checkSubscribeToEvent(client *server.RealtimeClient) {
	// Connect to the Ably Channel with name 'chat'
	channel := client.Channels.Get(examples.ChannelName)

	unsubscribe := subscribeToEvent(channel)

	// publish message with blocking call
	publish(channel, "Hey there !!")

	time.Sleep(time.Second)

	unsubscribe()
}

func subscribeToEvent(channel *server.RealtimeChannel) func() {
	// Subscribe to messages sent on the channel with given eventName
	unsubscribe, err := channel.Subscribe(context.Background(), examples.EventName, func(msg *server.Message) {
		fmt.Printf("Received message from %v: '%v'\n", msg.ClientID, msg.Data)
	})
	if err != nil {
		err := fmt.Errorf("error subscribing to channel: %w", err)
		fmt.Println(err)
	}
	return unsubscribe
}

func subscribeAll(channel *server.RealtimeChannel) func() {
	// Subscribe to all messages sent on the channel
	unsubscribeAll, err := channel.SubscribeAll(context.Background(), func(msg *server.Message) {
		fmt.Printf("Received message from %v: '%v'\n", msg.ClientID, msg.Data)
	})
	if err != nil {
		err := fmt.Errorf("error subscribing to channel: %w", err)
		fmt.Println(err)
	}
	return unsubscribeAll
}

func publish(channel *server.RealtimeChannel, message string) {
	// Publish the message to Ably Channel
	err := channel.Publish(context.Background(), examples.EventName, message)
	if err != nil {
		err := fmt.Errorf("error publishing to channel: %w", err)
		fmt.Println(err)
	}
}
