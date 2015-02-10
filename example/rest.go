// +build main2

package main

import (
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/ably/ably-go"
	"github.com/ably/ably-go/rest"
)

func main() {
	id, secret := os.Getenv("ABLY_APP_ID"), os.Getenv("ABLY_APP_SECRET")
	if id == "" || secret == "" {
		log.Fatal("You must set ABLY_APP_ID and ABLY_APP_SECRET")
	}

	client := rest.NewClient(ably.Params{
		RestEndpoint: "https://sandbox-rest.ably.io",
		AppID:        id,
		AppSecret:    secret,
	})

	t, err := client.Time()
	if err != nil {
		log.Fatalln("Error fetching service time:", err)
	}
	fmt.Println("The Ably Service Time is:", t.Format(time.RFC1123))

	msgs := map[string]interface{}{
		"event-1": "foo",
		"event-2": 42,
		"event-3": map[string]int{"bar": 2},
	}
	ch := client.Channel("my-channel")
	for name, data := range msgs {
		log.Println("Publishing message:", name, "==>", data)
		if err := ch.Publish(name, data); err != nil {
			log.Fatalf("Error publishing %s: %s", name, err)
		}
	}

	log.Println("Waiting 10 seconds for messages to persist")
	time.Sleep(10 * time.Second)

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
		fmt.Fprintf(w, "%s\t%v\n", msg.Name, msg.Data)
	}
}
