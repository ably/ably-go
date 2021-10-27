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

	checkPresence(client)
}

func checkPresence(client *ably.REST) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	examples.RealtimeEnterPresence(realtimeClient)

	printPresenceMessages(channel)

	time.Sleep(time.Second)
	examples.RealtimeLeavePresence(realtimeClient)
	realtimeClient.Close()
}

func printPresenceMessages(channel *ably.RESTChannel) {

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
