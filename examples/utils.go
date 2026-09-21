package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ably/ably-pubsub-go/server"
)

func InitRealtimeClient() *server.RealtimeClient {
	client, err := server.NewRealtimeClient(
		server.WithKey(os.Getenv(AblyKey)),
		// server.WithEchoMessages(true), // Uncomment to stop messages you send from being sent back
		server.WithClientID(UserName))
	if err != nil {
		panic(err)
	}
	return client
}

func RealtimeSubscribeToEvent(client *server.RealtimeClient) func() {
	channel := client.Channels.Get(ChannelName)

	// Subscribe to messages sent on the channel
	unsubscribe, err := channel.Subscribe(context.Background(), EventName, func(msg *server.Message) {
		fmt.Printf("Received message from %v: '%v'\n", msg.ClientID, msg.Data)
	})
	if err != nil {
		err := fmt.Errorf("error subscribing to channel: %w", err)
		fmt.Println(err)
	}
	return unsubscribe
}

func RealtimeEnterPresence(client *server.RealtimeClient) {
	channel := client.Channels.Get(ChannelName)
	pErr := channel.Presence.Enter(context.Background(), UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with enter presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func RealtimeLeavePresence(client *server.RealtimeClient) {
	channel := client.Channels.Get(ChannelName)
	pErr := channel.Presence.Leave(context.Background(), UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with leave presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func RealtimePublish(client *server.RealtimeClient, message string) {
	channel := client.Channels.Get(ChannelName)
	// Publish the message typed in to the Ably Channel
	err := channel.Publish(context.Background(), EventName, message)
	if err != nil {
		err := fmt.Errorf("error publishing to channel: %w", err)
		fmt.Println(err)
	}
}

func Jsonify(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}
