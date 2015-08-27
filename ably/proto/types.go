package proto

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
	Type       Type    `json:"type" msgpack:"type"`
	I32Data    int32   `json:"i32Data" msgpack:"i32Data"`
	I64Data    int64   `json:"i64Data" msgpack:"i64Data"`
	DoubleData float64 `json:"doubleData" msgpack:"doubleData"`
	StringData string  `json:"stringData" msgpack:"stringData"`
	BinaryData []byte  `json:"binaryData" msgpack:"binaryData"`
	CipherData []byte  `json:"cipherData" msgpack:"cipherData"`
}
