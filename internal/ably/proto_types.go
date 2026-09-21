package ably

import (
	"encoding/json"
	"time"

	"github.com/ugorji/go/codec"
)

// durationFromMsecs is a time.Duration that is marshaled as a JSON whole number of milliseconds.
type durationFromMsecs time.Duration

var _ interface {
	json.Marshaler
	json.Unmarshaler
	codec.Selfer
} = (*durationFromMsecs)(nil)

func (t durationFromMsecs) asMsecs() int64 {
	return time.Duration(t).Milliseconds()
}

func (t *durationFromMsecs) setFromMsecs(ms int64) {
	*t = durationFromMsecs(time.Duration(ms) * time.Millisecond)
}

func (t durationFromMsecs) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.asMsecs())
}

func (t *durationFromMsecs) UnmarshalJSON(js []byte) error {
	var ms int64
	err := json.Unmarshal(js, &ms)
	if err != nil {
		return err
	}
	t.setFromMsecs(ms)
	return nil
}

func (t durationFromMsecs) CodecEncodeSelf(encoder *codec.Encoder) {
	encoder.MustEncode(t.asMsecs())
}

func (t *durationFromMsecs) CodecDecodeSelf(decoder *codec.Decoder) {
	var ms int64
	decoder.MustDecode(&ms)
	t.setFromMsecs(ms)
}
