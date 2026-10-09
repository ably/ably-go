//go:build !integration
// +build !integration

package ably

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelOptionChannelWithCipherKey(t *testing.T) {
	tests := map[string]struct {
		key            []byte
		expectedResult *channelOptions
	}{
		"Can inject a cipher key of length 128 into cipher params": {
			key: []byte{82, 27, 7, 33, 130, 101, 79, 22, 63, 95, 15, 154, 98, 29, 114, 19},
			expectedResult: &channelOptions{
				Cipher: CipherParams{
					Algorithm: CipherAES,
					KeyLength: 128,
					Key:       []uint8{0x52, 0x1b, 0x7, 0x21, 0x82, 0x65, 0x4f, 0x16, 0x3f, 0x5f, 0xf, 0x9a, 0x62, 0x1d, 0x72, 0x13},
					Mode:      CipherCBC,
				},
			},
		},

		"Can inject a cipher key of length 256 into cipher params": {
			key: []byte{82, 27, 7, 33, 130, 101, 79, 22, 63, 95, 15, 154, 98, 29, 114, 19, 10, 23, 45, 56, 76, 29, 111, 23, 93, 22, 44, 66, 88, 43, 72, 42},
			expectedResult: &channelOptions{
				Cipher: CipherParams{
					Algorithm: CipherAES,
					KeyLength: 256,
					Key:       []uint8{0x52, 0x1b, 0x7, 0x21, 0x82, 0x65, 0x4f, 0x16, 0x3f, 0x5f, 0xf, 0x9a, 0x62, 0x1d, 0x72, 0x13, 0xa, 0x17, 0x2d, 0x38, 0x4c, 0x1d, 0x6f, 0x17, 0x5d, 0x16, 0x2c, 0x42, 0x58, 0x2b, 0x48, 0x2a},
					Mode:      CipherCBC,
				},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			opt := ChannelWithCipherKey(test.key)
			result := applyChannelOptions(opt)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestChannelOptionChannelWithCipher(t *testing.T) {
	tests := map[string]struct {
		params         CipherParams
		expectedResult *channelOptions
	}{
		"Can set cipher params as channel options": {
			params: CipherParams{
				Algorithm: CipherAES,
				KeyLength: 128,
				Key:       []uint8{0x52, 0x1b, 0x7, 0x21, 0x82, 0x65, 0x4f, 0x16, 0x3f, 0x5f, 0xf, 0x9a, 0x62, 0x1d, 0x72, 0x13},
				Mode:      CipherCBC,
			},
			expectedResult: &channelOptions{
				Cipher: CipherParams{
					Algorithm: CipherAES,
					KeyLength: 128,
					Key:       []uint8{0x52, 0x1b, 0x7, 0x21, 0x82, 0x65, 0x4f, 0x16, 0x3f, 0x5f, 0xf, 0x9a, 0x62, 0x1d, 0x72, 0x13},
					Mode:      CipherCBC,
				},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			opt := ChannelWithCipher(test.params)
			result := applyChannelOptions(opt)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestChannelOptionChannelWithParams(t *testing.T) {
	tests := map[string]struct {
		key            string
		value          string
		expectedResult *channelOptions
	}{
		"Can set a key and a value as channel options": {
			key:   "aKey",
			value: "aValue",
			expectedResult: &channelOptions{
				Params: map[string]string{"aKey": "aValue"},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			opt := ChannelWithParams(test.key, test.value)
			result := applyChannelOptions(opt)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestChannelOptionChannelWithModes(t *testing.T) {
	tests := map[string]struct {
		modes          []ChannelMode
		expectedResult *channelOptions
	}{
		"Can set a channel mode as channel options": {
			modes: []ChannelMode{ChannelModePresence},
			expectedResult: &channelOptions{
				Modes: []ChannelMode{ChannelModePresence},
			},
		},
		"Can set multiple channel mode as channel options": {
			modes: []ChannelMode{ChannelModePresence, ChannelModePublish},
			expectedResult: &channelOptions{
				Modes: []ChannelMode{ChannelModePresence, ChannelModePublish},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			opt := ChannelWithModes(test.modes...)
			result := applyChannelOptions(opt)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestChannelGet(t *testing.T) {
	tests := map[string]struct {
		mock                 *RealtimeChannels
		name                 string
		expectedChannelName  string
		expectedChannelState ChannelState
	}{
		"If channel does not exist, it is created and initialised": {
			mock: &RealtimeChannels{
				chans: map[string]*RealtimeChannel{},
				client: &Realtime{
					rest: &REST{
						log:  logger{l: &stdLogger{mocklogger}},
						opts: NewClientOptions(),
					},
				},
			},
			name:                 "new",
			expectedChannelName:  "new",
			expectedChannelState: ChannelStateInitialized,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			result := test.mock.Get(test.name)
			assert.Equal(t, test.expectedChannelName, result.Name)
			assert.Equal(t, test.expectedChannelState, result.state)
		})
	}
}

func TestChannelsRelease(t *testing.T) {
	newClient := func(t *testing.T) *Realtime {
		client, err := NewRealtime(WithKey("abc:def"), WithAutoConnect(false))
		require.NoError(t, err)
		return client
	}

	t.Run("RTS4c: releasing a non-existent channel is a no-op", func(t *testing.T) {
		client := newClient(t)
		assert.NoError(t, client.Channels.Release("nonexistent"))
	})

	for _, state := range []ChannelState{ChannelStateInitialized, ChannelStateDetached, ChannelStateFailed} {
		t.Run(fmt.Sprintf("RTS4d: releasing a %s channel removes it", state), func(t *testing.T) {
			client := newClient(t)
			channel := client.Channels.Get("test")
			channel.state = state
			assert.NoError(t, client.Channels.Release("test"))
			assert.False(t, client.Channels.Exists("test"))
			assert.Equal(t, state, channel.State())
			assert.NotSame(t, channel, client.Channels.Get("test"))
		})
	}

	for _, state := range []ChannelState{ChannelStateAttaching, ChannelStateAttached, ChannelStateDetaching, ChannelStateSuspended} {
		t.Run(fmt.Sprintf("RTS4e: releasing a %s channel fails and leaves it in place", state), func(t *testing.T) {
			client := newClient(t)
			channel := client.Channels.Get("test")
			channel.state = state
			err := client.Channels.Release("test")
			var errInfo *ErrorInfo
			require.ErrorAs(t, err, &errInfo)
			assert.Equal(t, ErrChannelReleaseInvalidState, errInfo.Code)
			assert.Equal(t, 400, errInfo.StatusCode)
			assert.Contains(t, errInfo.Message(), "the current state is "+state.String())
			assert.Equal(t, state, channel.State())
			assert.Same(t, channel, client.Channels.Get("test"))
		})
	}
}
