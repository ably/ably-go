package main

// go run presence.go constants.go utils.go

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Connect to Ably using the API key and ClientID
	client, err := ably.NewREST(
		ably.WithKey(os.Getenv(AblyKey)),
		ably.WithClientID(UserName))

	if err != nil {
		panic(err)
	}

	checkPresence(client)
}

func checkPresence(client *ably.REST) {
	channel := client.Channels.Get(ChannelName)
	realtimeClient := initRealtimeClient()
	realtimeEnterPresence(realtimeClient)

	printPresenceMessages(channel)

	time.Sleep(time.Second)
	realtimeLeavePresence(realtimeClient)
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
			fmt.Println(jsonify(presence))
			fmt.Println("----------")
		}
	}
	if err := pages.Err(); err != nil {
		panic(err)
	}
}
