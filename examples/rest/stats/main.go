package main

import (
	"context"
	"fmt"
	"os"

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

	printApplicationStats(client)
}

func printApplicationStats(client *ably.REST) {
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
