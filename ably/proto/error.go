package proto

import "fmt"

type Error struct {
	StatusCode int    `json:"statusCode" msgpack:"statusCode"`
	Code       int    `json:"code" msgpack:"code"`
	Message    string `json:"message" msgpack:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d: %d %s", e.StatusCode, e.Code, e.Message)
}
