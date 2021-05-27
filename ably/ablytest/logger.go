package ablytest

import (
	"fmt"

	"github.com/ably/ably-go/ably"
)

type LogMessage struct {
	Level   ably.LogLevel
	Message string
}

func NewLogger(messages chan<- LogMessage) ably.Logger {
	return testLogger{messages: messages}
}

type testLogger struct {
	messages chan<- LogMessage
}

func (l testLogger) Print(level ably.LogLevel, v ...interface{}) {
	l.Printf(level, "%s", v...)
}

func (l testLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {
	l.messages <- LogMessage{
		Level:   level,
		Message: fmt.Sprintf(format, v...),
	}
}

var DiscardLogger ably.Logger = discardLogger{}

type discardLogger struct{}

func (discardLogger) Print(level ably.LogLevel, v ...interface{}) {}

func (discardLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {}
