package ably

// Error constants used in ably-go.
// These used to be generated from the error description text.
// But this means that the error name changes if we update the text, and that some names are too long

const (
	ErrNotSet                                    ErrorCode = 0
	ErrBadRequest                                ErrorCode = 40000
	ErrInvalidCredential                         ErrorCode = 40005
	ErrInvalidClientID                           ErrorCode = 40012
	ErrUnauthorized                              ErrorCode = 40100
	ErrInvalidCredentials                        ErrorCode = 40101
	ErrIncompatibleCredentials                   ErrorCode = 40102
	ErrInvalidUseOfBasicAuthOverNonTLSTransport  ErrorCode = 40103
	ErrTokenErrorUnspecified                     ErrorCode = 40140
	ErrErrorFromClientTokenCallback              ErrorCode = 40170
	ErrForbidden                                 ErrorCode = 40300
	ErrNotFound                                  ErrorCode = 40400
	ErrMethodNotAllowed                          ErrorCode = 40500
	ErrInternalError                             ErrorCode = 50000
	ErrInternalChannelError                      ErrorCode = 50001
	ErrInternalConnectionError                   ErrorCode = 50002
	ErrTimeoutError                              ErrorCode = 50003
	ErrConnectionFailed                          ErrorCode = 80000
	ErrConnectionSuspended                       ErrorCode = 80002
	ErrConnectionClosed                          ErrorCode = 80017
	ErrDisconnected                              ErrorCode = 80003
	ErrProtocolError                             ErrorCode = 80013
	ErrChannelOperationFailed                    ErrorCode = 90000
	ErrChannelOperationFailedInvalidChannelState ErrorCode = 90001
)
