package ably

import (
	"fmt"
	"log"
	"os"
)

type LogLevel uint

const (
	LogNone LogLevel = iota
	LogError
	LogWarning
	LogInfo
	LogVerbose
	LogDebug
)

var logLevels = map[LogLevel]string{
	LogError:   "[ERROR] ",
	LogWarning: "[WARN] ",
	LogInfo:    "[INFO] ",
	LogVerbose: "[VERBOSE] ",
	LogDebug:   "[DEBUG] ",
}

var defaultLog = LoggerOptions{
	Logger: &StdLogger{Logger: log.New(os.Stderr, "", log.LstdFlags)},
	Level:  LogNone,
}

// LoggerOptions defines options for ably logging.
type LoggerOptions struct {
	Logger Logger
	Level  LogLevel
}

func (l LoggerOptions) Is(level LogLevel) bool {
	return l.Level != LogNone && l.Level >= level
}

func (l LoggerOptions) Print(level LogLevel, v ...interface{}) {
	if l.Is(level) {
		l.GetLogger().Print(level, v...)
	}
}

func (l LoggerOptions) Printf(level LogLevel, format string, v ...interface{}) {
	if l.Is(level) {
		l.GetLogger().Printf(level, format, v...)
	}
}

// GetLogger returns the custom logger if any. This will return the default
// logger if custom logger was not specified.
func (l LoggerOptions) GetLogger() Logger {
	if l.Logger != nil {
		return l.Logger
	}
	return defaultLog.Logger
}

// Logger is an interface for ably loggers.
type Logger interface {
	Print(level LogLevel, v ...interface{})
	Printf(level LogLevel, format string, v ...interface{})
}

// StdLogger wraps log.Logger to satisfy the Logger interface.
type StdLogger struct {
	*log.Logger
}

func (s *StdLogger) Printf(level LogLevel, format string, v ...interface{}) {
	s.Logger.Printf(logLevels[level]+format, v...)
}

func (s *StdLogger) Print(level LogLevel, v ...interface{}) {
	if len(v) != 0 {
		v[0] = fmt.Sprintf(logLevels[level]+"%v", v[0])
		s.Logger.Print(v...)
	}
}
