package ably_test

import (
	"fmt"

	"context"
	"github.com/ably/ably-go/ably"
)

func Example_paginatedResults() {
	ctx := context.Background()
	client, err := ably.NewRealtime(ably.WithKey("xxx:xxx"))
	if err != nil {
		panic(err)
	}
	channel := client.Channels.Get("persisted:test")

	err = channel.Publish(ctx, "EventName1", "EventData1")
	if err != nil {
		panic(err)
	}

	pages, err := channel.History().Pages(ctx)
	if err != nil {
		panic(err)
	}

	// Returning and iterating over the first page
	pages.Next(ctx)
	pages.First(ctx)
	for _, message := range pages.Items() {
		fmt.Println(message)
	}

	// Iteration over pages in PaginatedResult
	for pages.Next(ctx) {
		fmt.Println(pages.HasNext(ctx))
		fmt.Println(pages.IsLast(ctx))
		for _, presence := range pages.Items() {
			fmt.Println(presence)
		}
	}
	if err := pages.Err(); err != nil {
		panic(err)
	}
}
