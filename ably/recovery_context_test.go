//go:build !unit
// +build !unit

package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
	"github.com/stretchr/testify/assert"
)

func Test_ShouldEncodeRecoveryKeyContextObject(t *testing.T) {
	var expectedRecoveryKey = "{\"connectionKey\":\"uniqueKey\",\"msgSerial\":1,\"channelSerials\":{\"channel1\":\"1\",\"channel2\":\"2\",\"channel3\":\"3\"}}"
	var recoveryKey = &ably.RecoveryKeyContext{
		ConnectionKey: "uniqueKey",
		MsgSerial:     1,
		ChannelSerials: map[string]string{
			"channel1": "1",
			"channel2": "2",
			"channel3": "3",
		},
	}
	key, err := recoveryKey.Encode()
	assert.Nil(t, err)
	assert.Equal(t, expectedRecoveryKey, key)
}

func Test_ShouldDecodeRecoveryKeyToRecoveryKeyContextObject(t *testing.T) {
	var recoveryKey = "{\"connectionKey\":\"uniqueKey\",\"msgSerial\":1,\"channelSerials\":{\"channel1\":\"1\",\"channel2\":\"2\",\"channel3\":\"3\"}}"
	keyContext, err := ably.DecodeRecoveryKey(recoveryKey)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), keyContext.MsgSerial)
	assert.Equal(t, "uniqueKey", keyContext.ConnectionKey)
	assert.Equal(t, "1", keyContext.ChannelSerials["channel1"])
	assert.Equal(t, "2", keyContext.ChannelSerials["channel2"])
	assert.Equal(t, "3", keyContext.ChannelSerials["channel3"])
}

func Test_ShouldReturnNullRecoveryContextWhileDecodingFaultyRecoveryKey(t *testing.T) {
	var recoveryKey = "{\"connectionKey\":\"uniqueKey\",\"msgSerial\":\"incorrectStringSerial\",\"channelSerials\":{\"channel1\":\"1\",\"channel2\":\"2\",\"channel3\":\"3\"}}"
	keyContext, err := ably.DecodeRecoveryKey(recoveryKey)
	assert.NotNil(t, err)
	assert.Nil(t, keyContext)
}
