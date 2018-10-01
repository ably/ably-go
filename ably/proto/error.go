package proto

import "fmt"

// Error describes an error returned via ProtocolMessage.
type Error struct {
	StatusCode int     `json:"statusCode,omitempty" codec:"statusCode,omitempty"`
	Code       float64 `json:"code,omitempty" codec:"code,omitempty"`
	Message    string  `json:"message,omitempty" codec:"message,omitempty"`
	Server     string  `json:"serverId,omitempty" codec:"serverId,omitempty"`
}

// Error implements the builtin error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s (status=%d, internal=%d)", e.Message, e.StatusCode, int(e.Code))
}
