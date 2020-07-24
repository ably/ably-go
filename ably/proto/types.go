package proto

import (
	"encoding/json"
	"time"

	"github.com/ugorji/go/codec"
)

type Type int64

const (
	TypeNONE Type = iota
	TypeTRUE
	TypeFALSE
	TypeINT32
	TypeINT64
	TypeDOUBLE
	TypeSTRING
	TypeBUFFER
	TypeJSONARRAY
	TypeJSONOBJECT
)

type Data struct {
	Type       Type    `json:"type" codec:"type"`
	I32Data    int32   `json:"i32Data" codec:"i32Data"`
	I64Data    int64   `json:"i64Data" codec:"i64Data"`
	DoubleData float64 `json:"doubleData" codec:"doubleData"`
	StringData string  `json:"stringData" codec:"stringData"`
	BinaryData []byte  `json:"binaryData" codec:"binaryData"`
	CipherData []byte  `json:"cipherData" codec:"cipherData"`
}

// DurationFromMsecs is a time.Duration that is marshaled as a JSON whole number
// of milliseconds.
type DurationFromMsecs time.Duration

var _ interface {
	json.Marshaler
	json.Unmarshaler
	codec.Selfer
} = (*DurationFromMsecs)(nil)

func (t DurationFromMsecs) asMsecs() int64 {
	return int64(time.Duration(t) / time.Millisecond)
}

func (t *DurationFromMsecs) setFromMsecs(ms int64) {
	*t = DurationFromMsecs(time.Duration(ms) * time.Millisecond)
}

func (t DurationFromMsecs) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.asMsecs())
}

func (t *DurationFromMsecs) UnmarshalJSON(js []byte) error {
	var ms int64
	err := json.Unmarshal(js, &ms)
	if err != nil {
		return err
	}
	t.setFromMsecs(ms)
	return nil
}

func (t DurationFromMsecs) CodecEncodeSelf(encoder *codec.Encoder) {
	encoder.MustEncode(t.asMsecs())
}

func (t *DurationFromMsecs) CodecDecodeSelf(decoder *codec.Decoder) {
	var ms int64
	decoder.MustDecode(&ms)
	t.setFromMsecs(ms)
}
