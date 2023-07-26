package ably

// RecoveryKeyContext contains the properties required to recover existing connection.
var RecoveryContext struct {
	ConnectionKey  string
	MsgSerial      int64
	ChannelSerials map[string]string
}
