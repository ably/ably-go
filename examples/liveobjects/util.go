package liveobjects

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/objects"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Implements objects.Plugin
type loPlugin struct {
	mu                 sync.Mutex
	syncMessagesChan   chan *objects.Message
	objectMessagesChan chan *objects.Message
	done               chan struct{}

	syncMessages   []*objects.Message
	objectMessages []*objects.Message
}

func (c *loPlugin) PrepareObject(msg *objects.Message) error {
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

func (c *loPlugin) HandleObjectMessages(msgs []*objects.Message) {
	for _, msg := range msgs {
		c.objectMessagesChan <- msg
	}
}

func (c *loPlugin) HandleObjectSyncMessages(msgs []*objects.Message, channelSerial string) {
	for _, msg := range msgs {
		c.syncMessagesChan <- msg
	}
}

func (c *loPlugin) listen(ctx context.Context) {
	for {
		select {
		case msg := <-c.syncMessagesChan:
			c.mu.Lock()
			c.syncMessages = append(c.syncMessages, msg)
			c.mu.Unlock()
		case msg := <-c.objectMessagesChan:
			c.mu.Lock()
			c.objectMessages = append(c.objectMessages, msg)
			c.mu.Unlock()
		case <-c.done:
			log.Println("Plugin, done")
			return
		case <-ctx.Done():
			log.Println("Context done, exiting listen loop")
			return
		}
	}
}

func (c *loPlugin) EventuallyObjectMessages(condition func([]*objects.Message) bool, timeout, tick time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			messages := append([]*objects.Message{}, c.objectMessages...)
			c.mu.Unlock()
			if condition(messages) {
				return nil
			}
		case <-ctx.Done():
			return errors.New("timeout reached while waiting for condition")
		}
	}
}

func (c *loPlugin) close() {
	close(c.done)
}

func generateMessageID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateNonce() *string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	nonce := hex.EncodeToString(bytes)
	return &nonce
}

func getRealtimeClient(key string, plugin objects.Plugin) (*ably.Realtime, error) {
	return ably.NewRealtime(
		ably.WithKey(key),
		ably.WithPort(8081),
		ably.WithTLS(false),
		ably.WithInsecureAllowBasicAuthWithoutTLS(),
		ably.WithExperimentalObjectsPlugin(plugin),
		ably.WithEndpoint("localhost"),
	)
}

func createCounterOp(amount float64) (*objects.Message, error) {
	nonce := generateNonce()
	initialValue, err := json.Marshal(objects.Operation{Action: objects.Operation_CounterCreate, Nonce: nonce, Counter: &objects.Counter{Count: amount}})
	if err != nil {
		return nil, err
	}

	id, err := createObjectID("counter", initialValue, *nonce, time.Now())
	if err != nil {
		return nil, err
	}
	e := "json"
	return &objects.Message{
		ID: generateMessageID(),
		Operation: &objects.Operation{
			Nonce:                nonce,
			Action:               objects.Operation_CounterCreate,
			ObjectID:             id,
			Counter:              &objects.Counter{Count: amount},
			InitialValue:         initialValue,
			InitialValueEncoding: &e,
		},
	}, nil
}

func incCounterOp(id string, amount float64) *objects.Message {
	return &objects.Message{
		ID: generateMessageID(),
		Operation: &objects.Operation{
			Action:    objects.Operation_CounterInc,
			ObjectID:  id,
			CounterOp: &objects.CounterOp{Amount: amount},
		},
	}
}

func restCreateMap(key, channel string) (string, error) {
	url := fmt.Sprintf("http://localhost:8080/channels/%v/objects?key=%v", channel, key)
	payload := strings.NewReader("{\n\t\"operation\": \"MAP_CREATE\"\n}")

	req, _ := http.NewRequest("POST", url, payload)
	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create map, status code: %d", res.StatusCode)
	}

	bs, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	type pubRes struct {
		ObjectIDs []string `json:"objectIds"`
	}
	var body pubRes
	err = json.Unmarshal(bs, &body)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %v", err)
	}

	return body.ObjectIDs[0], nil
}

func setOnRootOp(key, objectID string) *objects.Message {
	return &objects.Message{
		ID: generateMessageID(),
		Operation: &objects.Operation{
			Action:   objects.Operation_MapSet,
			ObjectID: "root",
			MapOp: &objects.MapOp{
				Key: key,
				Data: &objects.Data{
					ObjectId: &objectID,
				},
			},
		},
	}
}

func createObjectID(objectType string, initialValue []byte, nonce string, now time.Time) (string, error) {
	hash := sha256.New()

	value := make([]byte, 0, len(initialValue)+len(nonce)+1)
	value = append(value, initialValue...)
	value = append(value, ':')
	value = append(value, []byte(nonce)...)

	_, err := hash.Write(value)
	if err != nil {
		return "", fmt.Errorf("failed to hash object ID: %v", err)
	}

	return fmt.Sprintf("%s:%s@%d", objectType, base64.RawURLEncoding.EncodeToString(hash.Sum(nil)), now.UnixMilli()), nil
}
