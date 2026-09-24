//go:build !integration
// +build !integration

package ably

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeFromStatus(t *testing.T) {
	tests := map[string]struct {
		statusCode     int
		expectedResult ErrorCode
	}{
		"Can convert status code 400 to ably error code 40000": {
			statusCode:     400,
			expectedResult: 40000,
		},
		"Can convert status code 401 to ably error code 40100": {
			statusCode:     401,
			expectedResult: 40100,
		},
		"Can convert status code 403 to ably error code 40300": {
			statusCode:     403,
			expectedResult: 40300,
		},
		"Can convert status code 404 to ably error code 40400": {
			statusCode:     404,
			expectedResult: 40400,
		},
		"Can convert status code 405 to ably error code 40500": {
			statusCode:     405,
			expectedResult: 40500,
		},
		"Can convert status code 500 to ably error code 50000": {
			statusCode:     500,
			expectedResult: 50000,
		},
		"All other status codes return ErrNotSet": {
			statusCode:     412,
			expectedResult: ErrNotSet,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := codeFromStatus(test.statusCode)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestNewErrorFromProto(t *testing.T) {
	tests := map[string]struct {
		errorInfo      *errorInfo
		expectedResult *ErrorInfo
	}{
		"internal errorInfo can be converted to public ErrorInfo": {
			errorInfo: &errorInfo{
				StatusCode: 500,
				Code:       50000,
				HRef:       "http://www.test.com",
				Message:    "message",
				Server:     "serverID",
			},
			expectedResult: &ErrorInfo{
				StatusCode: 500,
				Code:       50000,
				HRef:       "http://www.test.com",
				err:        errors.New("message"),
			},
		},
		"If errorInfo is nil, no result is returned": {
			errorInfo:      nil,
			expectedResult: nil,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := newErrorFromProto(test.errorInfo)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestUnwrapNil(t *testing.T) {
	tests := map[string]struct {
		errorInfo      *ErrorInfo
		expectedResult error
	}{
		"If errorInfo is not nil, it returns itself": {
			errorInfo: &ErrorInfo{
				StatusCode: 500,
				Code:       50000,
				HRef:       "http://www.test.com",
				err:        errors.New("message"),
			},
			expectedResult: &ErrorInfo{
				StatusCode: 500,
				Code:       50000,
				HRef:       "http://www.test.com",
				err:        errors.New("message"),
			},
		},
		"If errorInfo is nil, nil is returned": {
			errorInfo:      nil,
			expectedResult: nil,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := test.errorInfo.unwrapNil()
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestCode(t *testing.T) {
	tests := map[string]struct {
		err            error
		expectedResult ErrorCode
	}{
		"Can return a code for type *ErrorInfo": {
			err: &ErrorInfo{
				StatusCode: 500,
				Code:       50000,
				HRef:       "http://www.test.com",
				err:        errors.New("message"),
			},
			expectedResult: 50000,
		},
		"Returns ErrNotSet for other error types": {
			err:            errors.New("an error"),
			expectedResult: ErrNotSet,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := code(test.err)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestStatusCode(t *testing.T) {
	tests := map[string]struct {
		err            error
		expectedResult int
	}{
		"Can return a status code for type *ErrorInfo": {
			err: &ErrorInfo{
				StatusCode: 500,
				Code:       50000,
				HRef:       "http://www.test.com",
				err:        errors.New("message"),
			},
			expectedResult: 500,
		},
		"Returns 0 for other error types": {
			err:            errors.New("an error"),
			expectedResult: 0,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := statusCode(test.err)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestCheckValidHTTPResponse(t *testing.T) {
	tests := map[string]struct {
		response       *http.Response
		expectedResult error
	}{
		"No error is returned if response status code is less than 300": {
			response: &http.Response{
				StatusCode: 200,
			},
			expectedResult: nil,
		},
		"Can handle a mimeError if the Content-Type header is invalid": {
			response: &http.Response{
				Header:     http.Header{"Content-Type": []string{"/"}},
				StatusCode: 500,
				Body:       io.NopCloser(strings.NewReader("a response body")),
			},
			expectedResult: &ErrorInfo{
				Code:       50000,
				StatusCode: 500,
				err:        errors.New("mime: no media type"),
			},
		},
		"Can handle an unprocessable body if the Content-Type header is not application/json or application/x-msgpack": {
			response: &http.Response{
				Header:     http.Header{"Content-Type": []string{"application/xml"}},
				StatusCode: 400,
				Body:       io.NopCloser(strings.NewReader("a response body")),
			},
			expectedResult: &ErrorInfo{
				Code:       40000,
				StatusCode: 400,
				err:        errors.New("a response body"),
			},
		},
		"Can handle an error from a bad request response": {
			response: &http.Response{
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				StatusCode: 400,
				Body:       io.NopCloser(strings.NewReader("[]")),
			},
			expectedResult: &ErrorInfo{
				Code:       40000,
				StatusCode: 400,
				err:        errors.New("Bad Request"),
			},
		},
		"Can handle an error from an internal server error response": {
			response: &http.Response{
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				StatusCode: 500,
				Body:       io.NopCloser(strings.NewReader("[]")),
			},
			expectedResult: &ErrorInfo{
				Code:       50000,
				StatusCode: 500,
				err:        errors.New("Internal Server Error"),
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := checkValidHTTPResponse(test.response)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}
