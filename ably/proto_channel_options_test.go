//go:build !integration
// +build !integration

package ably_test

import (
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably"
)

func TestRealtimeChannelModes_ToFlag(t *testing.T) {
	mode := ably.ChannelModePresence
	flag := ably.ChannelModeToFlag(mode)
	if flag != ably.FlagPresence {
		t.Fatalf("Expected %v, received %v", ably.FlagPresence, flag)
	}

	mode = ably.ChannelModeSubscribe
	flag = ably.ChannelModeToFlag(mode)
	if flag != ably.FlagSubscribe {
		t.Fatalf("Expected %v, received %v", ably.FlagSubscribe, flag)
	}

	mode = ably.ChannelModePublish
	flag = ably.ChannelModeToFlag(mode)
	if flag != ably.FlagPublish {
		t.Fatalf("Expected %v, received %v", ably.FlagPublish, flag)
	}

	mode = ably.ChannelModePresenceSubscribe
	flag = ably.ChannelModeToFlag(mode)
	if flag != ably.FlagPresenceSubscribe {
		t.Fatalf("Expected %v, received %v", ably.FlagPresenceSubscribe, flag)
	}
}

func TestRealtimeChannelModes_FromFlag(t *testing.T) {

	// checks if element is present in the array
	inArray := func(v interface{}, in interface{}) (ok bool) {
		i := 0
		val := reflect.Indirect(reflect.ValueOf(in))
		switch val.Kind() {
		case reflect.Slice, reflect.Array:
			for ; i < val.Len(); i++ {
				if ok = v == val.Index(i).Interface(); ok {
					return
				}
			}
		}
		return
	}

	flags := ably.FlagPresence | ably.FlagPresenceSubscribe | ably.FlagSubscribe
	modes := ably.ChannelModeFromFlag(flags)

	if !inArray(ably.ChannelModePresence, modes) {
		t.Fatalf("Expected %v to be present in %v", ably.ChannelModePresence, modes)
	}

	if !inArray(ably.ChannelModePresenceSubscribe, modes) {
		t.Fatalf("Expected %v to be present in %v", ably.ChannelModePresenceSubscribe, modes)
	}

	if !inArray(ably.ChannelModeSubscribe, modes) {
		t.Fatalf("Expected %v to be present in %v", ably.ChannelModeSubscribe, modes)
	}

	if inArray(ably.ChannelModePublish, modes) {
		t.Fatalf("Expected %v not to be present in %v", ably.ChannelModePublish, modes)
	}
}
