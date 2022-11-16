package ably

// errorInfo describes an error object returned via ProtocolMessage.
type errorInfo struct {
	StatusCode int    `json:"statusCode,omitempty" codec:"statusCode,omitempty"`
	Code       int    `json:"code,omitempty" codec:"code,omitempty"`
	HRef       string `json:"href,omitempty" codec:"href,omitempty"` //spec TI4
	Message    string `json:"message,omitempty" codec:"message,omitempty"`
	Server     string `json:"serverId,omitempty" codec:"serverId,omitempty"`
}

func (e *errorInfo) FromMap(ctx map[string]interface{}) {
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
