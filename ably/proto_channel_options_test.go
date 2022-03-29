//go:build !integration
// +build !integration

package ably_test

import (
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably"

	"github.com/stretchr/testify/assert"
)

func TestRealtimeChannelModes_ToFlag(t *testing.T) {
	mode := ably.ChannelModePresence
	flag := ably.ChannelModeToFlag(mode)
	assert.Equal(t, ably.FlagPresence, flag,
		"Expected %v, received %v", ably.FlagPresence, flag)
	mode = ably.ChannelModeSubscribe
	flag = ably.ChannelModeToFlag(mode)
	assert.Equal(t, ably.FlagSubscribe, flag,
		"Expected %v, received %v", ably.FlagSubscribe, flag)
	mode = ably.ChannelModePublish
	flag = ably.ChannelModeToFlag(mode)
	assert.Equal(t, ably.FlagPublish, flag,
		"Expected %v, received %v", ably.FlagPublish, flag)
	mode = ably.ChannelModePresenceSubscribe
	flag = ably.ChannelModeToFlag(mode)
	assert.Equal(t, ably.FlagPresenceSubscribe, flag,
		"Expected %v, received %v", ably.FlagPresenceSubscribe, flag)
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

	assert.True(t, inArray(ably.ChannelModePresence, modes),
		"Expected %v to be present in %v", ably.ChannelModePresence, modes)
	assert.True(t, inArray(ably.ChannelModePresenceSubscribe, modes),
		"Expected %v to be present in %v", ably.ChannelModePresenceSubscribe, modes)
	assert.True(t, inArray(ably.ChannelModeSubscribe, modes),
		"Expected %v to be present in %v", ably.ChannelModeSubscribe, modes)
	assert.False(t, inArray(ably.ChannelModePublish, modes),
		"Expected %v not to be present in %v", ably.ChannelModePublish, modes)
}
