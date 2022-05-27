//go:build !integration
// +build !integration

package ably

import (
	"errors"
	// "context"
	// "errors"
	// "net/http"
	// "net/url"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRealtime(t *testing.T) {
	tests := map[string]struct {
		options        []ClientOption
		expectedErr    error
		expectedResult *Realtime
	}{
		"Can handle invalid key error when WithKey option is not provided": {
			options: []ClientOption{},
			expectedErr: &ErrorInfo{
				StatusCode: 400,
				Code:       40005,
				err:        errors.New("invalid key"),
			},
		},
		// "Can create a new realtime client with a valid key": {
		// 	options: []ClientOption{WithKey("abc:def")},

		// 	expectedResult: &Realtime{},
		// },

	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			fmt.Println(test)

			result, err := NewRealtime(test.options...)
			fmt.Printf("%+v\n", err)
			assert.Equal(t, test.expectedResult, result)
			assert.Equal(t, test.expectedErr, err)
		})
	}
}

// bug creating a NewRealtime passing in nil causes a panic. Should handle this gracefully.
