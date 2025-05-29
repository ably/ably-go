package objects

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// LiveObjects provides access to LiveMap and LiveCounter objects for a channel.
type LiveObjects struct {
	channel  Channel
	plugin   *StandardPlugin
	mu       sync.RWMutex
	maps     map[string]*LiveMap
	counters map[string]*LiveCounter
}

// Channel interface allows LiveObjects to work with realtime channels
type Channel interface {
	PublishObjects(ctx context.Context, msgs ...*Message) error
}

// NewLiveObjects creates a new LiveObjects instance for a channel.
func NewLiveObjects(channel Channel) *LiveObjects {
	lo := &LiveObjects{
		channel:  channel,
		maps:     make(map[string]*LiveMap),
		counters: make(map[string]*LiveCounter),
	}
	lo.plugin = &StandardPlugin{liveObjects: lo}
	return lo
}

// GetPlugin returns the plugin implementation for use with WithObjectsPlugin.
func (lo *LiveObjects) GetPlugin() Plugin {
	return lo.plugin
}

// Map returns a LiveMap with the given objectID, creating it if it doesn't exist.
func (lo *LiveObjects) Map(objectID string) *LiveMap {
	lo.mu.Lock()
	defer lo.mu.Unlock()

	if lm, exists := lo.maps[objectID]; exists {
		return lm
	}

	lm := &LiveMap{
		objectID:    objectID,
		liveObjects: lo,
		data:        make(map[string]interface{}),
		listeners:   make(map[string][]MapEventListener),
	}
	lo.maps[objectID] = lm
	return lm
}

// Counter returns a LiveCounter with the given objectID, creating it if it doesn't exist.
func (lo *LiveObjects) Counter(objectID string) *LiveCounter {
	lo.mu.Lock()
	defer lo.mu.Unlock()

	if lc, exists := lo.counters[objectID]; exists {
		return lc
	}

	lc := &LiveCounter{
		objectID:    objectID,
		liveObjects: lo,
		listeners:   make([]CounterEventListener, 0),
	}
	lo.counters[objectID] = lc
	return lc
}

// LiveMap represents a conflict-free map object that can be synchronized across clients.
type LiveMap struct {
	objectID    string
	liveObjects *LiveObjects
	mu          sync.RWMutex
	data        map[string]interface{}
	listeners   map[string][]MapEventListener
	created     bool
}

// MapEventListener is called when map operations occur.
type MapEventListener func(event MapEvent)

// MapEvent represents a change to a LiveMap.
type MapEvent struct {
	Type     MapEventType
	Key      string
	Value    interface{}
	Previous interface{}
}

// MapEventType represents the type of map event.
type MapEventType string

const (
	MapEventSet    MapEventType = "set"
	MapEventRemove MapEventType = "remove"
)

// Set sets a key-value pair in the map.
func (lm *LiveMap) Set(ctx context.Context, key string, value interface{}) error {
	if !lm.created {
		if err := lm.create(ctx, value); err != nil {
			return err
		}
	}

	return lm.set(ctx, key, value)
}

// Get retrieves a value from the map.
func (lm *LiveMap) Get(key string) (interface{}, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	value, exists := lm.data[key]
	return value, exists
}

// Remove removes a key from the map.
func (lm *LiveMap) Remove(ctx context.Context, key string) error {
	lm.mu.Lock()
	previous, exists := lm.data[key]
	if !exists {
		lm.mu.Unlock()
		return nil
	}
	delete(lm.data, key)
	lm.mu.Unlock()

	data := &Data{String: &key}
	mapOp := &MapOp{Key: key, Data: data}

	msg := &Message{
		Operation: &Operation{
			Action:   Operation_MapRemove,
			ObjectID: lm.objectID,
			MapOp:    mapOp,
		},
	}

	err := lm.liveObjects.channel.PublishObjects(ctx, msg)
	if err == nil {
		lm.notifyListeners(MapEvent{
			Type:     MapEventRemove,
			Key:      key,
			Previous: previous,
		})
	}
	return err
}

// Keys returns all keys in the map.
func (lm *LiveMap) Keys() []string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	keys := make([]string, 0, len(lm.data))
	for k := range lm.data {
		keys = append(keys, k)
	}
	return keys
}

// Size returns the number of key-value pairs in the map.
func (lm *LiveMap) Size() int {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return len(lm.data)
}

// Subscribe adds a listener for changes to a specific key.
func (lm *LiveMap) Subscribe(key string, listener MapEventListener) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.listeners[key] = append(lm.listeners[key], listener)
}

func (lm *LiveMap) create(ctx context.Context, initialValue interface{}) error {
	initialValueBytes, err := json.Marshal(initialValue)
	if err != nil {
		return fmt.Errorf("failed to marshal initial value: %w", err)
	}

	encoding := "json"
	msg := &Message{
		Operation: &Operation{
			Action:               Operation_MapCreate,
			ObjectID:             lm.objectID,
			InitialValue:         initialValueBytes,
			InitialValueEncoding: &encoding,
			Nonce:                generateNonce(),
		},
	}

	err = lm.liveObjects.channel.PublishObjects(ctx, msg)
	if err == nil {
		lm.created = true
	}
	return err
}

func (lm *LiveMap) set(ctx context.Context, key string, value interface{}) error {
	lm.mu.Lock()
	previous, _ := lm.data[key]
	lm.data[key] = value
	lm.mu.Unlock()

	data := &Data{}
	switch v := value.(type) {
	case string:
		data.String = &v
	case bool:
		data.Boolean = &v
	case float64:
		data.Number = &v
	case int:
		f := float64(v)
		data.Number = &f
	case []byte:
		data.Bytes = v
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("unsupported value type: %T", v)
		}
		data.Bytes = jsonBytes
		encoding := "json"
		data.Encoding = &encoding
	}

	mapOp := &MapOp{Key: key, Data: data}

	msg := &Message{
		Operation: &Operation{
			Action:   Operation_MapSet,
			ObjectID: lm.objectID,
			MapOp:    mapOp,
		},
	}

	err := lm.liveObjects.channel.PublishObjects(ctx, msg)
	if err == nil {
		eventType := MapEventSet
		lm.notifyListeners(MapEvent{
			Type:     eventType,
			Key:      key,
			Value:    value,
			Previous: previous,
		})
	}
	return err
}

func (lm *LiveMap) notifyListeners(event MapEvent) {
	lm.mu.RLock()
	listeners := lm.listeners[event.Key]
	lm.mu.RUnlock()

	for _, listener := range listeners {
		go listener(event)
	}
}

// LiveCounter represents a conflict-free counter that can be synchronized across clients.
type LiveCounter struct {
	objectID    string
	liveObjects *LiveObjects
	mu          sync.RWMutex
	value       float64
	listeners   []CounterEventListener
	created     bool
}

// CounterEventListener is called when counter operations occur.
type CounterEventListener func(event CounterEvent)

// CounterEvent represents a change to a LiveCounter.
type CounterEvent struct {
	Value    float64
	Previous float64
	Delta    float64
}

// Increment increments the counter by the given amount.
func (lc *LiveCounter) Increment(ctx context.Context, amount float64) error {
	if !lc.created {
		if err := lc.create(ctx, amount); err != nil {
			return err
		}
		return nil
	}

	return lc.increment(ctx, amount)
}

// Decrement decrements the counter by the given amount.
func (lc *LiveCounter) Decrement(ctx context.Context, amount float64) error {
	return lc.Increment(ctx, -amount)
}

// Value returns the current value of the counter.
func (lc *LiveCounter) Value() float64 {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.value
}

// Subscribe adds a listener for counter changes.
func (lc *LiveCounter) Subscribe(listener CounterEventListener) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.listeners = append(lc.listeners, listener)
}

func (lc *LiveCounter) create(ctx context.Context, initialValue float64) error {
	initialValueBytes, err := json.Marshal(initialValue)
	if err != nil {
		return fmt.Errorf("failed to marshal initial value: %w", err)
	}

	encoding := "json"
	msg := &Message{
		Operation: &Operation{
			Action:               Operation_CounterCreate,
			ObjectID:             lc.objectID,
			InitialValue:         initialValueBytes,
			InitialValueEncoding: &encoding,
			Nonce:                generateNonce(),
		},
	}

	err = lc.liveObjects.channel.PublishObjects(ctx, msg)
	if err == nil {
		lc.created = true
		lc.mu.Lock()
		lc.value = initialValue
		lc.mu.Unlock()
	}
	return err
}

func (lc *LiveCounter) increment(ctx context.Context, amount float64) error {
	lc.mu.Lock()
	previous := lc.value
	lc.value += amount
	newValue := lc.value
	lc.mu.Unlock()

	counterOp := &CounterOp{Amount: amount}

	msg := &Message{
		Operation: &Operation{
			Action:    Operation_CounterInc,
			ObjectID:  lc.objectID,
			CounterOp: counterOp,
		},
	}

	err := lc.liveObjects.channel.PublishObjects(ctx, msg)
	if err == nil {
		lc.notifyListeners(CounterEvent{
			Value:    newValue,
			Previous: previous,
			Delta:    amount,
		})
	}
	return err
}

func (lc *LiveCounter) notifyListeners(event CounterEvent) {
	lc.mu.RLock()
	listeners := make([]CounterEventListener, len(lc.listeners))
	copy(listeners, lc.listeners)
	lc.mu.RUnlock()

	for _, listener := range listeners {
		go listener(event)
	}
}

// StandardPlugin implements the Plugin interface for standard LiveObjects functionality.
type StandardPlugin struct {
	liveObjects *LiveObjects
}

// PrepareObject prepares an object message for publishing.
func (sp *StandardPlugin) PrepareObject(msg *Message) error {
	if msg.Operation == nil {
		return fmt.Errorf("operation is required")
	}

	if msg.Operation.ObjectID == "" {
		return fmt.Errorf("objectId is required")
	}

	// Generate message ID if not present
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}

	// Set timestamp if not present
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().UnixMilli()
	}

	return nil
}

// HandleObjectMessages handles incoming object messages.
func (sp *StandardPlugin) HandleObjectMessages(msgs []*Message) {
	for _, msg := range msgs {
		sp.handleObjectMessage(msg)
	}
}

// HandleObjectSyncMessages handles incoming object sync messages.
func (sp *StandardPlugin) HandleObjectSyncMessages(msgs []*Message, channelSerial string) {
	for _, msg := range msgs {
		sp.handleObjectMessage(msg)
	}
}

func (sp *StandardPlugin) handleObjectMessage(msg *Message) {
	if msg.Operation == nil {
		return
	}

	objectID := msg.Operation.ObjectID

	switch msg.Operation.Action {
	case Operation_MapCreate, Operation_MapSet, Operation_MapRemove:
		sp.handleMapOperation(objectID, msg.Operation)
	case Operation_CounterCreate, Operation_CounterInc:
		sp.handleCounterOperation(objectID, msg.Operation)
	}
}

func (sp *StandardPlugin) handleMapOperation(objectID string, op *Operation) {
	sp.liveObjects.mu.RLock()
	lm, exists := sp.liveObjects.maps[objectID]
	sp.liveObjects.mu.RUnlock()

	if !exists {
		return
	}

	switch op.Action {
	case Operation_MapCreate:
		lm.created = true
	case Operation_MapSet:
		if op.MapOp != nil {
			key := op.MapOp.Key
			value := extractDataValue(op.MapOp.Data)

			lm.mu.Lock()
			previous, _ := lm.data[key]
			lm.data[key] = value
			lm.mu.Unlock()

			lm.notifyListeners(MapEvent{
				Type:     MapEventSet,
				Key:      key,
				Value:    value,
				Previous: previous,
			})
		}
	case Operation_MapRemove:
		if op.MapOp != nil {
			key := op.MapOp.Key

			lm.mu.Lock()
			previous, existed := lm.data[key]
			if existed {
				delete(lm.data, key)
			}
			lm.mu.Unlock()

			if existed {
				lm.notifyListeners(MapEvent{
					Type:     MapEventRemove,
					Key:      key,
					Previous: previous,
				})
			}
		}
	}
}

func (sp *StandardPlugin) handleCounterOperation(objectID string, op *Operation) {
	sp.liveObjects.mu.RLock()
	lc, exists := sp.liveObjects.counters[objectID]
	sp.liveObjects.mu.RUnlock()

	if !exists {
		return
	}

	switch op.Action {
	case Operation_CounterCreate:
		lc.created = true
		if len(op.InitialValue) > 0 {
			var initialValue float64
			if err := json.Unmarshal(op.InitialValue, &initialValue); err == nil {
				lc.mu.Lock()
				lc.value = initialValue
				lc.mu.Unlock()
			}
		}
	case Operation_CounterInc:
		if op.CounterOp != nil {
			amount := op.CounterOp.Amount

			lc.mu.Lock()
			previous := lc.value
			lc.value += amount
			newValue := lc.value
			lc.mu.Unlock()

			lc.notifyListeners(CounterEvent{
				Value:    newValue,
				Previous: previous,
				Delta:    amount,
			})
		}
	}
}

func extractDataValue(data *Data) interface{} {
	if data == nil {
		return nil
	}

	if data.String != nil {
		return *data.String
	}
	if data.Boolean != nil {
		return *data.Boolean
	}
	if data.Number != nil {
		return *data.Number
	}
	if data.Bytes != nil {
		if data.Encoding != nil && *data.Encoding == "json" {
			var value interface{}
			if err := json.Unmarshal(data.Bytes, &value); err == nil {
				return value
			}
		}
		return data.Bytes
	}

	return nil
}

func generateNonce() *string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	nonce := hex.EncodeToString(bytes)
	return &nonce
}

func generateMessageID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
