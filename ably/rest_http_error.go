package ably

import (
	"io/ioutil"
	"net/http"
)

type RestHttpError struct {
	Msg          string
	ResponseBody string

	resp *http.Response
}

func NewRestHttpError(resp *http.Response, message string) *RestHttpError {
	err := &RestHttpError{Msg: message, resp: resp}
	err.readBody()
	err.resp = nil
	return err
}

func (e *RestHttpError) Error() string {
	return e.Msg
}

func (e *RestHttpError) readBody() {
	if e.resp == nil {
		return
	}
	body, err := ioutil.ReadAll(e.resp.Body)
	e.resp.Body.Close()
	if err == nil {
		e.ResponseBody = string(body)
	}
}
