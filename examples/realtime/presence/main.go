package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ably/ably-go/examples"
	"github.com/ably/ably-go/pubsub/server"
)

func main() {
	// Connect to Ably using the API key and ClientID specified
	client, err := server.NewRealtimeClient(
		server.WithKey(os.Getenv(examples.AblyKey)),
		server.WithClientID(examples.UserName))
	if err != nil {
		panic(err)
	}

	checkPresenceEnter(client)
	checkPresenceLeave(client)
	checkPresenceEnterAndLeave(client)
}

func checkPresenceEnter(client *server.RealtimeClient) {
	channel := client.Channels.Get(examples.ChannelName)
	unsubscribe := subscribePresenceEnter(channel)
	enterPresence(channel)
	time.Sleep(time.Second)
	unsubscribe()
}

func checkPresenceLeave(client *server.RealtimeClient) {
	channel := client.Channels.Get(examples.ChannelName)
	unsubscribe := subscribePresenceLeave(channel)
	leavePresence(channel)
	time.Sleep(time.Second)
	unsubscribe()
}

func checkPresenceEnterAndLeave(client *server.RealtimeClient) {
	channel := client.Channels.Get(examples.ChannelName)
	unsubscribe := subscribeAllPresence(channel)
	enterPresence(channel)
	printAllClientsOnChannel(channel)
	leavePresence(channel)
	time.Sleep(time.Second)
	unsubscribe()
}

func enterPresence(channel *server.RealtimeChannel) {
	pErr := channel.Presence.Enter(context.Background(), examples.UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with enter presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func enterOnBehalfOf(clientId string, channel *server.RealtimeChannel) {
	pErr := channel.Presence.EnterClient(context.Background(), clientId, examples.UserName+" entered the channel on behalf of "+clientId)
	if pErr != nil {
		err := fmt.Errorf("error with enter presence on behalf of other client on the channel %w", pErr)
		fmt.Println(err)
	}
}

func updatePresence(channel *server.RealtimeChannel) {
	pErr := channel.Presence.Update(context.Background(), examples.UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with update presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func leavePresence(channel *server.RealtimeChannel) {
	pErr := channel.Presence.Leave(context.Background(), examples.UserName+" entered the channel")
	if pErr != nil {
		err := fmt.Errorf("error with leave presence on the channel %w", pErr)
		fmt.Println(err)
	}
}

func subscribeAllPresence(channel *server.RealtimeChannel) func() {
	// Subscribe to presence events (people entering and leaving) on the channel
	unsubscribeAll, pErr := channel.Presence.SubscribeAll(context.Background(), func(msg *server.PresenceMessage) {
		if msg.Action == server.PresenceActionEnter {
			fmt.Printf("%v has entered the chat\n", msg.ClientID)
		} else if msg.Action == server.PresenceActionLeave {
			fmt.Printf("%v has left the chat\n", msg.ClientID)
		}
	})
	if pErr != nil {
		err := fmt.Errorf("error subscribing to presence in channel: %w", pErr)
		fmt.Println(err)

	}
	return unsubscribeAll
}

func subscribePresenceEnter(channel *server.RealtimeChannel) func() {
	// Subscribe to presence events entering the channel
	unsubscribe, pErr := channel.Presence.Subscribe(context.Background(), server.PresenceActionEnter, func(msg *server.PresenceMessage) {
		if msg.Action == server.PresenceActionEnter {
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

func subscribePresenceLeave(channel *server.RealtimeChannel) func() {
	// Subscribe to presence events leaving the channel
	unsubscribe, pErr := channel.Presence.Subscribe(context.Background(), server.PresenceActionLeave, func(msg *server.PresenceMessage) {
		if msg.Action == server.PresenceActionLeave {
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

func printAllClientsOnChannel(channel *server.RealtimeChannel) {
	clients, err := channel.Presence.Get(context.Background())
	if err != nil {
		panic(err)
	}
	for _, client := range clients {
		fmt.Println("Present client:", client)
	}
}
