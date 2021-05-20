package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
)

type dummyLogger struct {
	print  int
	printf int
}

func (d *dummyLogger) Print(level ably.LogLevel, v ...interface{}) {
	d.print++
}

func (d *dummyLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {
	d.printf++
}

func TestLoggerOptions(t *testing.T) {
	t.Run("must log smaller or same level", func(ts *testing.T) {
		l := &dummyLogger{}
		lg := &ably.LoggerOptions{
			Level:  ably.LogDebug,
			Logger: l,
		}
		say := "log this"
		lg.Print(ably.LogVerbose, say)
		lg.Print(ably.LogInfo, say)
		lg.Print(ably.LogWarning, say)
		lg.Print(ably.LogError, say)
		if l.print != 4 {
			t.Errorf("expected 4 log messages got %d", l.print)
		}
	})
	t.Run("must log nothing  for LogNone", func(ts *testing.T) {
		l := &dummyLogger{}
		lg := &ably.LoggerOptions{
			Logger: l,
		}
		say := "log this"
		lg.Print(ably.LogVerbose, say)
		lg.Print(ably.LogInfo, say)
		lg.Print(ably.LogWarning, say)
		lg.Print(ably.LogError, say)
		if l.print > 0 {
			ts.Error("expected nothing to be logged")
		}
	})
}
