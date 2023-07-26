package ably

import "encoding/json"

// recoveryKeyContext contains the properties required to recover existing connection.
type recoveryKeyContext struct {
	ConnectionKey  string            `json:"connectionKey" codec:"connectionKey"`
	MsgSerial      int64             `json:"msgSerial" codec:"msgSerial"`
	ChannelSerials map[string]string `json:"channelSerials" codec:"channelSerials"`
}

func (r *recoveryKeyContext) Encode() (serializedRecoveryKey string, err error) {
	result, err := json.Marshal(r)
	serializedRecoveryKey = string(result)
	return
}

func Decode(recoveryKey string) (rCtx *recoveryKeyContext, err error) {
	err = json.Unmarshal([]byte(recoveryKey), &rCtx)
	return
}
