package proto

type Action int64

const (
	ActionHeartbeat Action = iota
	ActionAck
	ActionNack
	ActionConnect
	ActionConnected
	ActionDisconnect
	ActionDisconnected
	ActionClose
	ActionClosed
	ActionError
	ActionAttach
	ActionAttached
	ActionDetach
	ActionDetached
	ActionPresence
	ActionMessage
	ActionSync
)

var actions = map[Action]string{
	ActionHeartbeat:    "heartbeat",
	ActionAck:          "ack",
	ActionNack:         "nack",
	ActionConnect:      "connect",
	ActionConnected:    "connected",
	ActionDisconnect:   "disconnect",
	ActionDisconnected: "disconnected",
	ActionClose:        "close",
	ActionClosed:       "closed",
	ActionError:        "error",
	ActionAttach:       "attach",
	ActionAttached:     "attached",
	ActionDetach:       "detach",
	ActionDetached:     "detached",
	ActionPresence:     "presence",
	ActionMessage:      "message",
	ActionSync:         "sync",
}

func (a Action) String() string {
	return actions[a]
}
