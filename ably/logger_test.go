package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"
)

type dummyLogger struct {
	printf int
}

func (d *dummyLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {
	d.printf++
}

func TestFilteredLogger(t *testing.T) {
	t.Run("must log smaller or same level", func(t *testing.T) {
		t.Parallel()

		l := &dummyLogger{}
		lg := &ably.FilteredLogger{
			Level:  ably.LogDebug,
			Logger: l,
		}
		say := "log this"
		lg.Printf(ably.LogVerbose, say)
		lg.Printf(ably.LogInfo, say)
		lg.Printf(ably.LogWarning, say)
		lg.Printf(ably.LogError, say)
		if l.printf != 4 {
			t.Errorf("expected 4 log messages got %d", l.printf)
		}
	})
	t.Run("must log nothing  for LogNone", func(t *testing.T) {
		t.Parallel()

		l := &dummyLogger{}
		lg := &ably.FilteredLogger{
			Logger: l,
		}
		say := "log this"
		lg.Printf(ably.LogVerbose, say)
		lg.Printf(ably.LogInfo, say)
		lg.Printf(ably.LogWarning, say)
		lg.Printf(ably.LogError, say)
		if l.printf > 0 {
			t.Error("expected nothing to be logged")
		}
	})
}
