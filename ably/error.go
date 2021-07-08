package ably

import (
	"errors"
	"fmt"
	"io/ioutil"
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

// ErrorInfo describes error returned from Ably API. It always has non-zero error
// code. It may contain underlying error value which caused the failure
// condition.
type ErrorInfo struct {
	StatusCode int
	Code       ErrorCode
	HRef       string
	Cause      *ErrorInfo

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
	errMsg, err := ioutil.ReadAll(resp.Body)
	if err == nil {
		err = errors.New(string(errMsg))
	}
	return &ErrorInfo{Code: ErrBadRequest, StatusCode: resp.StatusCode, err: err}
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
