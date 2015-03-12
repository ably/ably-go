package protocol

import "fmt"

type Error struct {
	StatusCode int    `json:"statusCode"`
	Code       int    `json:"code"`
	Message    string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d: %d %s", e.StatusCode, e.Code, e.Message)
}
