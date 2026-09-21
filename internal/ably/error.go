package ably

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
)

func (code ErrorCode) toStatusCode() int {
	switch status := int(code) / 100; status {
	case
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusInternalServerError:
		return status
	default:
		return 0
	}
}

func codeFromStatus(statusCode int) ErrorCode {
	switch code := ErrorCode(statusCode * 100); code {
	case
		ErrBadRequest,
		ErrUnauthorized,
		ErrForbidden,
		ErrNotFound,
		ErrMethodNotAllowed,
		ErrInternalError:
		return code
	default:
		return ErrNotSet
	}
}

// ErrorInfo is a generic Ably error object that contains an Ably-specific status code, and a generic status code.
// Errors returned from the Ably server are compatible with the ErrorInfo structure and should result in errors
// that inherit from ErrorInfo. It may contain underlying error value which caused the failure.
type ErrorInfo struct {
	// StatusCode is a http Status code corresponding to this error, where applicable (TI1).
	StatusCode int
	// Code is the standard [ably error code]
	// [ably error code]: https://github.com/ably/ably-common/blob/main/protocol/errors.json (TI1).
	Code ErrorCode
	// HRef is included for REST responses to provide a URL for additional help on the error code (TI4).
	HRef string
	// Cause provides Information pertaining to what caused the error where available (TI1).
	Cause *ErrorInfo
	// err is the application-level error we're wrapping, or just a message.
	// If Cause is non-nil, err == *Cause.
	err error
}

// Error implements the builtin error interface.
func (e ErrorInfo) Error() string {
	errorHref := e.HRef
	if errorHref == "" && e.Code != 0 {
		errorHref = fmt.Sprintf("https://help.ably.io/error/%d", e.Code)
	}
	msg := e.Message()
	var see string
	if !strings.Contains(msg, errorHref) {
		see = " See " + errorHref
	}
	return fmt.Sprintf("[ErrorInfo :%s code=%d %[2]v statusCode=%d]%s", msg, e.Code, e.StatusCode, see)
}

// Unwrap implements the implicit interface that errors.Unwrap understands.
func (e ErrorInfo) Unwrap() error {
	return e.err
}

// Message returns the undecorated error message.
func (e ErrorInfo) Message() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func newError(defaultCode ErrorCode, err error) *ErrorInfo {
	switch err := err.(type) {
	case *ErrorInfo:
		return err
	case net.Error:
		if err.Timeout() {
			return &ErrorInfo{Code: ErrTimeoutError, StatusCode: 500, err: err}
		}
	}
	return &ErrorInfo{
		Code:       defaultCode,
		StatusCode: defaultCode.toStatusCode(),

		err: err,
	}
}

func newErrorf(code ErrorCode, format string, v ...interface{}) *ErrorInfo {
	return newError(code, fmt.Errorf(format, v...))
}

func newErrorFromProto(err *errorInfo) *ErrorInfo {
	if err == nil {
		return nil
	}
	return &ErrorInfo{
		Code:       ErrorCode(err.Code),
		StatusCode: err.StatusCode,
		HRef:       err.HRef,
		err:        errors.New(err.Message),
	}
}

func (e *ErrorInfo) unwrapNil() error {
	if e == nil {
		return nil
	}
	return e
}

func code(err error) ErrorCode {
	if e, ok := err.(*ErrorInfo); ok {
		return e.Code
	}
	return ErrNotSet
}

func statusCode(err error) int {
	if e, ok := err.(*ErrorInfo); ok {
		return e.StatusCode
	}
	return 0
}

func errFromUnprocessableBody(resp *http.Response) error {
	errMsg, err := io.ReadAll(resp.Body)
	if err == nil {
		err = errors.New(string(errMsg))
	}
	return &ErrorInfo{Code: ErrBadRequest, StatusCode: resp.StatusCode, err: err}
}

func isTimeoutOrDnsErr(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() { // RSC15l2
			return true
		}
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) // RSC15l1
}

func checkValidHTTPResponse(resp *http.Response) error {
	type errorBody struct {
		Error errorInfo `json:"error,omitempty" codec:"error,omitempty"`
	}
	if resp.StatusCode < 300 {
		return nil
	}
	defer resp.Body.Close()
	typ, _, mimeErr := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mimeErr != nil {
		return &ErrorInfo{
			Code:       50000,
			StatusCode: resp.StatusCode,
			err:        mimeErr,
		}
	}
	if typ != protocolJSON && typ != protocolMsgPack {
		return errFromUnprocessableBody(resp)
	}

	body := &errorBody{}
	if err := decode(typ, resp.Body, &body); err != nil {
		return newError(codeFromStatus(resp.StatusCode), errors.New(http.StatusText(resp.StatusCode)))
	}

	err := newErrorFromProto(&body.Error)
	if body.Error.Message != "" {
		err.err = errors.New(body.Error.Message)
	}
	if err.Code == 0 && err.StatusCode == 0 {
		err.Code, err.StatusCode = codeFromStatus(resp.StatusCode), resp.StatusCode
	}
	return err
}
