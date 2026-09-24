//go:build !integration
// +build !integration

package ably

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRealtime(t *testing.T) {
	tests := map[string]struct {
		options     []ClientOption
		expectedErr error
	}{
		"Can handle invalid key error when WithKey option is not provided": {
			options: []ClientOption{},
			expectedErr: &ErrorInfo{
				StatusCode: 400,
				Code:       40005,
				err:        errors.New("invalid key"),
			},
		},
		"Can create a new realtime client with a valid key": {
			options: []ClientOption{WithKey("abc:def")},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {

			client, err := NewRealtime(test.options...)
			assert.Equal(t, test.expectedErr, err)

			if client != nil {
				assert.Equal(t, "abc:def", client.Auth.opts().Key)
				// Assert all client fields have been populated
				assert.NotNil(t, client.rest)
				assert.NotNil(t, client.Auth)
				assert.NotNil(t, client.Channels)
				assert.NotNil(t, client.Channels.client)
				assert.NotNil(t, client.Channels.chans)
				assert.NotNil(t, client.Connection)
			}
		})
	}
}
