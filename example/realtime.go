// +build main2

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ably/ably-go"
	"github.com/ably/ably-go/realtime"
)

func main() {
	client := realtime.NewClient(ably.Params{
		RealtimeEndpoint: "wss://sandbox-realtime.ably.io:443",
		RestEndpoint:     "https://sandbox-rest.ably.io",
		AppID:            os.Getenv("ABLY_APP_ID"),
		AppSecret:        os.Getenv("ABLY_APP_SECRET"),
	})
	go func() {
		err := <-client.Err
		log.Fatal(err)
	}()

	ch := client.Channel("my-channel")
	done := make(chan struct{})
	go publish(ch, done)
	subscribe(ch, done)
}

func publish(ch *realtime.Channel, done chan struct{}) {
	for i := 0; i < 10; i++ {
		data := fmt.Sprintf("event-%d", i)
		log.Println("publishing", data)
		if err := ch.Publish("events", data); err != nil {
			log.Println("error publishing", data, ":", err)
		}
		time.Sleep(1 * time.Second)
	}
	close(done)
}

func subscribe(ch *realtime.Channel, done chan struct{}) {
	msgs := ch.Subscribe("")
	for {
		select {
		case msg := <-msgs:
			log.Println("got:", msg.Data)
		case <-done:
			return
		}
	}
}
