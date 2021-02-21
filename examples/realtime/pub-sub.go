package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/ably/ably-go/ably"
	"github.com/joho/godotenv"
	"os"
	"time"
)

var EventName = "message"
var UserName = "testUser"
var AblyKey = "ABLY_KEY"
var ChannelName = "chat"

func main() {
	godotenv.Load()

	// Connect to Ably using the API key and ClientID specified above
	client, err := ably.NewRealtime(
		ably.WithKey(os.Getenv(AblyKey)),
		// ably.WithEchoMessages(true), // Uncomment to stop messages you send from being sent back
		ably.WithClientID(UserName))
	if err != nil {
		panic(err)
	}

	checkSubscribeAll(client)
	checkSubscribeToEvent(client)

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
}


func checkSubscribeAll(client *ably.Realtime) {
	// Connect to the Ably Channel with name 'chat'
	channel := client.Channels.Get(ChannelName)

	unsubscribeAll := subscribeAll(channel)
	// Start the goroutine to allow for publishing messages
	publish(channel, "Hey there !!")

	time.Sleep(time.Second)
	//
	unsubscribeAll();
}

func checkSubscribeToEvent(client *ably.Realtime) {
	// Connect to the Ably Channel with name 'chat'
	channel := client.Channels.Get(ChannelName)

	unsubscribe := subscribeToEvent(channel)
	// Start the goroutine to allow for publishing messages
	publish(channel, "Hey there !!")

	time.Sleep(time.Second)
	//
	unsubscribe();
}


func subscribeToEvent(channel *ably.RealtimeChannel) func() {
	// Subscribe to messages sent on the channel
	unsubscribe , err := channel.Subscribe(context.Background(), EventName, func(msg *ably.Message) {
		fmt.Printf("Received message from %v: '%v'\n", msg.ClientID, msg.Data)
	})
	if err != nil {
		err := fmt.Errorf("subscribing to channel: %w", err)
		fmt.Println(err)
	}
	return unsubscribe
}


func subscribeAll(channel *ably.RealtimeChannel) func() {
	// Subscribe to messages sent on the channel
	unsubscribeAll , err := channel.SubscribeAll(context.Background(), func(msg *ably.Message) {
		fmt.Printf("Received message from %v: '%v'\n", msg.ClientID, msg.Data)
	})
	if err != nil {
		err := fmt.Errorf("subscribing to channel: %w", err)
		fmt.Println(err)
	}
	return unsubscribeAll
}

func publish(channel *ably.RealtimeChannel, message string) {
	// Publish the message typed in to the Ably Channel
	err := channel.Publish(context.Background(), EventName, message)
	// await confirmation that message was received by Ably
	if err != nil {
		err := fmt.Errorf("publishing to channel: %w", err)
		fmt.Println(err)
	}
}
