package ably

import "encoding/json"

// recoveryKeyContext contains the properties required to recover existing connection.
type recoveryKeyContext struct {
	ConnectionKey  string            `json:"connectionKey,omitempty" codec:"connectionKey,omitempty"`
	MsgSerial      int64             `json:"msgSerial,omitempty" codec:"msgSerial,omitempty"`
	ChannelSerials map[string]string `json:"channelSerials,omitempty" codec:"channelSerials,omitempty"`
}

func (r *recoveryKeyContext) Encode() (string, error) {
	serializedRecoveryKey, err := json.Marshal(r)
	return string(serializedRecoveryKey), err
}
