//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ablytest"

	"github.com/stretchr/testify/assert"
)

func TestAuth_TimestampRSA10k(t *testing.T) {
	now, err := time.Parse(time.RFC822, time.RFC822)
	assert.NoError(t, err)

	t.Run("must use local time when UseQueryTime is false", func(t *testing.T) {

		rest, _ := ably.NewREST(
			ably.WithKey("fake:key"),
			ably.WithNow(func() time.Time {
				return now
			}))
		a := rest.Auth
		a.SetServerTimeFunc(func() (time.Time, error) {
			return now.Add(time.Minute), nil
		})
		stamp, err := a.Timestamp(context.Background(), false)
		assert.NoError(t, err)
		assert.True(t, stamp.Equal(now),
			"expected %s got %s", now, stamp)
	})

	t.Run("RSA10j, RSA10g: stores given arguments as defaults for subsequent authorizations with exception of tokenParams timestamp and queryTime", func(t *testing.T) {
		t.Parallel()
		rec, extraOpt := recorder()
		defer rec.Stop()

		opts := []ably.ClientOption{
			ably.WithTLS(true),
			ably.WithUseTokenAuth(true),
			ably.WithQueryTime(false),
		}

		app, client := ablytest.NewREST(append(opts, extraOpt...)...)
		defer safeclose(t, app)

		prevTokenParams := client.Auth.Params()
		prevAuthOptions := client.Auth.AuthOptions()
		prevUseQueryTime := prevAuthOptions.UseQueryTime

		tokenParams := &ably.TokenParams{
			TTL:        123890,
			Capability: `{"foo":["publish"]}`,
			ClientID:   "abcd1234",
			Timestamp:  time.Now().UnixNano() / int64(time.Millisecond),
		}

		newAuthOptions := []ably.AuthOption{
			ably.AuthWithMethod("POST"),
			ably.AuthWithQueryTime(true),
			ably.AuthWithKey(app.Key()),
		}

		_, err := client.Auth.Authorize(context.Background(), tokenParams, newAuthOptions...) // Call to Authorize should always refresh the token.
		assert.NoError(t, err)
		updatedParams := client.Auth.Params()
		updatedAuthOptions := client.Auth.AuthOptions()

		assert.Nil(t, prevTokenParams)
		assert.Equal(t, tokenParams, updatedParams) // RSA10J
		assert.Zero(t, updatedParams.Timestamp)     // RSA10g

		assert.Equal(t, prevAuthOptions, updatedAuthOptions)                  // RSA10J
		assert.NotEqual(t, prevUseQueryTime, updatedAuthOptions.UseQueryTime) // RSA10g
	})

	t.Run("must use server time when UseQueryTime is true", func(t *testing.T) {

		rest, _ := ably.NewREST(
			ably.WithKey("fake:key"),
			ably.WithNow(func() time.Time {
				return now
			}))
		a := rest.Auth
		a.SetServerTimeFunc(func() (time.Time, error) {
			return now.Add(time.Minute), nil
		})
		stamp, err := rest.Timestamp(true)
		assert.NoError(t, err)
		serverTime := now.Add(time.Minute)
		assert.True(t, stamp.Equal(serverTime),
			"expected %s got %s", serverTime, stamp)
	})
	t.Run("must use server time offset ", func(t *testing.T) {

		now := now
		rest, _ := ably.NewREST(
			ably.WithKey("fake:key"),
			ably.WithNow(func() time.Time {
				return now
			}))
		a := rest.Auth
		a.SetServerTimeFunc(func() (time.Time, error) {
			return now.Add(time.Minute), nil
		})
		stamp, err := rest.Timestamp(true)
		assert.NoError(t, err)
		serverTime := now.Add(time.Minute)
		assert.True(t, stamp.Equal(serverTime),
			"expected %s got %s", serverTime, stamp)

		now = now.Add(time.Minute)
		a.SetServerTimeFunc(func() (time.Time, error) {
			return time.Time{}, errors.New("must not be called")
		})
		stamp, err = rest.Timestamp(true)
		assert.NoError(t, err)
		serverTime = now.Add(time.Minute)
		assert.True(t, stamp.Equal(serverTime),
			"expected %s got %s", serverTime, stamp)
	})
}

func TestAuth_ClientID_Error(t *testing.T) {
	opts := []ably.ClientOption{
		ably.WithClientID("*"),
		ably.WithKey("abc:abc"),
		ably.WithUseTokenAuth(true),
	}
	_, err := ably.NewRealtime(opts...)
	err = checkError(40102, err)
	assert.NoError(t, err)
}
