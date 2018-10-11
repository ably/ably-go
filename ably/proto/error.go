package proto

import "fmt"

// Error describes an error returned via ProtocolMessage.
type Error struct {
	StatusCode int    `json:"statusCode,omitempty" codec:"statusCode,omitempty"`
	Code       int    `json:"code,omitempty" codec:"code,omitempty"`
	Message    string `json:"message,omitempty" codec:"message,omitempty"`
	Server     string `json:"serverId,omitempty" codec:"serverId,omitempty"`
}

func (e *Error) FromMap(ctx map[string]interface{}) {
	if v, ok := ctx["statusCode"]; ok {
		e.StatusCode = coerceInt(v)
	}
	if v, ok := ctx["code"]; ok {
		e.Code = coerceInt(v)
	}
	if v, ok := ctx["message"]; ok {
		e.Message = v.(string)
	}
	if v, ok := ctx["serverId"]; ok {
		e.Server = v.(string)
	}
}

// Error implements the builtin error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s (status=%d, internal=%d)", e.Message, e.StatusCode, e.Code)
}
