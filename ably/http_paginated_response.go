package ably

import (
	"net/http"
	"reflect"

	"github.com/ably/ably-go/ably/proto"
)

// HTTPPaginatedResponse represent a response from an http request.
type HTTPPaginatedResponse struct {
	*PaginatedResult
	StatusCode   int         //spec HP4
	Success      bool        //spec HP5
	ErrorCode    ErrorCode   //spec HP6
	ErrorMessage string      //spec HP7
	Headers      http.Header //spec HP8
}

func decodeHTTPPaginatedResult(opts *proto.ChannelOptions, typ reflect.Type, resp *http.Response) (interface{}, error) {
	var o interface{}
	err := decodeResp(resp, &o)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func newHTTPPaginatedResult(path string, params *PaginateParams,
	query queryFunc, log *LoggerOptions) (*HTTPPaginatedResponse, error) {
	p, err := newPaginatedResult(nil, paginatedRequest{typ: arrayTyp, path: path, params: params, query: query, logger: log, respCheck: func(_ *http.Response) error {
		return nil
	}, decoder: decodeHTTPPaginatedResult})
	if err != nil {
		return nil, err
	}
	//spec RSC19d
	return newHTTPPaginatedResultFromPaginatedResult(p), nil
}

func newHTTPPaginatedResultFromPaginatedResult(p *PaginatedResult) *HTTPPaginatedResponse {
	h := &HTTPPaginatedResponse{PaginatedResult: p}
	h.StatusCode = p.statusCode
	h.Success = p.success
	h.ErrorCode = ErrorCode(p.errorCode)
	h.ErrorMessage = p.errorMessage
	return h
}

// Next overrides PaginatedResult.Next
// spec HP2
func (h *HTTPPaginatedResponse) Next() (*HTTPPaginatedResponse, error) {
	p, err := h.PaginatedResult.Next()
	if err != nil {
		return nil, err
	}
	return newHTTPPaginatedResultFromPaginatedResult(p), nil
}
