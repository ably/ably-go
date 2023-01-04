package ably

type channelParams map[string]string

// ChannelMode Describes the possible flags used to configure client capabilities, using [ably.ChannelOption].
type ChannelMode int64

const (
	// ChannelModePresence allows the attached channel to enter Presence.
	ChannelModePresence ChannelMode = iota + 1
	// ChannelModePublish allows for messages to be published on the attached channel.
	ChannelModePublish
	// ChannelModeSubscribe allows the attached channel to subscribe to messages.
	ChannelModeSubscribe
	// ChannelModePresenceSubscribe allows the attached channel to subscribe to Presence updates.
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

// protoChannelOptions defines additional properties to a [ably.RESTChannel] or [ably.RealtimeChannel] object,
// such as encryption, [ably.ChannelMode] and channel parameters.
// It defines options provided for creating a new channel.
type protoChannelOptions struct {
	Cipher CipherParams
	cipher channelCipher
	Params channelParams
	Modes  []ChannelMode
}
