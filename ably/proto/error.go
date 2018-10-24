package proto

import (
	"fmt"
	"strings"
)

// ErrorInfo describes an error returned via ProtocolMessage.
type ErrorInfo struct {
	StatusCode int    `json:"statusCode,omitempty" codec:"statusCode,omitempty"`
	Code       int    `json:"code,omitempty" codec:"code,omitempty"`
	HRef       string `json:"href,omitempty" codec:"href,omitempty"` //spec TI4
	Message    string `json:"message,omitempty" codec:"message,omitempty"`
	Server     string `json:"serverId,omitempty" codec:"serverId,omitempty"`
}

func (e *ErrorInfo) FromMap(ctx map[string]interface{}) {
	if v, ok := ctx["statusCode"]; ok {
		e.StatusCode = coerceInt(v)
	}
	if v, ok := ctx["code"]; ok {
		e.Code = coerceInt(v)
	}
	if v, ok := ctx["href"]; ok {
		e.HRef = v.(string)
	}
	if v, ok := ctx["message"]; ok {
		e.Message = v.(string)
	}
	if v, ok := ctx["serverId"]; ok {
		e.Server = v.(string)
	}
}

// Error implements the builtin error interface.
func (e *ErrorInfo) Error() string {
	errorHref := e.HRef
	if errorHref == "" && e.Code != 0 {
		errorHref = fmt.Sprintf("https://help.ably.io/error/%d", e.Code)
	}
	var see string
	if !strings.Contains(e.Message, errorHref) {
		see = "See " + errorHref
	}
	return fmt.Sprintf("[ErrorInfo :%s code=%d statusCode=%d] %s", e.Message, e.Code, e.StatusCode, see)
}
