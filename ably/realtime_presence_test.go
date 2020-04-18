package ably_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/proto"
)

func contains(members []*proto.PresenceMessage, clients ...string) error {
	lookup := make(map[string]struct{}, len(members))
	for _, member := range members {
		lookup[member.ClientID] = struct{}{}
	}
	for _, client := range clients {
		if _, ok := lookup[client]; !ok {
			return fmt.Errorf("clientID=%q not found in presence map", client)
		}
	}
	return nil
}

func generateClients(num int) []string {
	clients := make([]string, 0, num)
	for i := 0; i < num; i++ {
		clients = append(clients, "client"+strconv.Itoa(i))
	}
	return clients
}

var fixtureMembers = []string{
	"client_bool",
	"client_int",
	"client_string",
	"client_json",
}

func TestRealtimePresence_Sync(t *testing.T) {
	t.Parallel()
	app, client := ablytest.NewRealtime(nil)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)

	members, err := client.Channels.GetAndAttach("persisted:presence_fixtures").Presence.Get(true)
	if err != nil {
		t.Fatal(err)
	}
	if err = contains(members, fixtureMembers...); err != nil {
		t.Fatal(err)
	}
}

func TestRealtimePresence_Sync250(t *testing.T) {
	t.Parallel()
	app, client1 := ablytest.NewRealtime(nil)
	defer safeclose(t, ablytest.FullRealtimeCloser(client1), app)
	client2 := app.NewRealtime()
	client3 := app.NewRealtime()
	defer safeclose(t, ablytest.FullRealtimeCloser(client2), ablytest.FullRealtimeCloser(client3))

	channel1 := client1.Channels.GetAndAttach("sync250")
	channel2 := client2.Channels.GetAndAttach("sync250")

	sub2, err := channel2.Presence.Subscribe()
	if err != nil {
		t.Fatalf("channel2.Presence.Subscribe()=%v", err)
	}
	defer safeclose(t, sub2)

	var rg ablytest.ResultGroup
	var clients = generateClients(250)
	for _, client := range clients {
		rg.Add(channel1.Presence.EnterClient(client, ""))
	}
	if err := rg.Wait(); err != nil {
		t.Fatalf("rg.Wait()=%v", err)
	}
	members2 := make([]*proto.PresenceMessage, 250)
	tout := time.After(250 * ablytest.Timeout)

	for i := range members2 {
		select {
		case msg := <-sub2.PresenceChannel():
			members2[i] = msg
		case <-tout:
			t.Fatalf("waiting for presence messages timed out after %v", 250*ablytest.Timeout)
		}
	}

	if err = contains(members2, clients...); err != nil {
		t.Fatalf("members2: %v", err)
	}
	members3, err := client3.Channels.GetAndAttach("sync250").Presence.Get(true)
	if err != nil {
		t.Fatal(err)
	}
	if err = contains(members3, clients...); err != nil {
		t.Fatalf("members3: %v", err)
	}
}

func TestRealtimePresence_EnsureChannelIsAttached(t *testing.T) {
	t.Parallel()
	presTransitions := []ably.StateEnum{
		ably.StateConnConnecting,
		ably.StateConnConnected,
		ably.StateChanAttaching,
		ably.StateChanAttached,
	}
	rec := ablytest.NewStateRecorder(4)
	opts := &ably.ClientOptions{
		Listener:  rec.Channel(),
		NoConnect: true,
	}
	app, client := ablytest.NewRealtime(opts)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	channel := client.Channels.Get("persisted:presence_fixtures")
	if err := ablytest.Wait(client.Connection.Connect()); err != nil {
		t.Fatal(err)
	}
	if err := rec.WaitFor(presTransitions[:2]); err != nil {
		t.Fatal(err)
	}
	members, err := channel.Presence.Get(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := rec.WaitFor(presTransitions); err != nil {
		t.Fatal(err)
	}
	rec.Stop()
	if err = contains(members, fixtureMembers...); err != nil {
		t.Fatal(err)
	}
}
