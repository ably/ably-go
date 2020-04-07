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

// Error describes error returned from Ably API. It always has non-zero error
// code. It may contain underlying error value which caused the failure
// condition.
type Error struct {
	Code       int    // internal error code
	StatusCode int    // HTTP status code
	Err        error  // underlying error responsible for the failure; may be nil
	Server     string // non-empty ID of the Ably server which the error was received from
}

// Error implements builtin error interface.
func (err *Error) Error() string {
	if err.Err != nil {
		return fmt.Sprintf("%s (status=%d, internal=%d)", err.Err, err.StatusCode, err.Code)
	}
	return errCodeText[err.Code]
}

func newError(code int, err error) *Error {
	switch err := err.(type) {
	case *Error:
		if _, ok := err.Err.(genericError); ok {
			// If err was returned from http.Response but we were unable to
			// parse the internal error code, we overwrite it here.
			err.Code = code
			return err
		}
		return err
	case net.Error:
		if err.Timeout() {
			return &Error{Code: ErrTimeoutError, StatusCode: 500, Err: err}
		}
	}
	return &Error{
		Code:       code,
		StatusCode: toStatusCode(code),
		Err:        err,
	}
}

func newErrorf(code int, format string, v ...interface{}) *Error {
	return &Error{
		Code:       code,
		StatusCode: toStatusCode(code),
		Err:        fmt.Errorf(format, v...),
	}
}

func newErrorProto(err *proto.ErrorInfo) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:       err.Code,
		StatusCode: err.StatusCode,
		Err:        errors.New(err.Message),
	}
}

type genericError error

func code(err error) int {
	if e, ok := err.(*Error); ok {
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
		return &Error{
			Code:       50000,
			StatusCode: resp.StatusCode,
			Err:        genericError(errors.New(http.StatusText(resp.StatusCode))),
		}
	}

	err := &Error{
		Code:       body.Error.Code,
		StatusCode: body.Error.StatusCode,
		Server:     body.Error.Server,
	}
	if body.Error.Message != "" {
		err.Err = errors.New(body.Error.Message)
	}
	if err.Code == 0 && err.StatusCode == 0 {
		err.Code, err.StatusCode = resp.StatusCode*100, resp.StatusCode
	}
	return err
}

// ErrorInfoV12 is an error produced by the Ably library.
type ErrorInfoV12 struct {
	// TODO: remove duplication with proto.ErrorInfo

	StatusCode int
	Code       int
	HRef       string
	Message    string
	Server     string
}

// Error implements the builtin error interface.
func (e ErrorInfoV12) Error() string {
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

func errorToErrorInfo(err error) *ErrorInfoV12 {
	switch err := err.(type) {
	case nil:
		return nil
	case *proto.ErrorInfo:
		return (*ErrorInfoV12)(err)
	case ErrorInfoV12:
		return &err
	case *ErrorInfoV12:
		return err
	case *Error:
		var msg string
		if err.Err != nil {
			msg = err.Err.Error()
		} else {
			msg = err.Error()
		}
		return &ErrorInfoV12{
			Code:       err.Code,
			StatusCode: err.StatusCode,
			Message:    msg,
			Server:     err.Server,
		}
	default:
		return &ErrorInfoV12{
			Message: err.Error(),
		}
	}
}
