package main

// go run presence.go constants.go

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Connect to Ably using the API key and ClientID specified
	client, err := ably.NewRealtime(
		ably.WithKey(os.Getenv(AblyKey)),
		ably.WithClientID(UserName))
	if err != nil {
		panic(err)
	}

	checkPresenceEnter(client)
	checkPresenceLeave(client)
	checkPresenceEnterAndLeave(client)
}

func checkPresenceEnter(client *ably.Realtime) {
	channel := client.Channels.Get(ChannelName)
	unsubscribe := subscribePresenceEnter(channel)
	enterPresence(channel)
	time.Sleep(time.Second)
	unsubscribe()
}

func checkPresenceLeave(client *ably.Realtime) {
	channel := client.Channels.Get(ChannelName)
	unsubscribe := subscribePresenceLeave(channel)
	leavePresence(channel)
	time.Sleep(time.Second)
	unsubscribe()
}

func checkPresenceEnterAndLeave(client *ably.Realtime) {
	channel := client.Channels.Get(ChannelName)
	unsubscribe := subscribeAllPresence(channel)
	enterPresence(channel)
	printAllClientsOnChannel(channel)
	leavePresence(channel)
	time.Sleep(time.Second)
	unsubscribe()
}

func enterPresence(channel *ably.RealtimeChannel) {
	pErr := channel.Presence.Enter(context.Background(), UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with enter presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func enterOnBehalfOf(clientId string, channel *ably.RealtimeChannel) {
	pErr := channel.Presence.EnterClient(context.Background(), clientId, UserName+" entered the channel on behalf of "+clientId)
	if pErr != nil {
		err := fmt.Errorf("error with enter presence on behalf of other client on the channel %w", pErr)
		fmt.Println(err)
	}
}

func updatePresence(channel *ably.RealtimeChannel) {
	pErr := channel.Presence.Update(context.Background(), UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with update presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func leavePresence(channel *ably.RealtimeChannel) {
	pErr := channel.Presence.Leave(context.Background(), UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with leave presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func subscribeAllPresence(channel *ably.RealtimeChannel) func() {
	// Subscribe to presence events (people entering and leaving) on the channel
	unsubscribeAll, pErr := channel.Presence.SubscribeAll(context.Background(), func(msg *ably.PresenceMessage) {
		if msg.Action == ably.PresenceActionEnter {
			fmt.Printf("%v has entered the chat\n", msg.ClientID)
		} else if msg.Action == ably.PresenceActionLeave {
			fmt.Printf("%v has left the chat\n", msg.ClientID)
		}
	})
	if pErr != nil {
		err := fmt.Errorf("error subscribing to presence in channel: %w", pErr)
		fmt.Println(err)

	}
	return unsubscribeAll
}

func subscribePresenceEnter(channel *ably.RealtimeChannel) func() {
	// Subscribe to presence events entering the channel
	unsubscribe, pErr := channel.Presence.Subscribe(context.Background(), ably.PresenceActionEnter, func(msg *ably.PresenceMessage) {
		if msg.Action == ably.PresenceActionEnter {
			fmt.Printf("%v has entered the chat\n", msg.ClientID)
		} else {
			panic("Not supposed to get presence related to actions other than presence enter")
		}
	})

	if pErr != nil {
		err := fmt.Errorf("error subscribing to enter presence in channel: %w", pErr)
		fmt.Println(err)
	}
	return unsubscribe
}

func subscribePresenceLeave(channel *ably.RealtimeChannel) func() {
	// Subscribe to presence events leaving the channel
	unsubscribe, pErr := channel.Presence.Subscribe(context.Background(), ably.PresenceActionLeave, func(msg *ably.PresenceMessage) {
		if msg.Action == ably.PresenceActionLeave {
			fmt.Printf("%v has left the chat\n", msg.ClientID)
		} else {
			panic("Not supposed to get presence related actions other than presence leave")
		}
	})
	if pErr != nil {
		err := fmt.Errorf("error subscribing to leave presence in channel: %w", pErr)
		fmt.Println(err)
	}
	return unsubscribe
}

func printAllClientsOnChannel(channel *ably.RealtimeChannel) {
	clients, err := channel.Presence.Get(context.Background())
	if err != nil {
		panic(err)
	}
	for _, client := range clients {
		fmt.Println("Present client:", client)
	}
}
