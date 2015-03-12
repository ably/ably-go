package protocol

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
	Type       Type    `json:"type"`
	I32Data    int32   `json:"i32Data"`
	I64Data    int64   `json:"i64Data"`
	DoubleData float64 `json:"doubleData"`
	StringData string  `json:"stringData"`
	BinaryData []byte  `json:"binaryData"`
	CipherData []byte  `json:"cipherData"`
}
