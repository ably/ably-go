package rest

import (
	"io/ioutil"
	"net/http"
)

type RestHttpError struct {
	Msg      string
	Response *http.Response

	responseBody string
}

func (e *RestHttpError) Error() string {
	return e.Msg
}

func (e *RestHttpError) ResponseBody() string {
	if e.Response == nil {
		return ""
	}

	if body, err := ioutil.ReadAll(e.Response.Body); err == nil {
		defer e.Response.Body.Close()
		e.responseBody = string(body)
	}

	return e.responseBody
}
