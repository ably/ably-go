package ably

// CP2
type ChannelProperties struct {
	// AttachSerial contains the channelSerial from latest ATTACHED ProtocolMessage received on the channel, see CP2a, RTL15a
	AttachSerial string

	// ChannelSerial contains the channelSerial from latest ProtocolMessage of action type Message/PresenceMessage received on the channel, see CP2b, RTL15b.
	ChannelSerial string
}
