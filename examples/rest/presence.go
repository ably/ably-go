package main

// go run presence.go constants.go utils.go

import (
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
	page, err := channel.Presence.Get(nil)
	for ; err == nil && page != nil; page, err = page.Next() {
		for _, presence := range page.PresenceMessages() {
			fmt.Println(jsonify(presence))
		}
	}
	if err != nil {
		err := fmt.Errorf("error getting presence on the channel: %w", err)
		panic(err)
	}
}
