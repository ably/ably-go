//go:build !integration
// +build !integration

package ablyutil

import (
	"bytes"
	"encoding/json"
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

func TestMsgpackJson(t *testing.T) {
	extras := map[string]interface{}{
		"headers": map[string]interface{}{
			"version": 1,
		},
	}

	b, err := MarshalMsgpack(extras)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	err = UnmarshalMsgpack(b, &got)
	if err != nil {
		t.Fatal(err)
	}

	buff := bytes.Buffer{}
	err = json.NewEncoder(&buff).Encode(got)
	if err != nil {
		t.Fatal(err)
	}
}
