package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ably/ably-go/examples"
	"github.com/ably/ably-go/pubsub/server"
)

func main() {
	// Connect to Ably using the API key and ClientID
	client, err := server.NewHTTPClient(
		server.WithKey(os.Getenv(examples.AblyKey)),
		server.WithClientID(examples.UserName))
	if err != nil {
		panic(err)
	}

	printApplicationStats(client)
}

func printApplicationStats(client *server.HTTPClient) {
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
