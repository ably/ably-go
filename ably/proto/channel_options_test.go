package proto_test

import (
	"reflect"
	"testing"

	"github.com/ably/ably-go/ably/proto"
)

func TestRealtimeChannelModes_ToFlag(t *testing.T) {
	mode := proto.ChannelModePresence
	flag := mode.ToFlag()
	if flag != proto.FlagPresence {
		t.Fatalf("Expected %v, received %v", proto.FlagPresence, flag)
	}

	mode = proto.ChannelModeSubscribe
	flag = mode.ToFlag()
	if flag != proto.FlagSubscribe {
		t.Fatalf("Expected %v, received %v", proto.FlagSubscribe, flag)
	}

	mode = proto.ChannelModePublish
	flag = mode.ToFlag()
	if flag != proto.FlagPublish {
		t.Fatalf("Expected %v, received %v", proto.FlagPublish, flag)
	}

	mode = proto.ChannelModePresenceSubscribe
	flag = mode.ToFlag()
	if flag != proto.FlagPresenceSubscribe {
		t.Fatalf("Expected %v, received %v", proto.FlagPresenceSubscribe, flag)
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

	flags := proto.FlagPresence | proto.FlagPresenceSubscribe | proto.FlagSubscribe
	modes := proto.FromFlag(flags)

	if !inArray(proto.ChannelModePresence, modes) {
		t.Fatalf("Expected %v to be present in %v", proto.ChannelModePresence, modes)
	}

	if !inArray(proto.ChannelModePresenceSubscribe, modes) {
		t.Fatalf("Expected %v to be present in %v", proto.ChannelModePresenceSubscribe, modes)
	}

	if !inArray(proto.ChannelModeSubscribe, modes) {
		t.Fatalf("Expected %v to be present in %v", proto.ChannelModeSubscribe, modes)
	}

	if inArray(proto.ChannelModePublish, modes) {
		t.Fatalf("Expected %v not to be present in %v", proto.ChannelModePublish, modes)
	}
}
