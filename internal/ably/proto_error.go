package ably

import (
	"strconv"
	"strings"
)

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

func (e *errorInfo) String() string {
	if e == nil {
		return "(nil)"
	}

	const (
		labelMsg        = "msg="
		labelCode       = "Code="
		labelStatusCode = "StatusCode="
		labelHref       = "Href="
		labelServer     = "Server="
	)

	size := len("(") + len(labelMsg) + len(e.Message) +
		len(","+labelCode) + len(strconv.Itoa(e.Code)) +
		len(","+labelStatusCode) + len(strconv.Itoa(e.StatusCode)) + len(")")

	if e.HRef != "" {
		size += len(","+labelHref) + len(e.HRef)
	}

	if e.Server != "" {
		size += len(","+labelServer) + len(e.Server)
	}

	var b strings.Builder
	b.Grow(size)

	b.WriteString("(" + labelMsg)
	b.WriteString(e.Message)
	b.WriteString("," + labelCode)
	b.WriteString(strconv.Itoa(e.Code))
	b.WriteString("," + labelStatusCode)
	b.WriteString(strconv.Itoa(e.StatusCode))

	if e.HRef != "" {
		b.WriteString("," + labelHref)
		b.WriteString(e.HRef)
	}

	if e.Server != "" {
		b.WriteString("," + labelServer)
		b.WriteString(e.Server)
	}

	b.WriteString(")")

	return b.String()
}
