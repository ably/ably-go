package ablyutil

import (
	"bytes"
	"testing"
)

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
