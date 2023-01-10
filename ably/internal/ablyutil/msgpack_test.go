//go:build !integration
// +build !integration

package ablyutil

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUnmarshallByte(t *testing.T) {
	buf := []byte{
		0xc4,     // bin8
		2,        // len
		'a', 'a', // bytes
	}
	var target interface{}

	err := UnmarshalMsgpack(buf, &target)
	require.NoError(t, err)
	assert.IsType(t, []byte{}, target,
		"bin8 should be decoded as []byte, but instead we got %T", target)
}

func TestMsgPackDecodingOpts(t *testing.T) {
	for _, rts := range []bool{false, true} {
		handle.RawToString = rts
		for _, msg := range []interface{}{
			//"aa",
			[]byte("aa"),
			//strings.Repeat("a", 300),
			bytes.Repeat([]byte("a"), 300),
			bytes.Repeat([]byte("a"), 64000),
		} {
			buf, err := MarshalMsgpack(msg)
			require.NoError(t, err)
			var target interface{}
			err = UnmarshalMsgpack(buf, &target)
			require.NoError(t, err)
			if len(buf) > 6 {
				buf = buf[:5]
			}
			t.Logf("WriteExt=%v RawToString=%v Msg=%T len=%d, -> %T, %#v", handle.WriteExt, rts, msg, Len(msg), target, buf)

		}
	}

}

func Len(a interface{}) int {
	switch v := a.(type) {
	case string:
		return len(v)
	case []byte:
		return len(v)
	default:
		panic(v)
	}
}

func TestMsgpack(t *testing.T) {
	type Object1 struct {
		Key int64 `codec:"my_key"`
	}
	type Object2 struct {
		Key float64 `codec:"my_key"`
	}
	t.Run("must decode int64 into float64", func(ts *testing.T) {
		var buf bytes.Buffer
		err := encodeMsg(&buf, &Object1{Key: 12})
		if err != nil {
			ts.Fatal(err)
		}
		b := &Object2{}
		err = decodeMsg(&buf, &b)
		if err != nil {
			ts.Fatal(err)
		}
		if b.Key != 12 {
			ts.Errorf("expected 12 got %v", b.Key)
		}
	})
}
