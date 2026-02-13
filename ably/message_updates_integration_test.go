//go:build !unit
// +build !unit

package ably_test

import (
	"context"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRESTChannel_MessageUpdates(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	require.NoError(t, err)
	defer app.Close()

	client, err := ably.NewREST(app.Options()...)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("PublishWithResult", func(t *testing.T) {
		// Use mutable: namespace to enable message operations feature
		channel := client.Channels.Get("mutable:test_publish_with_result")

		t.Run("returns serial for published message", func(t *testing.T) {
			result, err := channel.PublishWithResult(ctx, "event1", "test data")
			require.NoError(t, err)
			assert.NotEmpty(t, result.Serial, "Expected non-empty serial")
		})

		t.Run("PublishMultipleWithResult returns serials for all messages", func(t *testing.T) {
			messages := []*ably.Message{
				{Name: "event1", Data: "data1"},
				{Name: "event2", Data: "data2"},
				{Name: "event3", Data: "data3"},
			}

			results, err := channel.PublishMultipleWithResult(ctx, messages)
			require.NoError(t, err)
			assert.Len(t, results, 3)

			for i, result := range results {
				assert.NotEmpty(t, result.Serial, "Expected non-empty serial for message %d", i)
			}
		})
	})

	t.Run("UpdateMessage", func(t *testing.T) {
		channel := client.Channels.Get("mutable:test_update_message")

		t.Run("updates a message with new data", func(t *testing.T) {
			// Publish initial message
			publishResult, err := channel.PublishWithResult(ctx, "test-event", "initial data")
			require.NoError(t, err)

			// Update the message
			msg := &ably.Message{
				Serial: publishResult.Serial,
				Data:   "updated data",
			}
			updateResult, err := channel.UpdateMessage(ctx, msg,
				ably.UpdateWithDescription("Fixed typo"),
				ably.UpdateWithMetadata(map[string]string{"editor": "test"}),
			)
			require.NoError(t, err)
			assert.NotEmpty(t, updateResult.VersionSerial, "Expected version serial")
			assert.NotEqual(t, publishResult.Serial, updateResult.VersionSerial, "VersionSerial should differ from original Serial")

			// Verify the update by fetching the message (eventually consistent)
			require.Eventually(t, func() bool {
				retrieved, err := channel.GetMessage(ctx, publishResult.Serial)
				if err != nil {
					return false
				}
				return retrieved.Data == "updated data"
			}, 5*time.Second, 100*time.Millisecond, "Updated message should be retrievable")
		})

		t.Run("returns error when message has no serial", func(t *testing.T) {
			msg := &ably.Message{Data: "test"}
			_, err := channel.UpdateMessage(ctx, msg)
			require.Error(t, err)

			errorInfo, ok := err.(*ably.ErrorInfo)
			require.True(t, ok, "Expected ErrorInfo")
			assert.Equal(t, ably.ErrorCode(40003), errorInfo.Code)
			assert.Contains(t, errorInfo.Message(), "lacks a serial")
		})
	})

	t.Run("DeleteMessage", func(t *testing.T) {
		channel := client.Channels.Get("mutable:test_delete_message")

		t.Run("deletes a message", func(t *testing.T) {
			// Publish initial message
			publishResult, err := channel.PublishWithResult(ctx, "test-event", "data to delete")
			require.NoError(t, err)

			// Delete the message
			msg := &ably.Message{
				Serial: publishResult.Serial,
			}
			deleteResult, err := channel.DeleteMessage(ctx, msg,
				ably.UpdateWithDescription("Deleted by test"),
			)
			require.NoError(t, err)
			assert.NotEmpty(t, deleteResult.VersionSerial, "Expected version serial")
		})
	})

	t.Run("AppendMessage", func(t *testing.T) {
		channel := client.Channels.Get("mutable:test_append_message")

		t.Run("appends to a message", func(t *testing.T) {
			// Publish initial message
			publishResult, err := channel.PublishWithResult(ctx, "test-event", "Hello")
			require.NoError(t, err)

			// Append to the message
			msg := &ably.Message{
				Serial: publishResult.Serial,
				Data:   " World",
			}
			appendResult, err := channel.AppendMessage(ctx, msg)
			require.NoError(t, err)
			assert.NotEmpty(t, appendResult.VersionSerial, "Expected version serial")

			// Verify by fetching the message - data should be appended (eventually consistent)
			require.Eventually(t, func() bool {
				retrieved, err := channel.GetMessage(ctx, publishResult.Serial)
				if err != nil {
					return false
				}
				// Verify the data was appended: "Hello" + " World" = "Hello World"
				return retrieved.Data == "Hello World"
			}, 5*time.Second, 100*time.Millisecond, "Message data should be appended")
		})
	})

	t.Run("GetMessage", func(t *testing.T) {
		channel := client.Channels.Get("mutable:test_get_message")

		t.Run("retrieves a message by serial", func(t *testing.T) {
			// Publish a message
			publishResult, err := channel.PublishWithResult(ctx, "test-event", "test data")
			require.NoError(t, err)

			// GetMessage is eventually consistent - retry until message is available
			require.Eventually(t, func() bool {
				msg, err := channel.GetMessage(ctx, publishResult.Serial)
				if err != nil {
					return false
				}
				return msg.Data == "test data" && msg.Serial == publishResult.Serial
			}, 5*time.Second, 100*time.Millisecond, "Message should be retrievable")
		})
	})

	t.Run("GetMessageVersions", func(t *testing.T) {
		channel := client.Channels.Get("mutable:test_get_message_versions")

		t.Run("retrieves all versions after updates", func(t *testing.T) {
			// Publish initial message
			publishResult, err := channel.PublishWithResult(ctx, "test-event", "version 1")
			require.NoError(t, err)

			// Update the message twice
			msg := &ably.Message{
				Serial: publishResult.Serial,
				Data:   "version 2",
			}
			_, err = channel.UpdateMessage(ctx, msg, ably.UpdateWithDescription("First update"))
			require.NoError(t, err)

			msg.Data = "version 3"
			_, err = channel.UpdateMessage(ctx, msg, ably.UpdateWithDescription("Second update"))
			require.NoError(t, err)

			// GetMessageVersions is eventually consistent - retry until all versions are available
			var versions []*ably.Message
			require.Eventually(t, func() bool {
				page, err := channel.GetMessageVersions(publishResult.Serial, nil).Pages(ctx)
				if err != nil {
					return false
				}

				// Must call Next() to decode the response body into items
				if !page.Next(ctx) {
					return false
				}

				versions = page.Items()

				// Should have exactly 3 versions: original publish + 2 updates
				return len(versions) == 3
			}, 10*time.Second, 200*time.Millisecond, "All three message versions should be retrievable")

			// Verify we have exactly 3 versions in the correct order
			require.Equal(t, 3, len(versions))
			assert.Equal(t, ably.MessageActionCreate, versions[0].Action, "First version should be message.create")
			assert.Equal(t, ably.MessageActionUpdate, versions[1].Action, "Second version should be message.update")
			assert.Equal(t, ably.MessageActionUpdate, versions[2].Action, "Third version should be message.update")
		})
	})
}

func TestRealtimeChannel_MessageUpdates(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	require.NoError(t, err)
	defer app.Close()

	client, err := ably.NewRealtime(app.Options()...)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Wait for connection
	err = ablytest.Wait(ablytest.ConnWaiter(client, nil, ably.ConnectionEventConnected), nil)
	require.NoError(t, err)

	t.Run("PublishWithResult", func(t *testing.T) {
		channel := client.Channels.Get("mutable:test_realtime_publish_with_result")

		// Attach channel
		err := channel.Attach(ctx)
		require.NoError(t, err)

		t.Run("returns serial for published message", func(t *testing.T) {
			result, err := channel.PublishWithResult(ctx, "event1", "realtime data")
			require.NoError(t, err)
			assert.NotEmpty(t, result.Serial, "Expected non-empty serial")
		})

		t.Run("PublishMultipleWithResult returns serials", func(t *testing.T) {
			messages := []*ably.Message{
				{Name: "evt1", Data: "data1"},
				{Name: "evt2", Data: "data2"},
			}

			results, err := channel.PublishMultipleWithResult(ctx, messages)
			require.NoError(t, err)
			assert.Len(t, results, 2)

			for i, result := range results {
				assert.NotEmpty(t, result.Serial, "Expected serial for message %d", i)
			}
		})
	})

	t.Run("UpdateMessageAsync", func(t *testing.T) {
		channel := client.Channels.Get("mutable:test_realtime_update_async")

		// Attach channel
		err := channel.Attach(ctx)
		require.NoError(t, err)

		t.Run("updates message asynchronously", func(t *testing.T) {
			// Publish initial message
			publishResult, err := channel.PublishWithResult(ctx, "event", "initial")
			require.NoError(t, err)

			// Update asynchronously
			done := make(chan *ably.UpdateResult, 1)
			errChan := make(chan error, 1)

			msg := &ably.Message{
				Serial: publishResult.Serial,
				Data:   "updated async",
			}
			err = channel.UpdateMessageAsync(msg, func(result *ably.UpdateResult, err error) {
				if err != nil {
					errChan <- err
				} else {
					done <- result
				}
			})
			require.NoError(t, err)

			// Wait for callback
			select {
			case result := <-done:
				assert.NotEmpty(t, result.VersionSerial)
			case err := <-errChan:
				t.Fatalf("Update failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for update callback")
			}
		})
	})

	t.Run("AppendMessageAsync", func(t *testing.T) {
		channel := client.Channels.Get("mutable:test_ai_streaming")

		// Attach channel
		err := channel.Attach(ctx)
		require.NoError(t, err)

		t.Run("rapid async appends for AI token streaming", func(t *testing.T) {
			// Publish initial message
			publishResult, err := channel.PublishWithResult(ctx, "ai-response", "The answer is")
			require.NoError(t, err)

			// Simulate rapid token streaming
			tokens := []string{" 42", ".", " This", " is", " correct", "."}
			completed := make(chan int, len(tokens))

			for i, token := range tokens {
				msg := &ably.Message{
					Serial: publishResult.Serial,
					Data:   token,
				}

				idx := i // Capture for closure
				err := channel.AppendMessageAsync(msg, func(result *ably.UpdateResult, err error) {
					if err != nil {
						t.Logf("Append %d failed: %v", idx, err)
					} else {
						completed <- idx
					}
				})
				require.NoError(t, err, "Failed to queue append %d", i)
			}

			// Wait for all appends to complete (with timeout)
			timeout := time.After(10 * time.Second)
			completedCount := 0
			for completedCount < len(tokens) {
				select {
				case <-completed:
					completedCount++
				case <-timeout:
					t.Fatalf("Timeout: Only %d/%d appends completed before timeout", completedCount, len(tokens))
				}
			}

			assert.Equal(t, len(tokens), completedCount, "All appends should complete")
		})
	})
}
