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
	t.Run("must use default logger when no custom logger", func(ts *testing.T) {
		lg := &ably.LoggerOptions{}
		l := lg.GetLogger()
		if l == nil {
			ts.Fatal("expected to get a valid logger")
		}
		_, ok := l.(*ably.StdLogger)
		if !ok {
			ts.Error("expected default logger")
		}
	})

	t.Run("must use custom logger", func(ts *testing.T) {
		lg := &ably.LoggerOptions{
			Logger: &dummyLogger{},
		}
		l := lg.GetLogger()
		if l == nil {
			ts.Fatal("expected to get a valid logger")
		}
		_, ok := l.(*dummyLogger)
		if !ok {
			ts.Error("expected custom logger")
		}
	})

	t.Run("must log smaller or same level", func(ts *testing.T) {
		lg := &ably.LoggerOptions{
			Level:  ably.LogDebug,
			Logger: &dummyLogger{},
		}
		say := "log this"
		lg.Print(ably.LogVerbose, say)
		lg.Print(ably.LogInfo, say)
		lg.Print(ably.LogWarning, say)
		lg.Print(ably.LogError, say)
		l := lg.GetLogger().(*dummyLogger)
		if l.print != 4 {
			t.Errorf("expected 4 log messages got %d", l.print)
		}
	})
	t.Run("must log nothing  for LogNone", func(ts *testing.T) {
		lg := &ably.LoggerOptions{
			Logger: &dummyLogger{},
		}
		say := "log this"
		lg.Print(ably.LogVerbose, say)
		lg.Print(ably.LogInfo, say)
		lg.Print(ably.LogWarning, say)
		lg.Print(ably.LogError, say)
		l := lg.GetLogger().(*dummyLogger)
		if l.print > 0 {
			ts.Error("expected nothing to be logged")
		}
	})
}
