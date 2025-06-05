package liveobjects

import (
	"context"
	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/objects"
	"log"
	"slices"
	"testing"
	"time"
)

var (
	apiKey = "add-key"
)

func TestCreateAndIncrementCounterRealtime(_ *testing.T) {
	ctx := context.Background()
	channelName := "test_channel_counters"

	liveObjectsPlugin := &loPlugin{
		syncMessagesChan:   make(chan *objects.Message),
		objectMessagesChan: make(chan *objects.Message),
		done:               make(chan struct{}),
	}

	go liveObjectsPlugin.listen(ctx)

	client, err := getRealtimeClient(apiKey, liveObjectsPlugin)
	if err != nil {
		log.Fatal("Failed to create Ably client:", err)
	}

	createCounterOp1, err := createCounterOp(5)
	if err != nil {
		log.Fatal("Failed to create counter operation:", err)
	}

	channel := client.Channels.Get(channelName, ably.ChannelWithModes(ably.ChannelModeObjectSubscribe, ably.ChannelModeObjectPublish))
	err = channel.Attach(context.Background())
	if err != nil {
		log.Fatal("Failed to attach channel:", err)
	}

	if err := channel.ExperimentalObjects.PublishObjects(ctx, createCounterOp1); err != nil {
		log.Fatal("Failed to publish counter operation:", err)
	}

	inCounterOp1 := incCounterOp(createCounterOp1.Operation.ObjectID, 6)
	if err := channel.ExperimentalObjects.PublishObjects(ctx, inCounterOp1); err != nil {
		log.Fatal("Failed to publish counter operation:", err)
	}

	err = liveObjectsPlugin.EventuallyObjectMessages(func(stateMessages []*objects.Message) bool {
		containsCounter := slices.ContainsFunc(stateMessages, func(msg *objects.Message) bool {
			return msg.Operation.Action == objects.Operation_CounterCreate && msg.Operation.ObjectID == createCounterOp1.Operation.ObjectID && msg.ID == createCounterOp1.ID
		})

		containsInc := slices.ContainsFunc(stateMessages, func(msg *objects.Message) bool {
			return msg.Operation.Action == objects.Operation_CounterInc && msg.Operation.ObjectID == inCounterOp1.Operation.ObjectID && msg.ID == inCounterOp1.ID
		})

		return containsCounter && containsInc
	}, 15*time.Second, 200*time.Millisecond)
	if err != nil {
		log.Fatal("Failed to receive expected object messages:", err)
	}
}

func TestCreateMapsAndSetOnRoot(_ *testing.T) {
	// create multiple maps via rest
	// get the map ID and publish a root set command via realtime
	// assert all message are received.

	ctx := context.Background()
	channelName := "test_channel_maps"

	liveObjectsPlugin := &loPlugin{
		syncMessagesChan:   make(chan *objects.Message),
		objectMessagesChan: make(chan *objects.Message),
		done:               make(chan struct{}),
	}

	go liveObjectsPlugin.listen(ctx)

	client, err := getRealtimeClient(apiKey, liveObjectsPlugin)
	if err != nil {
		log.Fatal("Failed to create Ably client:", err)
	}

	channel := client.Channels.Get(channelName, ably.ChannelWithModes(ably.ChannelModeObjectSubscribe, ably.ChannelModeObjectPublish))
	err = channel.Attach(context.Background())
	if err != nil {
		log.Fatal("Failed to attach channel:", err)
	}

	map1ID, err := restCreateMap(apiKey, channelName)
	if err != nil {
		log.Fatal("Failed to create map:", err)
	}

	map2ID, err := restCreateMap(apiKey, channelName)
	if err != nil {
		log.Fatal("Failed to create map:", err)
	}

	map3ID, err := restCreateMap(apiKey, channelName)
	if err != nil {
		log.Fatal("Failed to create map:", err)
	}

	setOnRootMap1 := setOnRootOp("new-map-1", map1ID)
	if err := channel.ExperimentalObjects.PublishObjects(ctx, setOnRootMap1); err != nil {
		log.Fatal("Failed to publish root operation:", err)
	}

	if err := channel.ExperimentalObjects.PublishObjects(ctx, setOnRxootOp("new-map-2", map2ID)); err != nil {
		log.Fatal("Failed to publish root operation:", err)
	}

	if err := channel.ExperimentalObjects.PublishObjects(ctx, setOnRootOp("new-map-3", map3ID)); err != nil {
		log.Fatal("Failed to publish root operation:", err)
	}

	err = liveObjectsPlugin.EventuallyObjectMessages(func(stateMessages []*objects.Message) bool {
		containsMapOp1 := slices.ContainsFunc(stateMessages, func(msg *objects.Message) bool {
			return msg.Operation.Action == objects.Operation_MapCreate && msg.Operation.ObjectID == map1ID
		})
		containsRootOp1 := slices.ContainsFunc(stateMessages, func(msg *objects.Message) bool {
			return msg.Operation.Action == objects.Operation_MapSet && msg.Operation.ObjectID == "root" && msg.Operation.MapOp.Key == "new-map-1" && msg.ID == setOnRootMap1.ID
		})
		containsMapOp2 := slices.ContainsFunc(stateMessages, func(msg *objects.Message) bool {
			return msg.Operation.Action == objects.Operation_MapCreate && msg.Operation.ObjectID == map2ID
		})
		containsRootOp2 := slices.ContainsFunc(stateMessages, func(msg *objects.Message) bool {
			return msg.Operation.Action == objects.Operation_MapSet && msg.Operation.ObjectID == "root" && msg.Operation.MapOp.Key == "new-map-2"
		})
		containsMapOp3 := slices.ContainsFunc(stateMessages, func(msg *objects.Message) bool {
			return msg.Operation.Action == objects.Operation_MapCreate && msg.Operation.ObjectID == map3ID
		})
		containsRootOp3 := slices.ContainsFunc(stateMessages, func(msg *objects.Message) bool {
			return msg.Operation.Action == objects.Operation_MapSet && msg.Operation.ObjectID == "root" && msg.Operation.MapOp.Key == "new-map-3"
		})

		return containsMapOp1 &&
			containsRootOp1 &&
			containsMapOp2 &&
			containsRootOp2 &&
			containsMapOp3 &&
			containsRootOp3
	}, 15*time.Second, 200*time.Millisecond)
	if err != nil {
		log.Fatal("Failed to receive expected object messages:", err)
	}

	liveObjectsPlugin.close()
	log.Println("Done")
}
