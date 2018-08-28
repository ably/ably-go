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
	Type       Type    `json:"type" codec:"type"`
	I32Data    int32   `json:"i32Data" codec:"i32Data"`
	I64Data    int64   `json:"i64Data" codec:"i64Data"`
	DoubleData float64 `json:"doubleData" codec:"doubleData"`
	StringData string  `json:"stringData" codec:"stringData"`
	BinaryData []byte  `json:"binaryData" codec:"binaryData"`
	CipherData []byte  `json:"cipherData" codec:"cipherData"`
}
