package ably

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"strings"

	"github.com/ably/ably-go/ably/proto"
)

func toStatusCode(code int) int {
	switch status := code / 100; status {
	case 400, 401, 403, 404, 405, 500:
		return status
	default:
		return 0
	}
}

// ErrorInfo describes error returned from Ably API. It always has non-zero error
// code. It may contain underlying error value which caused the failure
// condition.
type ErrorInfo struct {
	StatusCode int
	Code       int
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
		see = "See " + errorHref
	}
	return fmt.Sprintf("[ErrorInfo :%s code=%d statusCode=%d] %s", msg, e.Code, e.StatusCode, see)
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

func newError(defaultCode int, err error) *ErrorInfo {
	switch err := err.(type) {
	case nil:
		return nil
	case *ErrorInfo:
		return err
	case net.Error:
		if err.Timeout() {
			return &ErrorInfo{Code: ErrTimeoutError, StatusCode: 500, err: err}
		}
	}
	return &ErrorInfo{
		Code:       defaultCode,
		StatusCode: toStatusCode(defaultCode),

		err: err,
	}
}

func newErrorf(code int, format string, v ...interface{}) *ErrorInfo {
	return newError(code, fmt.Errorf(format, v...))
}

func newErrorProto(err *proto.ErrorInfo) *ErrorInfo {
	if err == nil {
		return nil
	}
	return &ErrorInfo{
		Code:       err.Code,
		StatusCode: err.StatusCode,
		HRef:       err.HRef,
		err:        errors.New(err.Message),
	}
}

func code(err error) int {
	if e, ok := err.(*ErrorInfo); ok {
		return e.Code
	}
	return 0
}

func errFromUnprocessableBody(r io.Reader) error {
	errMsg, err := ioutil.ReadAll(r)
	if err == nil {
		err = errors.New(string(errMsg))
	}
	return newError(40000, err)
}

func checkValidHTTPResponse(resp *http.Response) error {
	type errorBody struct {
		Error proto.ErrorInfo `json:"error,omitempty" codec:"error,omitempty"`
	}
	if resp.StatusCode < 300 {
		return nil
	}
	defer resp.Body.Close()
	typ, _, mimeErr := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mimeErr != nil {
		return newError(50000, mimeErr)
	}
	if typ != protocolJSON && typ != protocolMsgPack {
		return errFromUnprocessableBody(resp.Body)
	}

	body := &errorBody{}
	if err := decode(typ, resp.Body, &body); err != nil {
		return newError(resp.StatusCode*100, errors.New(http.StatusText(resp.StatusCode)))
	}

	err := &ErrorInfo{
		Code:       body.Error.Code,
		StatusCode: body.Error.StatusCode,
	}
	if body.Error.Message != "" {
		err.err = errors.New(body.Error.Message)
	}
	if err.Code == 0 && err.StatusCode == 0 {
		err.Code, err.StatusCode = resp.StatusCode*100, resp.StatusCode
	}
	return err
}
