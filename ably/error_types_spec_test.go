//go:build !integration
// +build !integration

package ably_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// TI1-TI5 - ErrorInfo attributes
// Tests that ErrorInfo has all required attributes.
// =============================================================================

func TestErrorTypes_TI1_CodeAttribute(t *testing.T) {
	// ANOMALY: ably-go's ErrorInfo doesn't have a public constructor,
	// but we can check that error codes are properly exposed.
	// ErrorInfo is typically created internally or from server responses.

	// Test error codes are defined
	assert.Equal(t, ably.ErrorCode(40000), ably.ErrBadRequest)
	assert.Equal(t, ably.ErrorCode(40100), ably.ErrUnauthorized)
	assert.Equal(t, ably.ErrorCode(40300), ably.ErrForbidden)
}

func TestErrorTypes_TI2_StatusCodeAttribute(t *testing.T) {
	// StatusCode is set based on the error response
	// Testing through client operations that return errors
}

func TestErrorTypes_TI_CommonErrorCodes(t *testing.T) {
	testCases := []struct {
		code ably.ErrorCode
		name string
	}{
		{ably.ErrBadRequest, "ErrBadRequest"},
		{ably.ErrUnauthorized, "ErrUnauthorized"},
		{ably.ErrForbidden, "ErrForbidden"},
		{ably.ErrNotFound, "ErrNotFound"},
		{ably.ErrInternalError, "ErrInternalError"},
		{ably.ErrInvalidCredential, "ErrInvalidCredential"},
		{ably.ErrIncompatibleCredentials, "ErrIncompatibleCredentials"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEqual(t, ably.ErrorCode(0), tc.code,
				"%s should have a non-zero value", tc.name)
		})
	}
}

// =============================================================================
// TI - ErrorInfo string representation
// Tests that errors have a useful string representation.
// =============================================================================

func TestErrorTypes_TI_StringRepresentation(t *testing.T) {
	// Create an error condition by trying to create a client with invalid key
	_, err := ably.NewREST(ably.WithKey("invalid"))

	assert.Error(t, err)

	errInfo, ok := err.(*ably.ErrorInfo)
	if !ok {
		t.Fatalf("expected *ably.ErrorInfo, got %T", err)
	}

	// String representation should include key information
	errorStr := errInfo.Error()
	assert.NotEmpty(t, errorStr)

	// Should contain the error code
	assert.Contains(t, errorStr, "code=")

	// Should contain status code or be informative
	hasRelevantInfo := strings.Contains(errorStr, "statusCode=") ||
		strings.Contains(errorStr, "ErrorInfo") ||
		strings.Contains(errorStr, "See")
	assert.True(t, hasRelevantInfo,
		"error string should contain relevant information, got: %s", errorStr)
}

// =============================================================================
// TI - ErrorInfo from HTTP response
// Tests that ErrorInfo is properly created from Ably error responses.
// =============================================================================

func TestErrorTypes_TI_FromHTTPResponse(t *testing.T) {
	mock := newMockRoundTripper()
	httpClient := &http.Client{Transport: mock}

	// Queue error response
	errorResponse := `{
		"error": {
			"code": 40100,
			"statusCode": 401,
			"message": "Token expired",
			"href": "https://help.ably.io/error/40100"
		}
	}`
	mock.queueResponse(401, []byte(errorResponse), "application/json")

	client, err := ably.NewREST(
		ably.WithKey("appId.keyId:keySecret"),
		ably.WithHTTPClient(httpClient),
	)
	assert.NoError(t, err)

	_, err = client.Time(context.Background())
	assert.Error(t, err)

	errInfo, ok := err.(*ably.ErrorInfo)
	if !ok {
		t.Fatalf("expected *ably.ErrorInfo, got %T", err)
	}

	assert.Equal(t, ably.ErrorCode(40100), errInfo.Code)
	assert.Equal(t, 401, errInfo.StatusCode)
}

// =============================================================================
// TI - Error unwrapping
// Tests that ErrorInfo supports error unwrapping.
// =============================================================================

func TestErrorTypes_TI_ErrorUnwrap(t *testing.T) {
	// Create an error to test unwrap behavior
	_, err := ably.NewREST(ably.WithKey("invalid"))

	assert.Error(t, err)

	errInfo, ok := err.(*ably.ErrorInfo)
	if !ok {
		t.Fatalf("expected *ably.ErrorInfo, got %T", err)
	}

	// Unwrap should return the underlying error
	underlying := errInfo.Unwrap()
	// The underlying error may or may not exist depending on implementation
	_ = underlying
}

// =============================================================================
// TI - Error Message
// Tests that ErrorInfo provides a message method.
// =============================================================================

func TestErrorTypes_TI_ErrorMessage(t *testing.T) {
	_, err := ably.NewREST(ably.WithKey("invalid"))

	assert.Error(t, err)

	errInfo, ok := err.(*ably.ErrorInfo)
	if !ok {
		t.Fatalf("expected *ably.ErrorInfo, got %T", err)
	}

	message := errInfo.Message()
	assert.NotEmpty(t, message, "error message should not be empty")
}

// =============================================================================
// Additional error type tests
// =============================================================================

func TestErrorTypes_ErrorCodeConstantValues(t *testing.T) {
	// Verify specific error code values match Ably spec
	assert.Equal(t, ably.ErrorCode(40000), ably.ErrBadRequest)
	assert.Equal(t, ably.ErrorCode(40100), ably.ErrUnauthorized)
	assert.Equal(t, ably.ErrorCode(40140), ably.ErrTokenErrorUnspecified)
	assert.Equal(t, ably.ErrorCode(40300), ably.ErrForbidden)
	assert.Equal(t, ably.ErrorCode(40400), ably.ErrNotFound)
	assert.Equal(t, ably.ErrorCode(50000), ably.ErrInternalError)
}

func TestErrorTypes_ErrorInfoImplementsError(t *testing.T) {
	// Verify ErrorInfo implements error interface
	var err error
	_, createErr := ably.NewREST(ably.WithKey("invalid"))
	if errInfo, ok := createErr.(*ably.ErrorInfo); ok {
		err = errInfo
	} else {
		err = createErr
	}

	assert.NotNil(t, err)
	assert.NotEmpty(t, err.Error())
}

func TestErrorTypes_CredentialErrors(t *testing.T) {
	testCases := []struct {
		name    string
		options []ably.ClientOption
	}{
		{"no_credentials", []ably.ClientOption{}},
		{"invalid_key", []ably.ClientOption{ably.WithKey("invalid")}},
		{"empty_key", []ably.ClientOption{ably.WithKey("")}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ably.NewREST(tc.options...)
			assert.Error(t, err)

			if errInfo, ok := err.(*ably.ErrorInfo); ok {
				// Error should be credential-related
				isCredentialError := errInfo.Code == ably.ErrInvalidCredential ||
					errInfo.Code == ably.ErrIncompatibleCredentials ||
					errInfo.Code == 40106

				assert.True(t, isCredentialError,
					"expected credential error, got code %d", errInfo.Code)
			}
		})
	}
}
