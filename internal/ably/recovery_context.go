package ably

import "encoding/json"

// RecoveryKeyContext contains the properties required to recover existing connection.
type RecoveryKeyContext struct {
	ConnectionKey  string            `json:"connectionKey" codec:"connectionKey"`
	MsgSerial      int64             `json:"msgSerial" codec:"msgSerial"`
	ChannelSerials map[string]string `json:"channelSerials" codec:"channelSerials"`
}

func (r *RecoveryKeyContext) Encode() (serializedRecoveryKey string, err error) {
	result, err := json.Marshal(r)
	if err != nil {
		serializedRecoveryKey = ""
	} else {
		serializedRecoveryKey = string(result)
	}
	return
}

func DecodeRecoveryKey(recoveryKey string) (rCtx *RecoveryKeyContext, err error) {
	err = json.Unmarshal([]byte(recoveryKey), &rCtx)
	if err != nil {
		rCtx = nil
	}
	return
}
