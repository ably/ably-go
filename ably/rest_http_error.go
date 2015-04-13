package ably

import (
	"io/ioutil"
	"net/http"
)

type RestHttpError struct {
	Msg      string
	Response *http.Response

	ResponseBody string
}

func NewRestHttpError(response *http.Response, message string) *RestHttpError {
	err := &RestHttpError{Response: response, Msg: message}
	err.readBody()
	return err
}

func (e *RestHttpError) Error() string {
	return e.Msg
}

func (e *RestHttpError) readBody() {
	if e.Response == nil {
		return
	}

	if body, err := ioutil.ReadAll(e.Response.Body); err == nil {
		defer e.Response.Body.Close()
		e.ResponseBody = string(body)
	}
}
