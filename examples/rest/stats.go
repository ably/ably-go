package main

//go run stats.go utils.go constants.go

import (
	"fmt"
	"os"

	"github.com/ably/ably-go/ably"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Connect to Ably using the API key and ClientID specified above
	client, err := ably.NewREST(
		ably.WithKey(os.Getenv(AblyKey)),
		// ably.WithEchoMessages(true), // Uncomment to stop messages you send from being sent back
		ably.WithClientID(UserName))
	if err != nil {
		panic(err)
	}

	printApplicationStats(client)
}

func printApplicationStats(client *ably.REST) {
	page, err := client.Stats(&ably.PaginateParams{})
	for ; err == nil && page != nil; page, err = page.Next() {
		for _, stat := range page.Stats() {
			fmt.Println(jsonify(stat))
		}
	}
	if err != nil {
		err := fmt.Errorf("error getting application stats %w", err)
		panic(err)
	}
}
