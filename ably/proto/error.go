package proto

import "fmt"

// Error describes an error returned via ProtocolMessage.
type Error struct {
	StatusCode int    `json:"statusCode,omitempty" msgpack:"statusCode,omitempty"`
	Code       int    `json:"code,omitempty" msgpack:"code,omitempty"`
	Message    string `json:"message,omitempty" msgpack:"message,omitempty"`
	Server     string `json:"serverId,omitempty" msgpack:"serverId,omitempty"`
}

// Error implements the builtin error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s (status=%d, internal=%d)", e.Message, e.StatusCode, e.Code)
}
