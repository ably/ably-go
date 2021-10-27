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

	checkRestChannelMessageHistory(client)
	checkRestChannelPresenceHistory(client)
}

func checkRestChannelMessageHistory(client *ably.REST) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	examples.RealtimePublish(realtimeClient, "Hey there!")
	examples.RealtimePublish(realtimeClient, "How are you")

	time.Sleep(time.Second)
	printChannelMessageHistory(channel)
	time.Sleep(time.Second)
	realtimeClient.Close()
}

func checkRestChannelPresenceHistory(client *ably.REST) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	examples.RealtimeEnterPresence(realtimeClient)

	time.Sleep(time.Second)
	printChannelPresenceHistory(channel)
	time.Sleep(time.Second)
	realtimeClient.Close()
}

func printChannelMessageHistory(channel *ably.RESTChannel) {
	pages, err := channel.History().Pages(context.Background())
	if err != nil {
		panic(err)
	}
	for pages.Next(context.Background()) {
		for _, message := range pages.Items() {
			fmt.Println("--- Channel history ---")
			fmt.Println(examples.Jsonify(message))
			fmt.Println("--------")
		}
	}
	if err := pages.Err(); err != nil {
		panic(err)
	}
}

func printChannelPresenceHistory(channel *ably.RESTChannel) {
	pages, err := channel.Presence.History().Pages(context.Background())
	if err != nil {
		panic(err)
	}
	for pages.Next(context.Background()) {
		for _, presence := range pages.Items() {
			fmt.Println("--- Channel presence history ---")
			fmt.Println(examples.Jsonify(presence))
			fmt.Println("----------")
		}
	}
	if err := pages.Err(); err != nil {
		panic(err)
	}
}
