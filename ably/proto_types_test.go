package ably_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
)

func TestDurationFromMsecsMarshal(t *testing.T) {

	d := 123456 * time.Millisecond

	for _, codec := range []struct {
		name      string
		marshal   func(interface{}) ([]byte, error)
		unmarshal func([]byte, interface{}) error
	}{
		{"JSON", json.Marshal, json.Unmarshal},
		{"Msgpack", ablyutil.MarshalMsgpack, ablyutil.UnmarshalMsgpack},
	} {
		t.Run(codec.name, func(t *testing.T) {
			t.Parallel()

			js, err := codec.marshal(ably.DurationFromMsecs(d))
			if err != nil {
				t.Fatal(err)
			}

			var msecs int64
			err = codec.unmarshal(js, &msecs)
			if err != nil {
				t.Fatal(err)
			}
			if expected, got := int64(123456), msecs; expected != got {
				t.Fatalf("expected marshaling as JSON number of milliseconds; got %d (JSON: %q)", got, js)
			}

			var decoded ably.DurationFromMsecs
			err = codec.unmarshal(js, &decoded)
			if err != nil {
				t.Fatal(err)
			}

			if expected, got := d, time.Duration(decoded); expected != got {
				t.Fatalf("expected json.Unmarshal after Marshal to produce the same duration; got %v", got)
			}
		})
	}
}
