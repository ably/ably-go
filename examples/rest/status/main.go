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

	// Get channel
	channel := client.Channels.Get("channelName")
	// Get the channel status
	status, err := channel.Status(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Print(status, status.ChannelId)

}
