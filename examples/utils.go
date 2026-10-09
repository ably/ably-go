package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ably/ably-pubsub-go/server/pubsub"
)

func InitRealtimeClient() *pubsub.RealtimeClient {
	client, err := pubsub.NewRealtimeClient(
		pubsub.WithKey(os.Getenv(AblyKey)),
		// pubsub.WithEchoMessages(true), // Uncomment to stop messages you send from being sent back
		pubsub.WithClientID(UserName))
	if err != nil {
		panic(err)
	}
	return client
}

func RealtimeSubscribeToEvent(client *pubsub.RealtimeClient) func() {
	channel := client.Channels.Get(ChannelName)

	// Subscribe to messages sent on the channel
	unsubscribe, err := channel.Subscribe(context.Background(), EventName, func(msg *pubsub.Message) {
		fmt.Printf("Received message from %v: '%v'\n", msg.ClientID, msg.Data)
	})
	if err != nil {
		err := fmt.Errorf("error subscribing to channel: %w", err)
		fmt.Println(err)
	}
	return unsubscribe
}

func RealtimeEnterPresence(client *pubsub.RealtimeClient) {
	channel := client.Channels.Get(ChannelName)
	pErr := channel.Presence.Enter(context.Background(), UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with enter presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func RealtimeLeavePresence(client *pubsub.RealtimeClient) {
	channel := client.Channels.Get(ChannelName)
	pErr := channel.Presence.Leave(context.Background(), UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with leave presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func RealtimePublish(client *pubsub.RealtimeClient, message string) {
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
