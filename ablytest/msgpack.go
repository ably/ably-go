package ablytest

import (
	"bytes"
	"io"

	"github.com/ugorji/go/codec"
)

var handle codec.MsgpackHandle

func init() {
	handle.Raw = true
	handle.WriteExt = true
	handle.RawToString = true
}

// marshalMsgpack returns msgpack encoding of v
func marshalMsgpack(v interface{}) ([]byte, error) {
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
