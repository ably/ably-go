package ably

// RecoveryKeyContext contains the properties required to recover existing connection.
type recoveryKeyContext struct {
	ConnectionKey  string
	MsgSerial      int64
	ChannelSerials map[string]string
}

func (r *recoveryKeyContext) Encode() string {
	return ""
}
