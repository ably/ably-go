package ably

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

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

func checkValidHTTPResponse(resp *http.Response) *Error {
	type errorBody struct {
		Error proto.ErrorInfo `json:"error,omitempty" codec:"error,omitempty"`
	}
	if resp.StatusCode < 300 {
		return nil
	}
	defer resp.Body.Close()
	body := &errorBody{}
	if e := json.NewDecoder(resp.Body).Decode(body); e != nil {
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
