package ablyutil

import (
	"bytes"
	"io"
	"reflect"

	"github.com/ugorji/go/codec"
)

var handle codec.MsgpackHandle

func init() {
	handle.Raw = true
	handle.WriteExt = true
	handle.RawToString = true
	handle.MapType = reflect.TypeOf(map[string]interface{}(nil))
}

// UnmarshalMsgpack decodes the MessagePack-encoded data and stores the result in the
// value pointed to by v.
func UnmarshalMsgpack(data []byte, v interface{}) error {
	return decodeMsg(bytes.NewReader(data), v)
}

// decodeMsg decodes msgpack message read from r into v.
func decodeMsg(r io.Reader, v interface{}) error {
	dec := codec.NewDecoder(r, &handle)
	return dec.Decode(v)
}

// MarshalMsgpack returns msgpack encoding of v
func MarshalMsgpack(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := encodeMsg(&buf, v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeMsg encodes v into msgpack format and writes the output to w.
func encodeMsg(w io.Writer, v interface{}) error {
	enc := codec.NewEncoder(w, &handle)
	return enc.Encode(v)
}
