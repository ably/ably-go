//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func TestRSL1f1(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()
	opts := app.Options()
	// RSL1f
	opts = append(opts, ably.WithUseTokenAuth(false))
	client, err := ably.NewREST(opts...)
	assert.NoError(t, err)
	channel := client.Channels.Get("RSL1f")
	var msgs []*ably.Message
	size := 10
	for i := 0; i < size; i++ {
		msgs = append(msgs, &ably.Message{
			ClientID: "any_client_id",
			Data:     fmt.Sprint(i),
		})
	}
	err = channel.PublishMultiple(context.Background(), msgs)
	assert.NoError(t, err)
	var m []*ably.Message
	err = ablytest.AllPages(&m, channel.History())
	assert.NoError(t, err)
	assert.Equal(t, 10, len(m),
		"expected 10 messages got %d", len(m))
	for _, v := range m {
		assert.Equal(t, "any_client_id", v.ClientID,
			"expected clientId \"any_client_id\" got %s data:%v", v.ClientID, v.Data)
	}
}

func TestRSL1g(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	assert.NoError(t, err)
	defer app.Close()
	opts := append(app.Options(),
		ably.WithUseTokenAuth(true),
	)
	opts = append(opts, ably.WithClientID("some_client_id"))
	client, err := ably.NewREST(opts...)
	assert.NoError(t, err)
	t.Run("RSL1g1b", func(t *testing.T) {
		channel := client.Channels.Get("RSL1g1b")
		err := channel.PublishMultiple(context.Background(), []*ably.Message{
			{Name: "some 1"},
			{Name: "some 2"},
			{Name: "some 3"},
		})
		if err != nil {
			t.Fatal(err)
		}
		var history []*ably.Message
		err = ablytest.AllPages(&history, channel.History())
		assert.NoError(t, err)
		for _, m := range history {
			assert.Equal(t, "some_client_id", m.ClientID,
				"expected \"some_client_id\" got %s", m.ClientID)
		}
	})
	t.Run("RSL1g2", func(t *testing.T) {
		channel := client.Channels.Get("RSL1g2")
		err := channel.PublishMultiple(context.Background(), []*ably.Message{
			{Name: "1", ClientID: "some_client_id"},
			{Name: "2", ClientID: "some_client_id"},
			{Name: "3", ClientID: "some_client_id"},
		})
		if err != nil {
			t.Fatal(err)
		}
		var history []*ably.Message
		err = ablytest.AllPages(&history, channel.History())
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range history {
			assert.Equal(t, "some_client_id", m.ClientID,
				"expected \"some_client_id\" got %s", m.ClientID)
		}
	})
	t.Run("RSL1g3", func(t *testing.T) {
		channel := client.Channels.Get("RSL1g3")
		err := channel.PublishMultiple(context.Background(), []*ably.Message{
			{Name: "1", ClientID: "some_client_id"},
			{Name: "2", ClientID: "other client"},
			{Name: "3", ClientID: "some_client_id"},
		})
		assert.Error(t, err,
			"expected an error")
	})
}

func TestHistory_RSL2_RSL2b3(t *testing.T) {

	for _, limit := range []int{2, 3, 20} {
		t.Run(fmt.Sprintf("limit=%d", limit), func(t *testing.T) {
			app, rest := ablytest.NewREST()
			defer app.Close()
			channel := rest.Channels.Get("persisted:test")

			fixtures := historyFixtures()
			channel.PublishMultiple(context.Background(), fixtures)

			err := ablytest.TestPagination(
				reverseMessages(fixtures),
				channel.History(ably.HistoryWithLimit(limit)),
				limit,
				ablytest.PaginationWithEqual(messagesEqual),
			)
			assert.NoError(t, err)
		})
	}
}

/*
FAILING TEST
https://github.com/ably/ably-go/pull/383/checks?check_run_id=3486092179#step:7:792

=== CONT  TestRealtimeConn_Connect
--- PASS: TestImplicitNACK (0.01s)
--- FAIL: TestHistory_Direction_RSL2b2 (60.00s)
    --- FAIL: TestHistory_Direction_RSL2b2/direction=backwards (60.00s)
panic: Post "https://sandbox-rest.ably.io/apps": context deadline exceeded (Client.Timeout exceeded while awaiting headers) [recovered]
	panic: Post "https://sandbox-rest.ably.io/apps": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
goroutine 2621 [running]:

testing.tRunner.func1.1(0xd5a860, 0xc000e618f0)
	/opt/hostedtoolcache/go/1.15.15/x64/src/testing/testing.go:1072 +0x46a
testing.tRunner.func1(0xc000bdc600)
	/opt/hostedtoolcache/go/1.15.15/x64/src/testing/testing.go:1075 +0x636
panic(0xd5a860, 0xc000e618f0)
	/opt/hostedtoolcache/go/1.15.15/x64/src/runtime/panic.go:975 +0x47a
github.com/ably/ably-go/ablytest.MustSandbox(0x0, 0x12101a4)
	/home/runner/work/ably-go/ably-go/ablytest/sandbox.go:124 +0xd1
github.com/ably/ably-go/ablytest.NewREST(0x0, 0x0, 0x0, 0xf, 0x0)
	/home/runner/work/ably-go/ably-go/ablytest/sandbox.go:113 +0x52
github.com/ably/ably-go/ably_test.TestHistory_Direction_RSL2b2.func1(0xc000bdc600)
	/home/runner/work/ably-go/ably-go/ably/rest_channel_spec_test.go:172 +0x65
testing.tRunner(0xc000bdc600, 0xc000e60360)
	/opt/hostedtoolcache/go/1.15.15/x64/src/testing/testing.go:1123 +0x203
created by testing.(*T).Run
	/opt/hostedtoolcache/go/1.15.15/x64/src/testing/testing.go:1168 +0x5bc
FAIL	github.com/ably/ably-go/ably	284.413s
*/
func TestHistory_Direction_RSL2b2(t *testing.T) {
	t.Skip("FAILING TEST")

	for _, c := range []struct {
		direction ably.Direction
		expected  []*ably.Message
	}{
		{
			direction: ably.Backwards,
			expected:  reverseMessages(historyFixtures()),
		},
		{
			direction: ably.Forwards,
			expected:  historyFixtures(),
		},
	} {
		c := c
		t.Run(fmt.Sprintf("direction=%v", c.direction), func(t *testing.T) {
			app, rest := ablytest.NewREST()
			defer app.Close()
			channel := rest.Channels.Get("persisted:test")

			fixtures := historyFixtures()
			channel.PublishMultiple(context.Background(), fixtures)

			expected := c.expected

			err := ablytest.TestPagination(expected, channel.History(
				ably.HistoryWithLimit(len(expected)),
				ably.HistoryWithDirection(c.direction),
			), 100, ablytest.PaginationWithEqual(messagesEqual))
			assert.NoError(t, err)
		})
	}
}

func historyFixtures() []*ably.Message {
	var fixtures []*ably.Message
	for i := 0; i < 10; i++ {
		fixtures = append(fixtures, &ably.Message{Name: fmt.Sprintf("msg%d", i)})
	}
	return fixtures
}

func reverseMessages(msgs []*ably.Message) []*ably.Message {
	var reversed []*ably.Message
	for i := len(msgs) - 1; i >= 0; i-- {
		reversed = append(reversed, msgs[i])
	}
	return reversed
}

func messagesEqual(x, y interface{}) bool {
	mx, my := x.(*ably.Message), y.(*ably.Message)
	return mx.Name == my.Name && reflect.DeepEqual(mx.Data, my.Data)
}
