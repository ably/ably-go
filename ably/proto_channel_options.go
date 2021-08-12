package ably

type channelParams map[string]string
type ChannelMode int64

type channelModeSet map[ChannelMode]struct{}

func newChannelModeSet() channelModeSet {
	return make(channelModeSet)
}
func (s channelModeSet) add(mode ...ChannelMode) {
	for _, channelMode := range mode {
		s[channelMode] = struct{}{}
	}
}
func (s channelModeSet) Has(mode ChannelMode) bool {
	_, ok := s[mode]
	return ok
}

const (
	// ChannelModePresence - Allows attached channel to enter Presence.
	ChannelModePresence ChannelMode = iota + 1
	// ChannelModePublish - Allows messages to be published on the attached channel.
	ChannelModePublish
	// ChannelModeSubscribe - Allows attached channel to subscribe to messages.
	ChannelModeSubscribe
	// ChannelModePresenceSubscribe - Allows attached channel to subscribe to Presence updates.
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
	Modes  channelModeSet
}
