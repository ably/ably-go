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

	// Get channel
	channel := client.Channels.Get("channelName")
	// Get the channel status
	status, err := channel.Status(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Print(status, status.ChannelId)

}
