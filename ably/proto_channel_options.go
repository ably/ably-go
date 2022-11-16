package ably

type channelParams map[string]string

// ChannelMode Describes the possible flags used to configure client capabilities, using [ChannelOptions]{@link ChannelOptions}.
type ChannelMode int64

const (
	// **LEGACY**
	// Presence mode. Allows the attached channel to enter Presence.
	// **CANONICAL**
	// The client can enter the presence set.
	ChannelModePresence ChannelMode = iota + 1
	// **LEGACY**
	// Publish mode. Allows the messages to be published to the attached channel.
	// **CANONICAL**
	// The client can publish messages..
	ChannelModePublish
	// **LEGACY**
	// Subscribe mode. Allows the attached channel to subscribe to messages.
	// **CANONICAL**
	// The client can subscribe to messages.
	ChannelModeSubscribe
	// **LEGACY**
	// PresenceSubscribe. Allows the attached channel to subscribe to Presence updates.
	// **CANONICAL**
	// The client can receive presence messages.
	ChannelModePresenceSubscribe
)

func (mode ChannelMode) toFlag() protoFlag {
	switch mode {
	case ChannelModePresence:
		return flagPresence
	case ChannelModePublish:
		return flagPublish
	case ChannelModeSubscribe:
		return flagSubscribe
	case ChannelModePresenceSubscribe:
		return flagPresenceSubscribe
	default:
		return 0
	}
}

func channelModeFromFlag(flags protoFlag) []ChannelMode {
	var modes []ChannelMode
	if flags.Has(flagPresence) {
		modes = append(modes, ChannelModePresence)
	}
	if flags.Has(flagPublish) {
		modes = append(modes, ChannelModePublish)
	}
	if flags.Has(flagSubscribe) {
		modes = append(modes, ChannelModeSubscribe)
	}
	if flags.Has(flagPresenceSubscribe) {
		modes = append(modes, ChannelModePresenceSubscribe)
	}
	return modes
}

// **LEGACY**
// protoChannelOptions defines options provided for creating a new channel.
// **CANONICAL**
// Passes additional properties to a [RestChannel]{@link RestChannel} or [RealtimeChannel]{@link RealtimeChannel} object, such as encryption, [ChannelMode]{@link ChannelMode} and channel parameters.
type protoChannelOptions struct {
	Cipher CipherParams
	cipher channelCipher
	Params channelParams
	Modes  []ChannelMode
}
