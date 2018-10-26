package ably

import "net/http"

// HTTPPaginatedResponse represent a response from an http request.
type HTTPPaginatedResponse struct {
	*PaginatedResult
	StatusCode   int         //spec HP4
	Success      bool        //spec HP5
	ErrorCode    int         //spec HP6
	ErrorMessage string      //spec HP7
	Headers      http.Header //spec HP8
}

func newHTTPPaginatedResult(path string, params *PaginateParams,
	query QueryFunc, log *LoggerOptions) (*HTTPPaginatedResponse, error) {
	p, err := newPaginatedResult(nil, paginatedRequest{typ: arrayTyp, path: path, params: params, query: query, logger: log, respCheck: func(_ *http.Response) error {
		return nil
	}})
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
	h.ErrorCode = p.errorCode
	h.ErrorMessage = p.errorMessage
	return h
}

// Next overides PaginatedResult.Next
// spec HP2
func (h *HTTPPaginatedResponse) Next() (*HTTPPaginatedResponse, error) {
	p, err := h.PaginatedResult.Next()
	if err != nil {
		return nil, err
	}
	return newHTTPPaginatedResultFromPaginatedResult(p), nil
}
