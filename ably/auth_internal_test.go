//go:build !integration

package ably

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddParams(t *testing.T) {
	tests := map[string]struct {
		leftHandSide   url.Values
		rightHandSide  url.Values
		expectedResult url.Values
	}{
		"Can add a LeftHandSide value and a RightHandSidevalue": {
			leftHandSide:   url.Values{"Key1": []string{"Value1"}},
			rightHandSide:  url.Values{"Key2": []string{"Value2"}},
			expectedResult: url.Values{"Key1": []string{"Value1"}, "Key2": []string{"Value2"}},
		},
		"if a RightHandSide value is already in LeftHandSide it is not added again": {
			leftHandSide:   url.Values{"Key": []string{"Value"}},
			rightHandSide:  url.Values{"Key": []string{"Value"}},
			expectedResult: url.Values{"Key": []string{"Value"}},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := addParams(test.leftHandSide, test.rightHandSide)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestAddHeaders(t *testing.T) {
	tests := map[string]struct {
		leftHandSide   http.Header
		rightHandSide  http.Header
		expectedResult http.Header
	}{
		"Can add a LeftHandSide value and a RightHandSidevalue": {
			leftHandSide:   http.Header{"Key1": []string{"Value1"}},
			rightHandSide:  http.Header{"Key2": []string{"Value2"}},
			expectedResult: http.Header{"Key1": []string{"Value1"}, "Key2": []string{"Value2"}},
		},
		"if a RightHandSide value is already in LeftHandSide it is not added again": {
			leftHandSide:   http.Header{"Key": []string{"Value"}},
			rightHandSide:  http.Header{"Key": []string{"Value"}},
			expectedResult: http.Header{"Key": []string{"Value"}},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := addHeaders(test.leftHandSide, test.rightHandSide)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestNewAuth(t *testing.T) {
	tests := map[string]struct {
		client         *REST
		expectedMethod int
		expectedErr    error
	}{
		"Use authBasic for a client with a valid key and no token": {
			client: &REST{
				opts: &clientOptions{authOptions: authOptions{
					Key: "abc:def",
				}},
			},
			expectedMethod: authBasic,
			expectedErr:    nil,
		},
		"Use authToken for a client with a valid key and a token": {
			client: &REST{
				opts: &clientOptions{authOptions: authOptions{
					Key:   "abc:def",
					Token: "123",
				}},
			},
			expectedMethod: authToken,
			expectedErr:    nil,
		},
		"Can handle a client with an invalid key": {
			client: &REST{
				opts: &clientOptions{authOptions: authOptions{
					Key: "abcdef",
				}},
			},
			expectedErr: newError(ErrInvalidCredential, errInvalidKey),
		},
		"Can handle an invalid auth URL": {
			client: &REST{
				opts: &clientOptions{authOptions: authOptions{
					Key:     "abc:def",
					AuthURL: ":",
				}},
			},
			expectedErr: newError(40003, &url.Error{
				Op:  "parse",
				URL: ":",
				Err: errors.New(`missing protocol scheme`),
			}),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {

			result, err := newAuth(test.client)

			if result != nil {
				assert.Equal(t, test.expectedMethod, result.method)
			}

			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func TestClientID(t *testing.T) {
	tests := map[string]struct {
		auth           *Auth
		expectedResult string
	}{
		"Can handle a client ID": {
			auth: &Auth{
				clientID: "aClientID",
			},
			expectedResult: "aClientID",
		},
		"Can handle a wildcard client ID": {
			auth: &Auth{
				clientID: "*",
			},
			expectedResult: "",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := test.auth.ClientID()
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestClientIDForCheck(t *testing.T) {
	tests := map[string]struct {
		auth           *Auth
		expectedResult string
	}{
		"If the authorization method is authBasic, no client ID check is performed": {
			auth: &Auth{
				method:   authBasic,
				clientID: "aClientID",
			},
			expectedResult: "*",
		},
		"If the authorization method is authToken, client ID is used for check": {
			auth: &Auth{
				method:   authToken,
				clientID: "aClientID",
			},
			expectedResult: "aClientID",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := test.auth.clientIDForCheck()
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestUpdateClientID(t *testing.T) {
	tests := map[string]struct {
		auth           *Auth
		clientID       string
		expectedResult *Auth
	}{
		"Can update a client ID": {
			auth:           &Auth{clientID: "aClientID"},
			clientID:       "aNewClientID",
			expectedResult: &Auth{clientID: "aNewClientID"},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			test.auth.updateClientID(test.clientID)
			assert.Equal(t, test.expectedResult, test.auth)
		})
	}
}

func TestCreateTokenRequest(t *testing.T) {
	tests := map[string]struct {
		auth               *Auth
		params             *TokenParams
		opts               []AuthOption
		expectedTTL        int64
		expectedCapability string
		expectedTimestamp  int64
		expectedKeyName    string
		expectedClientID   string
		expectedErr        error
	}{
		"Can create a token request": {
			auth: &Auth{
				clientID: "aClientID",
				client: &REST{
					opts: &clientOptions{authOptions: authOptions{
						Key: "abc:def",
					}},
				},
			},
			params:             &TokenParams{Timestamp: int64(1645459276)},
			expectedTTL:        3600000,
			expectedCapability: `{"*":["*"]}`,
			expectedTimestamp:  1645459276,
			expectedKeyName:    "abc",
			expectedErr:        nil,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result, err := test.auth.CreateTokenRequest(test.params, test.opts...)
			assert.Equal(t, test.expectedTTL, result.TokenParams.TTL)
			assert.Equal(t, test.expectedCapability, result.TokenParams.Capability)
			assert.Equal(t, test.expectedTimestamp, result.Timestamp)
			assert.Equal(t, test.expectedKeyName, result.KeyName)
			assert.Equal(t, test.expectedClientID, result.ClientID)
			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func TestRequestToken(t *testing.T) {

	tests := map[string]struct {
		auth           *Auth
		params         *TokenParams
		opt            []AuthOption
		expectedResult *TokenDetails
		expectedErr    error
	}{
		"Can request successfully request token when a token is found in auth options": {
			auth: &Auth{
				client: &REST{
					log: logger{l: &stdLogger{mocklogger}},
				},
			},
			opt:            []AuthOption{AuthWithToken("aToken")},
			expectedResult: &TokenDetails{Token: "aToken", KeyName: "", Expires: 0, ClientID: "", Issued: 0, Capability: ""},
			expectedErr:    nil,
		},
		"Can handle an error when making a http request to request token ": {
			auth: &Auth{
				clientID: "aClientID",
				client: &REST{
					successFallbackHost: &fallbackCache{},
					log:                 logger{l: &stdLogger{mocklogger}},
					opts: &clientOptions{authOptions: authOptions{
						AuthURL: "foo.com",
						Key:     "abc:def",
					}},
				},
			},
			params:         &TokenParams{Timestamp: int64(1645459276)},
			opt:            nil,
			expectedResult: nil,
			expectedErr: newError(ErrErrorFromClientTokenCallback, &url.Error{
				Op:  "Get",
				URL: "foo.com?timestamp=1645459276",
				Err: errors.New(`unsupported protocol scheme ""`),
			}),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result, err := test.auth.RequestToken(context.Background(), test.params, test.opt...)
			assert.Equal(t, test.expectedResult, result)
			assert.Equal(t, test.expectedErr, err)
		})
	}
}
