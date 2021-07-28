package ably

type protoAction int8

const (
	actionHeartbeat protoAction = iota
	actionAck
	actionNack
	actionConnect
	actionConnected
	actionDisconnect
	actionDisconnected
	actionClose
	actionClosed
	actionError
	actionAttach
	actionAttached
	actionDetach
	actionDetached
	actionPresence
	actionMessage
	actionSync
	actionAuth
)

var actions = map[protoAction]string{
	actionHeartbeat:    "heartbeat",
	actionAck:          "ack",
	actionNack:         "nack",
	actionConnect:      "connect",
	actionConnected:    "connected",
	actionDisconnect:   "disconnect",
	actionDisconnected: "disconnected",
	actionClose:        "close",
	actionClosed:       "closed",
	actionError:        "error",
	actionAttach:       "attach",
	actionAttached:     "attached",
	actionDetach:       "detach",
	actionDetached:     "detached",
	actionPresence:     "presence",
	actionMessage:      "message",
	actionSync:         "sync",
	actionAuth:         "auth",
}

func (a protoAction) String() string {
	return actions[a]
}
