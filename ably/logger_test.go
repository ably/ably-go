//go:build !integration
// +build !integration

package ably_test

import (
	"testing"

	"github.com/ably/ably-go/ably"

	"github.com/stretchr/testify/assert"
)

type dummyLogger struct {
	printf int
}

func (d *dummyLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {
	d.printf++
}

func TestFilteredLogger(t *testing.T) {
	t.Run("must log smaller or same level", func(t *testing.T) {

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
		assert.Equal(t, 4, l.printf,
			"expected 4 log messages got %d", l.printf)
	})
	t.Run("must log nothing  for LogNone", func(t *testing.T) {

		l := &dummyLogger{}
		lg := &ably.FilteredLogger{
			Logger: l,
		}
		say := "log this"
		lg.Printf(ably.LogVerbose, say)
		lg.Printf(ably.LogInfo, say)
		lg.Printf(ably.LogWarning, say)
		lg.Printf(ably.LogError, say)
		assert.Equal(t, 0, l.printf,
			"expected nothing to be logged")
	})
}
