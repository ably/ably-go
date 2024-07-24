//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func contains(members []*ably.PresenceMessage, clients ...string) error {
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
	app, client := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	members, err := client.Channels.Get("persisted:presence_fixtures").Presence.Get(context.Background())
	assert.NoError(t, err)

	err = contains(members, fixtureMembers...)
	assert.NoError(t, err)
}

func TestRealtimePresence_Sync250_RTP4(t *testing.T) {
	app, client1 := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client1), app)
	client2 := app.NewRealtime(nil...)
	client3 := app.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client2), ablytest.FullRealtimeCloser(client3))
	err := ablytest.Wait(ablytest.ConnWaiter(client1, client1.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	err = ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	err = ablytest.Wait(ablytest.ConnWaiter(client3, client3.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	channel1 := client1.Channels.Get("sync250")
	err = channel1.Attach(context.Background())
	assert.NoError(t, err)
	channel2 := client2.Channels.Get("sync250")
	err = channel2.Attach(context.Background())
	assert.NoError(t, err)

	sub2, unsub2, err := ablytest.ReceivePresenceMessages(channel2, nil)
	assert.NoError(t, err,
		"channel2.ablytest.ReceiveMessages(Presence)=%v", err)
	defer unsub2()

	var rg ablytest.ResultGroup
	var clients = generateClients(250)
	for _, client := range clients {
		client := client
		rg.GoAdd(func(ctx context.Context) error { return channel1.Presence.EnterClient(ctx, client, "") })
	}
	err = rg.Wait()
	assert.NoError(t, err,
		"rg.Wait()=%v", err)

	members2 := make([]*ably.PresenceMessage, 0)
	tout := time.After(250 * ablytest.Timeout)
	client_ids := make(map[string]struct{})

	for len(client_ids) < 250 {
		select {
		case msg := <-sub2:
			members2 = append(members2, msg)
			client_ids[msg.ClientID] = struct{}{}
		case <-tout:
			t.Fatalf("waiting for presence messages timed out after %v", 250*ablytest.Timeout)
		}
	}

	err = contains(members2, clients...)
	assert.NoError(t, err,
		"members2: %v", err)
	members3, err := client3.Channels.Get("sync250").Presence.Get(context.Background())
	assert.NoError(t, err)
	err = contains(members3, clients...)
	assert.NoError(t, err,
		"members3: %v", err)
}

func TestRealtimePresence_EnsureChannelIsAttached(t *testing.T) {

	var rec ablytest.ChanStatesRecorder
	opts := []ably.ClientOption{
		ably.WithAutoConnect(false),
	}
	app, client := ablytest.NewRealtime(opts...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client), app)
	channel := client.Channels.Get("persisted:presence_fixtures")
	off := rec.Listen(channel)
	defer off()
	err := ablytest.Wait(ablytest.ConnWaiter(client, client.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	members, err := channel.Presence.Get(context.Background())
	assert.NoError(t, err)
	expectedTransitions := []ably.ChannelState{
		ably.ChannelStateInitialized,
		ably.ChannelStateAttaching,
		ably.ChannelStateAttached,
	}
	assert.Equal(t, expectedTransitions, rec.States(),
		"expected %+v, got %+v", expectedTransitions, rec.States())
	err = contains(members, fixtureMembers...)
	assert.NoError(t, err)
}

func TestRealtimePresence_Presence_Enter_Update_Leave(t *testing.T) {
	app, client1 := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client1), app)

	client2 := app.NewRealtime(ably.WithClientID("client2"))
	defer safeclose(t, ablytest.FullRealtimeCloser(client2))

	err := ablytest.Wait(ablytest.ConnWaiter(client1, client1.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	err = ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	channel1 := client1.Channels.Get("channel")
	err = channel1.Attach(context.Background())
	assert.NoError(t, err)

	channel2 := client2.Channels.Get("channel")
	err = channel2.Attach(context.Background())
	assert.NoError(t, err)

	subCh1, unsub1, err := ablytest.ReceivePresenceMessages(channel1, nil)
	assert.NoError(t, err)
	defer unsub1()

	// ENTER
	err = channel2.Presence.Enter(context.Background(), "enter client2")
	assert.NoError(t, err)

	member_received := <-subCh1
	assert.Len(t, subCh1, 0) // Ensure no more updates received

	assert.Equal(t, member_received.Action, ably.PresenceActionEnter)
	assert.Equal(t, member_received.ClientID, "client2")
	assert.Equal(t, member_received.Data, "enter client2")

	// UPDATE
	err = channel2.Presence.Update(context.Background(), "update client2")
	assert.NoError(t, err)

	member_received = <-subCh1
	assert.Len(t, subCh1, 0) // Ensure no more updates received

	assert.Equal(t, member_received.Action, ably.PresenceActionUpdate)
	assert.Equal(t, member_received.ClientID, "client2")
	assert.Equal(t, member_received.Data, "update client2")

	// LEAVE
	err = channel2.Presence.Leave(context.Background(), "leave client2")
	assert.NoError(t, err)

	member_received = <-subCh1
	assert.Len(t, subCh1, 0) // Ensure no more updates received

	assert.Equal(t, member_received.Action, ably.PresenceActionLeave)
	assert.Equal(t, member_received.ClientID, "client2")
	assert.Equal(t, member_received.Data, "leave client2")
}

func TestRealtimePresence_Server_Synthesized_Leave(t *testing.T) {

}

// When a client is created with a ClientID, Enter is used to announce the client's presence.
// This example shows Client A entering their presence.
func ExampleRealtimePresence_Enter() {

	// A new realtime client is created with a ClientID.
	client, err := ably.NewRealtime(
		ably.WithKey("ABLY_PRIVATE_KEY"),
		ably.WithClientID("Client A"),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	// A new channel is initialised.
	channel := client.Channels.Get("chat")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// The client announces presence with Enter.
	if err := channel.Presence.Enter(ctx, nil); err != nil {
		fmt.Println(err)
		return
	}
}

// When a client is created without a ClientID, EnterClient is used to announce the presence of a client.
// This example shows a client without a clientID announcing the presence of "Client A" using EnterClient.
func ExampleRealtimePresence_EnterClient() {

	// A new realtime client is created without providing a ClientID.
	client, err := ably.NewRealtime(
		ably.WithKey("ABLY_PRIVATE_KEY"),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	// A new channel is initialised.
	channel := client.Channels.Get("chat")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// The presence of Client A is announced using EnterClient.
	if err := channel.Presence.EnterClient(ctx, "Client A", nil); err != nil {
		fmt.Println(err)
		return
	}
}
