package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"bitbucket.org/ably/ably-go"
)

func main() {
	id, secret := os.Getenv("ABLY_APP_ID"), os.Getenv("ABLY_APP_SECRET")
	if id == "" || secret == "" {
		log.Fatal("You must set ABLY_APP_ID and ABLY_APP_SECRET")
	}

	client := ably.RestClient(ably.Params{
		Endpoint:  "https://sandbox-rest.ably.io",
		AppID:     id,
		AppSecret: secret,
	})

	t, err := client.Time()
	if err != nil {
		log.Fatalln("Error fetching service time:", err)
	}
	fmt.Println("The Ably Service Time is:", t.Format(time.RFC1123))

	msgs := []*ably.Message{
		{Name: "event-1", Data: "foo"},
		{Name: "event-2", Data: "bar"},
		{Name: "event-3", Data: "baz"},
	}
	ch := client.Channel("my-channel")
	for i, msg := range msgs {
		log.Println("Publishing message:", msg.Name, "==>", msg.Data)
		if err := ch.Publish(msg); err != nil {
			log.Printf("Error publishing message %d: %s", i, err)
		}
	}

	log.Println("Waiting 5 seconds for messages to persist")
	time.Sleep(5 * time.Second)

	history, err := ch.History()
	if err != nil {
		log.Fatalln("Error fetching history:", err)
	}

	if len(history) == 0 {
		log.Fatal("No history :(")
	}

	log.Println("Channel history:")
	w := tabwriter.NewWriter(os.Stdout, 1, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "Name\tMessage")
	for _, msg := range history {
		fmt.Fprintf(w, "%s\t%s\n", msg.Name, msg.Data)
	}
}
