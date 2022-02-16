//go:build !integration
// +build !integration

package ably_test

import (
	"context"

	"errors"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
)

func TestAuth_TimestampRSA10k(t *testing.T) {
	now, err := time.Parse(time.RFC822, time.RFC822)
	if err != nil {
		t.Fatal(err)
	}

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
		if err != nil {
			t.Fatal(err)
		}
		if !stamp.Equal(now) {
			t.Errorf("expected %s got %s", now, stamp)
		}
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
		if err != nil {
			t.Fatal(err)
		}
		serverTime := now.Add(time.Minute)
		if !stamp.Equal(serverTime) {
			t.Errorf("expected %s got %s", serverTime, stamp)
		}
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
		if err != nil {
			t.Fatal(err)
		}
		serverTime := now.Add(time.Minute)
		if !stamp.Equal(serverTime) {
			t.Errorf("expected %s got %s", serverTime, stamp)
		}

		now = now.Add(time.Minute)
		a.SetServerTimeFunc(func() (time.Time, error) {
			return time.Time{}, errors.New("must not be called")
		})
		stamp, err = rest.Timestamp(true)
		if err != nil {
			t.Fatal(err)
		}
		serverTime = now.Add(time.Minute)
		if !stamp.Equal(serverTime) {
			t.Errorf("expected %s got %s", serverTime, stamp)
		}
	})
}

func TestAuth_ClientID_Error(t *testing.T) {
	opts := []ably.ClientOption{
		ably.WithClientID("*"),
		ably.WithKey("abc:abc"),
		ably.WithUseTokenAuth(true),
	}
	_, err := ably.NewRealtime(opts...)
	if err := checkError(40102, err); err != nil {
		t.Fatal(err)
	}
}
