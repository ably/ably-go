package ably

import (
	"fmt"
	"strconv"
	"strings"
)

// PresenceAction describes the possible actions members in the presence set can emit (TP2).
type PresenceAction int64

const (
	// PresenceActionAbsent specifies a member is not present in the channel.
	PresenceActionAbsent PresenceAction = iota
	// PresenceActionPresent denotes when subscribing to presence events on a channel that already has members present,
	// this event is emitted for every member already present on the channel before the subscribe listener was registered.
	PresenceActionPresent
	// PresenceActionEnter denotes that new member has entered the channel.
	PresenceActionEnter
	// PresenceActionLeave is a member who was present has now left the channel. This may be a result of an explicit
	// request to leave or implicitly when detaching from the channel. Alternatively, if a member's connection is
	// abruptly disconnected and they do not resume their connection within a minute, Ably treats this as a
	// leave event as the client is no longer present.
	PresenceActionLeave
	// PresenceActionUpdate is a already present member has updated their member data. Being notified of member data
	// updates can be very useful, for example, it can be used to update the status of a user when they are
	// typing a message.
	PresenceActionUpdate
)

func (e PresenceAction) String() string {
	switch e {
	case PresenceActionAbsent:
		return "ABSENT"
	case PresenceActionPresent:
		return "PRESENT"
	case PresenceActionEnter:
		return "ENTER"
	case PresenceActionLeave:
		return "LEAVE"
	case PresenceActionUpdate:
		return "UPDATE"
	}
	return ""
}

func (PresenceAction) isEmitterEvent() {}

type PresenceMessage struct {
	Message
	Action PresenceAction `json:"action" codec:"action"`
}

func (m PresenceMessage) String() string {
	return fmt.Sprintf("<PresenceMessage %v id=%v connectionId=%v clientID=%v data=%v>", [...]string{
		"absent",
		"present",
		"enter",
		"leave",
		"update",
	}[m.Action], m.ID, m.ConnectionID, m.ClientID, m.Data)
}

func (msg *PresenceMessage) isServerSynthesized() bool {
	return !strings.HasPrefix(msg.ID, msg.ConnectionID)
}

func (msg *PresenceMessage) getMsgSerialAndIndex() (int64, int64, error) {
	msgIds := strings.Split(msg.ID, ":")
	if len(msgIds) != 3 {
		return 0, 0, fmt.Errorf("parsing error, the presence message has invalid id %v", msg.ID)
	}
	msgSerial, err := strconv.ParseInt(msgIds[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing error, the presence message has invalid msgSerial, for msgId %v", msg.ID)
	}
	msgIndex, err := strconv.ParseInt(msgIds[2], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing error, the presence message has invalid msgIndex, for msgId %v", msg.ID)
	}
	return msgSerial, msgIndex, nil
}

// RTP2b, RTP2c
func (msg1 *PresenceMessage) IsNewerThan(msg2 *PresenceMessage) (bool, error) {
	// RTP2b1
	if msg1.isServerSynthesized() || msg2.isServerSynthesized() {
		return msg1.Timestamp >= msg2.Timestamp, nil
	}

	// RTP2b2
	msg1Serial, msg1Index, err := msg1.getMsgSerialAndIndex()
	if err != nil {
		return false, err
	}
	msg2Serial, msg2Index, err := msg2.getMsgSerialAndIndex()
	if err != nil {
		return true, err
	}
	if msg1Serial == msg2Serial {
		return msg1Index > msg2Index, nil
	}
	return msg1Serial > msg2Serial, nil
}
