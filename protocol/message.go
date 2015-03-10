package protocol

type Message struct {
	Name     string      `json:"name"`
	Data     interface{} `json:"data"`
	Encoding string      `json:"encoding,omitempty"`
}
