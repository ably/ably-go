package ably

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/ably/ably-go/ably/proto"
)

func toStatusCode(code int) int {
	switch status := code / 100; status {
	case 400, 401, 403, 404, 405, 500:
		return status
	default:
		return 0
	}
}

// Error describes error returned from Ably API. It always has non-zero error
// code. It may contain underlying error value which caused the failure
// condition.
type Error struct {
	Code       int   // internal error code
	StatusCode int   // HTTP status code
	Err        error // underlying error responsible for the failure; may be nil
}

// Error implements builtin error interface.
func (err *Error) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return errCodeText[err.Code]
}

func newError(code int, err error) *Error {
	switch err := err.(type) {
	case *Error:
		return err
	case net.Error:
		if err.Timeout() {
			return &Error{Code: ErrCodeTimeout, StatusCode: 500, Err: err}
		}
	}
	return &Error{
		Code:       code,
		StatusCode: toStatusCode(code),
		Err:        err,
	}
}

func newErrorf(code int, format string, v ...interface{}) *Error {
	return &Error{
		Code:       code,
		StatusCode: toStatusCode(code),
		Err:        fmt.Errorf(format, v...),
	}
}

func newErrorProto(err *proto.Error) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:       err.Code,
		StatusCode: err.StatusCode,
		Err:        errors.New(err.Message),
	}
}

func checkValidHTTPResponse(resp *http.Response) error {
	if resp.StatusCode < 300 {
		return nil
	}
	defer resp.Body.Close()
	body := &proto.Error{}
	if e := json.NewDecoder(resp.Body).Decode(body); e != nil {
		return &Error{Code: 50000, StatusCode: 500, Err: e}
	}
	err := &Error{Code: body.Code, StatusCode: body.StatusCode}
	if body.Message != "" {
		err.Err = errors.New(body.Message)
	}
	if err.Code == 0 && err.StatusCode == 0 {
		err.Code, err.StatusCode = resp.StatusCode*100, resp.StatusCode
	}
	return err
}

const (
	// generic error codes
	ErrCodeNoError = 10000

	// error codes for HTTP 400
	ErrCodeBadRequest                   = 40000
	ErrCodeInvalidRequestBody           = 40001
	ErrCodeInvalidParameterName         = 40002
	ErrCodeInvalidParameterValue        = 40003
	ErrCodeInvalidHeader                = 40004
	ErrCodeInvalidCredential            = 40005
	ErrCodeInvalidConnectionID          = 40006
	ErrCodeInvalidMessageID             = 40007
	ErrCodeInvalidContentLength         = 40008
	ErrCodeMaximumMessageLengthExceeded = 40009
	ErrCodeInvalidChannelName           = 40010

	// error codes for HTTP 401
	ErrCodeUnauthorized                                = 40100
	ErrCodeInvalidCredentials                          = 40101
	ErrCodeIncompatibleCredentials                     = 40102
	ErrCodeInvalidUseOfBasicAuthOverNonTLSTransport    = 40103
	ErrCodeAccountDisabled                             = 40110
	ErrCodeAccountBlockedConnectionLimitsExceeded      = 40111
	ErrCodeAccountBlockedMessageLimitsExceeded         = 40112
	ErrCodeAccountBlocked                              = 40113
	ErrCodeApplicationDisabled                         = 40120
	ErrCodeKeyDisabled                                 = 40130
	ErrCodeKeyRevoked                                  = 40131
	ErrCodeKeyExpired                                  = 40132
	ErrCodeTokenExpired                                = 40140
	ErrCodeConnectionBlockedLimitsExceeded             = 40150
	ErrCodeOperationNotPermittedWithProvidedCapability = 40160
	ErrCodeClientTokenCallback                         = 40170

	// error codes for HTTP 403
	ErrCodeForbidden                         = 40300
	ErrCodeAccountDoesNotPermitTlsConnection = 40310
	ErrCodeOperationRequiresTlsConnection    = 40311
	ErrCodeApplicationRequiresAuthentication = 40320

	// error codes for HTTP 404
	ErrCodeNotFound = 40400

	// error codes for HTTP 405
	ErrCodeMethodNotAllowed = 40500

	// error codes for HTTP 500
	ErrCodeInternal           = 50000
	ErrCodeInternalChannel    = 50001
	ErrCodeInternalConnection = 50002
	ErrCodeTimeout            = 50003

	// error codes for webhook failures
	ErrCodeWebhookOperationFailed                                       = 70000
	ErrCodeWebhookOperationFailedPostOperationFailed                    = 70001
	ErrCodeWebhookOperationFailedPostOperationReturnedUnexpectedErrCode = 70002
	ErrCodeWebhookChannelRecoveryMessagesExpired                        = 70003
	ErrCodeWebhookChannelRecoveryMessageLimitExceeded                   = 70004
	ErrCodeWebhookChannelRecoveryHistoryIncomplete                      = 70005

	// error codes for connection failures
	ErrCodeConnectionFailed                                       = 80000
	ErrCodeConnectionFailedNoCompatibleTransport                  = 80001
	ErrCodeConnectionSuspended                                    = 80002
	ErrCodeDisconnected                                           = 80003
	ErrCodeAlreadyConnected                                       = 80004
	ErrCodeInvalidConnectionIDRemoteNotFound                      = 80005
	ErrCodeConnectionRecoveryMessagesExpired                      = 80006
	ErrCodeConnectionRecoveryMessageLimitExceeded                 = 80007
	ErrCodeConnectionRecoveryConnectionExpired                    = 80008
	ErrCodeConnectionNotEstablishedNoTransportHandle              = 80009
	ErrCodeInvalidOperationInvalidTransportHandle                 = 80010
	ErrCodeConnectionRecoveryIncompatibleAuthParams               = 80011
	ErrCodeConnectionRecoveryInvalidOrUnspecifiedConnectionSerial = 80012
	ErrCodeProtocol                                               = 80013
	ErrCodeConnectionTimedOut                                     = 80014
	ErrCodeIncompatibleConnectionParameters                       = 80015
	ErrCodeOperationOnSupersededTransport                         = 80016

	// error codes for channel failures
	ErrCodeChannelOperationFailed                           = 90000
	ErrCodeChannelOperationFailedInvalidChannelState        = 90001
	ErrCodeChannelOperationFailedEpochExpiredOrNeverExisted = 90002
	ErrCodeChannelRecoveryMessagesExpired                   = 90003
	ErrCodeChannelRecoveryMessageLimitExceeded              = 90004
	ErrCodeChannelRecoveryNoMatchingEpoch                   = 90005
	ErrCodeChannelRecoveryUnboundedRequest                  = 90006
	ErrCodeUnableToEnterPresenceChannelNoClientID           = 91000
	ErrCodeUnableToEnterPresenceChannelInvalidChannelState  = 91001
	ErrCodeUnableToLeavePresenceChannelThatIsNotEntered     = 91002
)

var errCodeText = map[int]string{
	ErrCodeNoError:                                                      "no error",
	ErrCodeBadRequest:                                                   "bad request",
	ErrCodeInvalidRequestBody:                                           "invalid request body",
	ErrCodeInvalidParameterName:                                         "invalid parameter name",
	ErrCodeInvalidParameterValue:                                        "invalid parameter value",
	ErrCodeInvalidHeader:                                                "invalid header",
	ErrCodeInvalidCredential:                                            "invalid credential",
	ErrCodeInvalidConnectionID:                                          "invalid connection id",
	ErrCodeInvalidMessageID:                                             "invalid message id",
	ErrCodeInvalidContentLength:                                         "invalid content length",
	ErrCodeMaximumMessageLengthExceeded:                                 "maximum message length exceeded",
	ErrCodeInvalidChannelName:                                           "invalid channel name",
	ErrCodeUnauthorized:                                                 "unauthorized",
	ErrCodeInvalidCredentials:                                           "invalid credentials",
	ErrCodeIncompatibleCredentials:                                      "incompatible credentials",
	ErrCodeInvalidUseOfBasicAuthOverNonTLSTransport:                     "invalid use of Basic auth over non-TLS transport",
	ErrCodeAccountDisabled:                                              "account disabled",
	ErrCodeAccountBlockedConnectionLimitsExceeded:                       "account blocked (connection limits exceeded)",
	ErrCodeAccountBlockedMessageLimitsExceeded:                          "account blocked (message limits exceeded)",
	ErrCodeAccountBlocked:                                               "account blocked",
	ErrCodeApplicationDisabled:                                          "application disabled",
	ErrCodeKeyDisabled:                                                  "key disabled",
	ErrCodeKeyRevoked:                                                   "key revoked",
	ErrCodeKeyExpired:                                                   "key expired",
	ErrCodeTokenExpired:                                                 "token expired",
	ErrCodeConnectionBlockedLimitsExceeded:                              "connection blocked (limits exceeded)",
	ErrCodeOperationNotPermittedWithProvidedCapability:                  "operation not permitted with provided capability",
	ErrCodeClientTokenCallback:                                          "error from client token callback",
	ErrCodeForbidden:                                                    "forbidden",
	ErrCodeAccountDoesNotPermitTlsConnection:                            "account does not permit tls connection",
	ErrCodeOperationRequiresTlsConnection:                               "operation requires tls connection",
	ErrCodeApplicationRequiresAuthentication:                            "application requires authentication",
	ErrCodeNotFound:                                                     "not found",
	ErrCodeMethodNotAllowed:                                             "method not allowed",
	ErrCodeInternal:                                                     "internal error",
	ErrCodeInternalChannel:                                              "internal channel error",
	ErrCodeInternalConnection:                                           "internal connection error",
	ErrCodeTimeout:                                                      "timeout error",
	ErrCodeWebhookOperationFailed:                                       "webhook operation failed",
	ErrCodeWebhookOperationFailedPostOperationFailed:                    "webhook operation failed (post operation failed)",
	ErrCodeWebhookOperationFailedPostOperationReturnedUnexpectedErrCode: "webhook operation failed (post operation returned unexpected code)",
	ErrCodeWebhookChannelRecoveryMessagesExpired:                        "unable to recover channel (messages expired)",
	ErrCodeWebhookChannelRecoveryMessageLimitExceeded:                   "unable to recover channel (message limit exceeded)",
	ErrCodeWebhookChannelRecoveryHistoryIncomplete:                      "unable to recover channel (history incomplete)",
	ErrCodeConnectionFailed:                                             "connection failed",
	ErrCodeConnectionFailedNoCompatibleTransport:                        "connection failed (no compatible transport)",
	ErrCodeConnectionSuspended:                                          "connection suspended",
	ErrCodeDisconnected:                                                 "disconnected",
	ErrCodeAlreadyConnected:                                             "already connected",
	ErrCodeInvalidConnectionIDRemoteNotFound:                            "invalid connection id (remote not found)",
	ErrCodeConnectionRecoveryMessagesExpired:                            "unable to recover connection (messages expired)",
	ErrCodeConnectionRecoveryMessageLimitExceeded:                       "unable to recover connection (message limit exceeded)",
	ErrCodeConnectionRecoveryConnectionExpired:                          "unable to recover connection (connection expired)",
	ErrCodeConnectionNotEstablishedNoTransportHandle:                    "connection not established (no transport handle)",
	ErrCodeInvalidOperationInvalidTransportHandle:                       "invalid operation (invalid transport handle)",
	ErrCodeConnectionRecoveryIncompatibleAuthParams:                     "unable to recover connection (incompatible auth params)",
	ErrCodeConnectionRecoveryInvalidOrUnspecifiedConnectionSerial:       "unable to recover connection (invalid or unspecified connection serial)",
	ErrCodeProtocol:                                         "protocol error",
	ErrCodeConnectionTimedOut:                               "connection timed out",
	ErrCodeIncompatibleConnectionParameters:                 "incompatible connection parameters",
	ErrCodeOperationOnSupersededTransport:                   "operation on superseded transport",
	ErrCodeChannelOperationFailed:                           "channel operation failed",
	ErrCodeChannelOperationFailedInvalidChannelState:        "channel operation failed (invalid channel state)",
	ErrCodeChannelOperationFailedEpochExpiredOrNeverExisted: "channel operation failed (epoch expired or never existed)",
	ErrCodeChannelRecoveryMessagesExpired:                   "unable to recover channel (messages expired)",
	ErrCodeChannelRecoveryMessageLimitExceeded:              "unable to recover channel (message limit exceeded)",
	ErrCodeChannelRecoveryNoMatchingEpoch:                   "unable to recover channel (no matching epoch)",
	ErrCodeChannelRecoveryUnboundedRequest:                  "unable to recover channel (unbounded request)",
	ErrCodeUnableToEnterPresenceChannelNoClientID:           "unable to enter presence channel (no clientId)",
	ErrCodeUnableToEnterPresenceChannelInvalidChannelState:  "unable to enter presence channel (invalid channel state)",
	ErrCodeUnableToLeavePresenceChannelThatIsNotEntered:     "unable to leave presence channel that is not entered",
}
