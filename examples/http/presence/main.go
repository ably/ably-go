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

	checkPresence(client)
}

func checkPresence(client *pubsub.HTTPClient) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	examples.RealtimeEnterPresence(realtimeClient)

	printPresenceMessages(channel)

	time.Sleep(time.Second)
	examples.RealtimeLeavePresence(realtimeClient)
	realtimeClient.Close()
}

func printPresenceMessages(channel *pubsub.HTTPChannel) {

	pages, err := channel.Presence.Get().Pages(context.Background())
	if err != nil {
		panic(err)
	}
	for pages.Next(context.Background()) {
		for _, presence := range pages.Items() {
			fmt.Println("--- Channel presence ---")
			fmt.Println(examples.Jsonify(presence))
			fmt.Println("----------")
		}
	}
	if err := pages.Err(); err != nil {
		panic(err)
	}
}
