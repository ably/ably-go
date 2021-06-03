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

func (mode ChannelMode) ToFlag() Flag {
	switch mode {
	case ChannelModePresence:
		return FlagPresence
	case ChannelModePublish:
		return FlagPublish
	case ChannelModeSubscribe:
		return FlagSubscribe
	case ChannelModePresenceSubscribe:
		return FlagPresenceSubscribe
	default:
		return 0
	}
}

func FromFlag(flags Flag) []ChannelMode {
	var modes []ChannelMode
	if flags.Has(FlagPresence) {
		modes = append(modes, ChannelModePresence)
	}
	if flags.Has(FlagPublish) {
		modes = append(modes, ChannelModePublish)
	}
	if flags.Has(FlagSubscribe) {
		modes = append(modes, ChannelModeSubscribe)
	}
	if flags.Has(FlagPresenceSubscribe) {
		modes = append(modes, ChannelModePresenceSubscribe)
	}
	return modes
}

// ChannelOptions defines options provided for creating a new channel.
type ChannelOptions struct {
	Cipher CipherParams
	cipher ChannelCipher
	Params channelParams
	Modes  []ChannelMode
}
