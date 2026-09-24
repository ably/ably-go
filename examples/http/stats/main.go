package main

import (
	"context"
	"fmt"
	"os"

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

	printApplicationStats(client)
}

func printApplicationStats(client *pubsub.HTTPClient) {
	pages, err := client.Stats().Pages(context.Background())
	if err != nil {
		panic(err)
	}

	for pages.Next(context.Background()) {
		for _, stat := range pages.Items() {
			fmt.Println(examples.Jsonify(stat))
		}
	}
	if err := pages.Err(); err != nil {
		panic(err)
	}
}
