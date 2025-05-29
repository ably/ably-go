package objects

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockChannel implements the Channel interface for testing
type mockChannel struct {
	name     string
	messages []*Message
	mu       sync.Mutex
}

func (m *mockChannel) PublishObjects(ctx context.Context, msgs ...*Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msgs...)
	return nil
}

func (m *mockChannel) getMessages() []*Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*Message, len(m.messages))
	copy(result, m.messages)
	return result
}

func TestLiveMap_BasicOperations(t *testing.T) {
	channel := &mockChannel{name: "test"}
	liveObjects := NewLiveObjects(channel)
	liveMap := liveObjects.Map("map1")

	ctx := context.Background()

	// Test Set operation
	err := liveMap.Set(ctx, "key1", "value1")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test Get operation
	value, exists := liveMap.Get("key1")
	if !exists {
		t.Fatal("Key should exist")
	}
	if value != "value1" {
		t.Fatalf("Expected 'value1', got %v", value)
	}

	// Test Size
	if liveMap.Size() != 1 {
		t.Fatalf("Expected size 1, got %d", liveMap.Size())
	}

	// Test Keys
	keys := liveMap.Keys()
	if len(keys) != 1 || keys[0] != "key1" {
		t.Fatalf("Expected ['key1'], got %v", keys)
	}

	// Verify messages were sent
	messages := channel.getMessages()
	if len(messages) != 2 { // CREATE + SET
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	// Check CREATE message
	createMsg := messages[0]
	if createMsg.Operation.Action != Operation_MapCreate {
		t.Fatalf("Expected MAP_CREATE, got %v", createMsg.Operation.Action)
	}

	// Check SET message
	setMsg := messages[1]
	if setMsg.Operation.Action != Operation_MapSet {
		t.Fatalf("Expected MAP_SET, got %v", setMsg.Operation.Action)
	}
	if setMsg.Operation.MapOp.Key != "key1" {
		t.Fatalf("Expected key 'key1', got %v", setMsg.Operation.MapOp.Key)
	}
}

func TestLiveMap_Remove(t *testing.T) {
	channel := &mockChannel{name: "test"}
	liveObjects := NewLiveObjects(channel)
	liveMap := liveObjects.Map("map1")

	ctx := context.Background()

	// Set initial value
	err := liveMap.Set(ctx, "key1", "value1")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Remove the key
	err = liveMap.Remove(ctx, "key1")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify key is removed
	_, exists := liveMap.Get("key1")
	if exists {
		t.Fatal("Key should not exist after removal")
	}

	if liveMap.Size() != 0 {
		t.Fatalf("Expected size 0, got %d", liveMap.Size())
	}
}

func TestLiveMap_EventListeners(t *testing.T) {
	channel := &mockChannel{name: "test"}
	liveObjects := NewLiveObjects(channel)
	liveMap := liveObjects.Map("map1")

	ctx := context.Background()

	// Set up event listener
	eventReceived := make(chan MapEvent, 1)
	liveMap.Subscribe("key1", func(event MapEvent) {
		eventReceived <- event
	})

	// Set a value
	err := liveMap.Set(ctx, "key1", "value1")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Wait for event
	select {
	case event := <-eventReceived:
		if event.Type != MapEventSet {
			t.Fatalf("Expected SET event, got %v", event.Type)
		}
		if event.Key != "key1" {
			t.Fatalf("Expected key 'key1', got %v", event.Key)
		}
		if event.Value != "value1" {
			t.Fatalf("Expected value 'value1', got %v", event.Value)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Event not received")
	}
}

func TestLiveCounter_BasicOperations(t *testing.T) {
	channel := &mockChannel{name: "test"}
	liveObjects := NewLiveObjects(channel)
	liveCounter := liveObjects.Counter("counter1")

	ctx := context.Background()

	// Test Increment
	err := liveCounter.Increment(ctx, 5.0)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}

	// Test Value
	if liveCounter.Value() != 5.0 {
		t.Fatalf("Expected value 5.0, got %v", liveCounter.Value())
	}

	// Test Decrement
	err = liveCounter.Decrement(ctx, 2.0)
	if err != nil {
		t.Fatalf("Decrement failed: %v", err)
	}

	if liveCounter.Value() != 3.0 {
		t.Fatalf("Expected value 3.0, got %v", liveCounter.Value())
	}

	// Verify messages were sent
	messages := channel.getMessages()
	if len(messages) != 2 { // CREATE + INCREMENT
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	// Check CREATE message
	createMsg := messages[0]
	if createMsg.Operation.Action != Operation_CounterCreate {
		t.Fatalf("Expected COUNTER_CREATE, got %v", createMsg.Operation.Action)
	}

	// Check INCREMENT message
	incMsg := messages[1]
	if incMsg.Operation.Action != Operation_CounterInc {
		t.Fatalf("Expected COUNTER_INC, got %v", incMsg.Operation.Action)
	}
	if incMsg.Operation.CounterOp.Amount != -2.0 {
		t.Fatalf("Expected amount -2.0, got %v", incMsg.Operation.CounterOp.Amount)
	}
}

func TestLiveCounter_EventListeners(t *testing.T) {
	channel := &mockChannel{name: "test"}
	liveObjects := NewLiveObjects(channel)
	liveCounter := liveObjects.Counter("counter1")

	ctx := context.Background()

	// Create the counter first
	err := liveCounter.Increment(ctx, 5.0)
	if err != nil {
		t.Fatalf("Initial increment failed: %v", err)
	}

	// Set up event listener
	eventReceived := make(chan CounterEvent, 1)
	liveCounter.Subscribe(func(event CounterEvent) {
		eventReceived <- event
	})

	// Increment counter again to trigger event
	err = liveCounter.Increment(ctx, 10.0)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}

	// Wait for event
	select {
	case event := <-eventReceived:
		if event.Value != 15.0 {
			t.Fatalf("Expected value 15.0, got %v", event.Value)
		}
		if event.Previous != 5.0 {
			t.Fatalf("Expected previous 5.0, got %v", event.Previous)
		}
		if event.Delta != 10.0 {
			t.Fatalf("Expected delta 10.0, got %v", event.Delta)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Event not received")
	}
}

func TestStandardPlugin_HandleObjectMessages(t *testing.T) {
	channel := &mockChannel{name: "test"}
	liveObjects := NewLiveObjects(channel)
	plugin := liveObjects.GetPlugin()

	// Create a map in LiveObjects
	liveMap := liveObjects.Map("map1")
	liveMap.created = true // Mark as created to simulate incoming messages

	// Create a SET operation message
	setMsg := &Message{
		Operation: &Operation{
			Action:   Operation_MapSet,
			ObjectID: "map1",
			MapOp: &MapOp{
				Key: "key1",
				Data: &Data{
					String: stringPtr("value1"),
				},
			},
		},
	}

	// Handle the message
	plugin.HandleObjectMessages([]*Message{setMsg})

	// Verify the value was set
	value, exists := liveMap.Get("key1")
	if !exists {
		t.Fatal("Key should exist after handling message")
	}
	if value != "value1" {
		t.Fatalf("Expected 'value1', got %v", value)
	}
}

func TestStandardPlugin_HandleCounterMessages(t *testing.T) {
	channel := &mockChannel{name: "test"}
	liveObjects := NewLiveObjects(channel)
	plugin := liveObjects.GetPlugin()

	// Create a counter in LiveObjects
	liveCounter := liveObjects.Counter("counter1")
	liveCounter.created = true // Mark as created to simulate incoming messages

	// Create an INCREMENT operation message
	incMsg := &Message{
		Operation: &Operation{
			Action:   Operation_CounterInc,
			ObjectID: "counter1",
			CounterOp: &CounterOp{
				Amount: 5.0,
			},
		},
	}

	// Handle the message
	plugin.HandleObjectMessages([]*Message{incMsg})

	// Verify the value was incremented
	if liveCounter.Value() != 5.0 {
		t.Fatalf("Expected value 5.0, got %v", liveCounter.Value())
	}
}

func TestConcurrentAccess(t *testing.T) {
	channel := &mockChannel{name: "test"}
	liveObjects := NewLiveObjects(channel)
	liveMap := liveObjects.Map("map1")

	ctx := context.Background()

	// Create initial map
	err := liveMap.Set(ctx, "key1", "initial")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				err := liveMap.Set(ctx, "key1", i*10+j)
				if err != nil {
					t.Errorf("Set failed: %v", err)
				}
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, exists := liveMap.Get("key1")
				if !exists {
					t.Errorf("Key should exist")
				}
			}
		}()
	}

	wg.Wait()

	// Verify map is still in a consistent state
	_, exists := liveMap.Get("key1")
	if !exists {
		t.Fatal("Key should exist after concurrent operations")
	}
}

// Helper function for test
func stringPtr(s string) *string {
	return &s
}
