package ablyutil

import (
	"bytes"
	"io"

	"github.com/ugorji/go/codec"
)

var handle codec.MsgpackHandle

// Unmarshal decodes the MessagePack-encoded data and stores the result in the
// value pointed to by v.
func Unmarshal(data []byte, v interface{}) error {
	return decodeMsg(bytes.NewReader(data), v)
}

// decodeMsg decodes msgpack message read from r into v.
func decodeMsg(r io.Reader, v interface{}) error {
	dec := codec.NewDecoder(r, &handle)
	return dec.Decode(v)
}

// Marshal retruns msgpack encoding of v
func Marshal(v interface{}) ([]byte, error) {
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
