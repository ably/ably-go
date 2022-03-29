//go:build !integration
// +build !integration

package ably_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"

	"github.com/stretchr/testify/assert"
)

func TestDurationFromMsecsMarshal(t *testing.T) {

	for _, codec := range []struct {
		name      string
		marshal   func(interface{}) ([]byte, error)
		unmarshal func([]byte, interface{}) error
	}{
		{"JSON", json.Marshal, json.Unmarshal},
		{"Msgpack", ablyutil.MarshalMsgpack, ablyutil.UnmarshalMsgpack},
	} {
		t.Run(codec.name, func(t *testing.T) {

			js, err := codec.marshal(ably.DurationFromMsecs((123456 * time.Millisecond)))
			assert.NoError(t, err)

			var msecs int64
			err = codec.unmarshal(js, &msecs)
			assert.NoError(t, err)
			assert.Equal(t, int64(123456), msecs, "expected marshaling as JSON number of milliseconds; got %d (JSON: %q)", msecs, js)

			var decoded ably.DurationFromMsecs
			err = codec.unmarshal(js, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, (123456 * time.Millisecond), time.Duration(decoded),
				"expected json.Unmarshal after Marshal to produce the same duration; got %v", time.Duration(decoded))
		})
	}
}
