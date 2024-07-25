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

func SkipTestRealtimePresence_Presence_Enter_Update_Leave(t *testing.T) {
	app, client1 := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client1), app)

	client2 := app.NewRealtime(ably.WithClientID("client2"))
	defer safeclose(t, ablytest.FullRealtimeCloser(client2))

	err := ablytest.Wait(ablytest.ConnWaiter(client1, client1.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)
	err = ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	client1Channel := client1.Channels.Get("channel")
	err = client1Channel.Attach(context.Background())
	assert.NoError(t, err)

	client2Channel := client2.Channels.Get("channel")
	err = client2Channel.Attach(context.Background())
	assert.NoError(t, err)

	subCh1, unsub1, err := ablytest.ReceivePresenceMessages(client1Channel, nil)
	assert.NoError(t, err)
	defer unsub1()

	// ENTER
	err = client2Channel.Presence.Enter(context.Background(), "enter client2")
	assert.NoError(t, err)

	member_received := <-subCh1
	assert.Len(t, subCh1, 0) // Ensure no more updates received

	assert.Equal(t, ably.PresenceActionEnter, member_received.Action)
	assert.Equal(t, "client2", member_received.ClientID)
	assert.Equal(t, "enter client2", member_received.Data)

	// UPDATE
	err = client2Channel.Presence.Update(context.Background(), "update client2")
	assert.NoError(t, err)

	member_received = <-subCh1
	assert.Len(t, subCh1, 0) // Ensure no more updates received

	assert.Equal(t, ably.PresenceActionUpdate, member_received.Action)
	assert.Equal(t, "client2", member_received.ClientID)
	assert.Equal(t, "update client2", member_received.Data)

	// LEAVE
	err = client2Channel.Presence.Leave(context.Background(), "leave client2")
	assert.NoError(t, err)

	member_received = <-subCh1
	assert.Len(t, subCh1, 0) // Ensure no more updates received

	assert.Equal(t, ably.PresenceActionLeave, member_received.Action)
	assert.Equal(t, "client2", member_received.ClientID)
	assert.Equal(t, "leave client2", member_received.Data)
}

func SkipTestRealtimePresence_ServerSynthesized_Leave(t *testing.T) {
	app, client1 := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client1), app)

	client2 := app.NewRealtime(ably.WithClientID("client2"))
	defer safeclose(t, ablytest.FullRealtimeCloser(client2))

	err := ablytest.Wait(ablytest.ConnWaiter(client1, client1.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	err = ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	client1Channel := client1.Channels.Get("channel")
	err = client1Channel.Attach(context.Background())

	assert.NoError(t, err)

	client2Channel := client2.Channels.Get("channel")
	err = client2Channel.Attach(context.Background())
	assert.NoError(t, err)

	subCh1, unsub1, err := ablytest.ReceivePresenceMessages(client1Channel, nil)
	assert.NoError(t, err)
	defer unsub1()

	// ENTER
	err = client2Channel.Presence.Enter(context.Background(), "enter client2")
	assert.NoError(t, err)

	member_received := <-subCh1
	assert.Len(t, subCh1, 0) // Ensure no more updates received
	assert.Equal(t, ably.PresenceActionEnter, member_received.Action)
	assert.Equal(t, "client2", member_received.ClientID)
	assert.Equal(t, "enter client2", member_received.Data)

	members, err := client1Channel.Presence.Get(context.Background())
	assert.NoError(t, err)
	assert.Len(t, members, 1)

	// Server Synthesized Leave when client2 disconnects
	client2.Close()

	member_received = <-subCh1
	assert.Len(t, subCh1, 0) // Ensure no more updates received

	assert.Equal(t, ably.PresenceActionLeave, member_received.Action)
	assert.Equal(t, "client2", member_received.ClientID)

	// Make sure no members are present on the channel
	members, err = client1Channel.Presence.Get(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, members)
}

func TestRealtimePresence_ServerSynthesized_Leave(t *testing.T) {
	app, client1 := ablytest.NewRealtime(nil...)
	defer safeclose(t, ablytest.FullRealtimeCloser(client1), app)

	client2 := app.NewRealtime(ably.WithClientID("client2"))
	defer safeclose(t, ablytest.FullRealtimeCloser(client2))

	err := ablytest.Wait(ablytest.ConnWaiter(client1, client1.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	err = ablytest.Wait(ablytest.ConnWaiter(client2, client2.Connect, ably.ConnectionEventConnected), nil)
	assert.NoError(t, err)

	client1Channel := client1.Channels.Get("channel")
	err = client1Channel.Attach(context.Background())

	assert.NoError(t, err)

	client2Channel := client2.Channels.Get("channel")
	err = client2Channel.Attach(context.Background())
	assert.NoError(t, err)

	// ENTER
	err = client2Channel.Presence.Enter(context.Background(), "enter client2")
	assert.NoError(t, err)

	leaveAction := ably.PresenceActionLeave
	subCh1, unsub1, err := ablytest.ReceivePresenceMessages(client1Channel, &leaveAction)
	assert.NoError(t, err)
	defer unsub1()

	// Server Synthesized Leave when client2 disconnects
	client2.Close()

	member_received := <-subCh1
	assert.Len(t, subCh1, 0) // Ensure no more updates received

	assert.Equal(t, ably.PresenceActionLeave, member_received.Action)
	assert.Equal(t, "client2", member_received.ClientID)
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
