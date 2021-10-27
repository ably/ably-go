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
	// Connect to Ably using the API key and ClientID specified
	client, err := ably.NewRealtime(
		ably.WithKey(os.Getenv(examples.AblyKey)),
		ably.WithClientID(examples.UserName))
	if err != nil {
		panic(err)
	}

	checkSubscribeAll(client)
	checkSubscribeToEvent(client)
}

func checkSubscribeAll(client *ably.Realtime) {

	channel := client.Channels.Get(examples.ChannelName)

	unsubscribeAll := subscribeAll(channel)

	publish(channel, "Hey there !!")

	time.Sleep(time.Second)

	unsubscribeAll()
}

func checkSubscribeToEvent(client *ably.Realtime) {
	// Connect to the Ably Channel with name 'chat'
	channel := client.Channels.Get(examples.ChannelName)

	unsubscribe := subscribeToEvent(channel)

	// publish message with blocking call
	publish(channel, "Hey there !!")

	time.Sleep(time.Second)

	unsubscribe()
}

func subscribeToEvent(channel *ably.RealtimeChannel) func() {
	// Subscribe to messages sent on the channel with given eventName
	unsubscribe, err := channel.Subscribe(context.Background(), examples.EventName, func(msg *ably.Message) {
		fmt.Printf("Received message from %v: '%v'\n", msg.ClientID, msg.Data)
	})
	if err != nil {
		err := fmt.Errorf("error subscribing to channel: %w", err)
		fmt.Println(err)
	}
	return unsubscribe
}

func subscribeAll(channel *ably.RealtimeChannel) func() {
	// Subscribe to all messages sent on the channel
	unsubscribeAll, err := channel.SubscribeAll(context.Background(), func(msg *ably.Message) {
		fmt.Printf("Received message from %v: '%v'\n", msg.ClientID, msg.Data)
	})
	if err != nil {
		err := fmt.Errorf("error subscribing to channel: %w", err)
		fmt.Println(err)
	}
	return unsubscribeAll
}

func publish(channel *ably.RealtimeChannel, message string) {
	// Publish the message to Ably Channel
	err := channel.Publish(context.Background(), examples.EventName, message)
	if err != nil {
		err := fmt.Errorf("error publishing to channel: %w", err)
		fmt.Println(err)
	}
}
