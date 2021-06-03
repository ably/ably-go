package ably

type channelParams map[string]string
type ChannelMode int64

const (
	// Presence mode. Allows the attached channel to enter Presence.
	ChannelModePresence ChannelMode = iota + 1
	// Publish mode. Allows the messages to be published to the attached channel.
	ChannelModePublish
	// Subscribe mode. Allows the attached channel to subscribe to messages.
	ChannelModeSubscribe
	// PresenceSubscribe. Allows the attached channel to subscribe to Presence updates.
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

// protoChannelOptions defines options provided for creating a new channel.
type protoChannelOptions struct {
	Cipher CipherParams
	cipher channelCipher
	Params channelParams
	Modes  []ChannelMode
}
