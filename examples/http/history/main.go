package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ably/ably-pubsub-go/examples"
	"github.com/ably/ably-pubsub-go/server/pubsub"
)

func main() {
	// Connect to Ably using the API key and ClientID
	client, err := pubsub.NewHTTPClient(
		pubsub.WithKey(os.Getenv(examples.AblyKey)),
		pubsub.WithClientID(examples.UserName))

	if err != nil {
		panic(err)
	}

	checkHTTPChannelMessageHistory(client)
	checkHTTPChannelPresenceHistory(client)
}

func checkHTTPChannelMessageHistory(client *pubsub.HTTPClient) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	examples.RealtimePublish(realtimeClient, "Hey there!")
	examples.RealtimePublish(realtimeClient, "How are you")

	time.Sleep(time.Second)
	printChannelMessageHistory(channel)
	time.Sleep(time.Second)
	realtimeClient.Close()
}

func checkHTTPChannelPresenceHistory(client *pubsub.HTTPClient) {
	channel := client.Channels.Get(examples.ChannelName)
	realtimeClient := examples.InitRealtimeClient()
	examples.RealtimeEnterPresence(realtimeClient)

	time.Sleep(time.Second)
	printChannelPresenceHistory(channel)
	time.Sleep(time.Second)
	realtimeClient.Close()
}

func printChannelMessageHistory(channel *pubsub.HTTPChannel) {
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

func printChannelPresenceHistory(channel *pubsub.HTTPChannel) {
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
