package ably

import (
	"context"
	"errors"
	"testing"

	"github.com/ably/ably-go/ably/objects"
	"github.com/stretchr/testify/assert"
)

func TestRealtimeExperimentalObjects_PublishObjects(t *testing.T) {
	channelName := "test-channel"

	tests := []struct {
		name                 string
		messages             []*objects.Message
		plugin               objects.Plugin
		sendError            error
		ackError             error
		expectedError        string
		expectedProtocolMsg  *protocolMessage
		expectedPrepareCount int
	}{
		{
			name: "successful publish with single message",
			messages: []*objects.Message{
				{
					ID:       "msg1",
					ClientID: "client1",
					Operation: &objects.Operation{
						Action:   objects.Operation_CounterCreate,
						ObjectID: "counter1",
					},
				},
			},
			plugin: &pluginMock{
				PrepareObjectFunc: func(msg *objects.Message) error {
					return nil
				},
			},
			sendError:            nil,
			ackError:             nil,
			expectedError:        "",
			expectedPrepareCount: 1,
		},
		{
			name: "successful publish with multiple messages",
			messages: []*objects.Message{
				{
					ID:       "msg1",
					ClientID: "client1",
					Operation: &objects.Operation{
						Action:   objects.Operation_CounterCreate,
						ObjectID: "counter1",
					},
				},
				{
					ID:       "msg2",
					ClientID: "client1",
					Operation: &objects.Operation{
						Action:   objects.Operation_MapCreate,
						ObjectID: "map1",
					},
				},
			},
			plugin: &pluginMock{
				PrepareObjectFunc: func(msg *objects.Message) error {
					return nil
				},
			},
			sendError:            nil,
			ackError:             nil,
			expectedError:        "",
			expectedPrepareCount: 2,
		},
		{
			name: "error when no plugin configured",
			messages: []*objects.Message{
				{
					ID:       "msg1",
					ClientID: "client1",
					Operation: &objects.Operation{
						Action:   objects.Operation_CounterCreate,
						ObjectID: "counter1",
					},
				},
			},
			plugin:               nil,
			sendError:            nil,
			ackError:             nil,
			expectedError:        "missing objects plugin",
			expectedPrepareCount: 0,
		},
		{
			name: "error during prepare object",
			messages: []*objects.Message{
				{
					ID:       "msg1",
					ClientID: "client1",
					Operation: &objects.Operation{
						Action:   objects.Operation_CounterCreate,
						ObjectID: "counter1",
					},
				},
			},
			plugin: &pluginMock{
				PrepareObjectFunc: func(msg *objects.Message) error {
					return errors.New("prepare failed")
				},
			},
			sendError:            nil,
			ackError:             nil,
			expectedError:        "prepare failed",
			expectedPrepareCount: 1,
		},
		{
			name: "error during send",
			messages: []*objects.Message{
				{
					ID:       "msg1",
					ClientID: "client1",
					Operation: &objects.Operation{
						Action:   objects.Operation_CounterCreate,
						ObjectID: "counter1",
					},
				},
			},
			plugin: &pluginMock{
				PrepareObjectFunc: func(msg *objects.Message) error {
					return nil
				},
			},
			sendError:            errors.New("send failed"),
			ackError:             nil,
			expectedError:        "send failed",
			expectedPrepareCount: 1,
		},
		{
			name: "error during ack",
			messages: []*objects.Message{
				{
					ID:       "msg1",
					ClientID: "client1",
					Operation: &objects.Operation{
						Action:   objects.Operation_CounterCreate,
						ObjectID: "counter1",
					},
				},
			},
			plugin: &pluginMock{
				PrepareObjectFunc: func(msg *objects.Message) error {
					return nil
				},
			},
			sendError:            nil,
			ackError:             errors.New("ack failed"),
			expectedError:        "ack failed",
			expectedPrepareCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prepareObjectFuncCalled int
			var capturedProtocolMsg *protocolMessage

			// Create channel mock
			channelMock := &channelMock{
				SendFunc: func(msg *protocolMessage, callback *ackCallback) error {
					if tt.sendError != nil {
						return tt.sendError
					}
					capturedProtocolMsg = msg
					// Simulate async ack
					go func() {
						callback.call(nil, tt.ackError)
					}()
					return nil
				},
				GetClientOptionsFunc: func() *clientOptions {
					return &clientOptions{
						ExperimentalObjectsPlugin: tt.plugin,
					}
				},
				GetNameFunc: func() string {
					return channelName
				},
			}

			// Wrap plugin to count prepare calls
			var wrappedPlugin objects.Plugin
			if tt.plugin != nil {
				wrappedPlugin = &pluginMock{
					PrepareObjectFunc: func(msg *objects.Message) error {
						prepareObjectFuncCalled++
						return tt.plugin.PrepareObject(msg)
					},
				}
			}

			// Update the channel's client options to use the wrapped plugin
			channelMock.GetClientOptionsFunc = func() *clientOptions {
				return &clientOptions{
					ExperimentalObjectsPlugin: wrappedPlugin,
				}
			}

			exp := newRealtimeExperimentalObjects(channelMock)

			ctx := context.Background()
			err := exp.PublishObjects(ctx, tt.messages...)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedPrepareCount, prepareObjectFuncCalled)

			// Verify protocol message if no error expected
			if tt.expectedError == "" && tt.sendError == nil {
				assert.NotNil(t, capturedProtocolMsg)
				assert.Equal(t, actionObject, capturedProtocolMsg.Action)
				assert.Equal(t, channelName, capturedProtocolMsg.Channel)
				assert.Equal(t, tt.messages, capturedProtocolMsg.State)
			}
		})
	}
}

func TestRealtimeExperimentalObjects_PublishObjectsContextCancellation(t *testing.T) {
	// Test context cancellation during publish
	channelMock := &channelMock{
		SendFunc: func(msg *protocolMessage, callback *ackCallback) error {
			// Don't call callback to simulate hanging
			return nil
		},
		GetClientOptionsFunc: func() *clientOptions {
			return &clientOptions{
				ExperimentalObjectsPlugin: &pluginMock{
					PrepareObjectFunc: func(msg *objects.Message) error {
						return nil
					},
				},
			}
		},
		GetNameFunc: func() string {
			return "test-channel"
		},
	}

	exp := newRealtimeExperimentalObjects(channelMock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	messages := []*objects.Message{
		{
			ID:       "msg1",
			ClientID: "client1",
			Operation: &objects.Operation{
				Action:   objects.Operation_CounterCreate,
				ObjectID: "counter1",
			},
		},
	}

	err := exp.PublishObjects(ctx, messages...)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

// channelMock implements the channel interface
type channelMock struct {
	SendFunc             func(msg *protocolMessage, callback *ackCallback) error
	GetClientOptionsFunc func() *clientOptions
	GetNameFunc          func() string
}

func (c channelMock) send(msg *protocolMessage, callback *ackCallback) error {
	return c.SendFunc(msg, callback)
}

func (c channelMock) getClientOptions() *clientOptions {
	return c.GetClientOptionsFunc()
}

func (c channelMock) getName() string {
	return c.GetNameFunc()
}

// pluginMock implements the objects.Plugin interface
type pluginMock struct {
	PrepareObjectFunc func(msg *objects.Message) error
}

func (p pluginMock) PrepareObject(msg *objects.Message) error {
	return p.PrepareObjectFunc(msg)
}

func (p pluginMock) HandleObjectMessages(msgs []*objects.Message) {
	// implementation not required for test
}

func (p pluginMock) HandleObjectSyncMessages(msgs []*objects.Message, channelSerial string) {
	// implementation not required for test
}
