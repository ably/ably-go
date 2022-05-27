//go:build !integration
// +build !integration

package ably

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			var opt ChannelOption
			if len(test.modes) == 1 {
				opt = ChannelWithModes(test.modes[0])
			} else {
				opt = ChannelWithModes(test.modes[0], test.modes[1])
			}
			result := applyChannelOptions(opt)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestChannelGet(t *testing.T) {
	tests := map[string]struct {
		name                 string
		expectedChannelName  string
		expectedChannelState ChannelState
	}{
		"Can get an existing channel": {
			name:                 "existing",
			expectedChannelName:  "existing",
			expectedChannelState: ChannelStateAttached,
		},
		"If channel does not exist, it is created and initialised": {
			name:                 "new",
			expectedChannelName:  "new",
			expectedChannelState: ChannelStateInitialized,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {

			//These mocks must be created here because type RealtimeChannel
			//contains a sync.Mutex which is a value that cannot be copied.
			//TODO: investigate representing sync.Mutex with sync.Locker interface.

			mockedExistingChannel := RealtimeChannels{
				chans: map[string]*RealtimeChannel{
					"existing": {
						Name:  "existing",
						state: ChannelState{name: "ATTACHED"},
					},
				},
				client: &Realtime{
					rest: &REST{
						log: logger{l: &stdLogger{mocklogger}},
					},
				},
			}

			mockedNoChannels := RealtimeChannels{
				chans: map[string]*RealtimeChannel{},
				client: &Realtime{
					rest: &REST{
						log: logger{l: &stdLogger{mocklogger}},
					},
				},
			}

			var result *RealtimeChannel
			if test.name == "existing"{
				result = mockedExistingChannel.Get(test.name)
			} else {
				result = mockedNoChannels.Get(test.name)
			}

			assert.Equal(t, test.expectedChannelName, result.Name)
			assert.Equal(t, test.expectedChannelState, result.state)
		})
	}
}
