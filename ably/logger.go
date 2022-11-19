package ably

import (
	"fmt"
	"log"
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

var logLevelNames = map[LogLevel]string{
	LogError:   "ERROR",
	LogWarning: "WARN",
	LogInfo:    "INFO",
	LogVerbose: "VERBOSE",
	LogDebug:   "DEBUG",
}

func (l LogLevel) String() string {
	return logLevelNames[l]
}

type filteredLogger struct {
	Logger Logger
	Level  LogLevel
}

func (l filteredLogger) Is(level LogLevel) bool {
	return l.Level != LogNone && l.Level >= level
}

func (l filteredLogger) Printf(level LogLevel, format string, v ...interface{}) {
	if l.Is(level) {
		l.Logger.Printf(level, format, v...)
	}
}

// Logger is an interface for ably loggers.
type Logger interface {
	Printf(level LogLevel, format string, v ...interface{})
}

// stdLogger wraps log.Logger to satisfy the Logger interface.
type stdLogger struct {
	*log.Logger
}

func (s *stdLogger) Printf(level LogLevel, format string, v ...interface{}) {
	s.Logger.Printf(fmt.Sprintf("[%s] %s", level, format), v...)
}

// logger is the internal logger type, with helper methods that wrap the raw Logger interface.
type logger struct {
	l Logger
}

func (l logger) Error(v ...interface{}) {
	l.print(LogError, v...)
}

func (l logger) Errorf(fmt string, v ...interface{}) {
	l.l.Printf(LogError, fmt, v...)
}

func (l logger) Warn(v ...interface{}) {
	l.print(LogWarning, v...)
}

func (l logger) Warnf(fmt string, v ...interface{}) {
	l.l.Printf(LogWarning, fmt, v...)
}

func (l logger) Info(v ...interface{}) {
	l.print(LogInfo, v...)
}

func (l logger) Infof(fmt string, v ...interface{}) {
	l.l.Printf(LogInfo, fmt, v...)
}

func (l logger) Verbose(v ...interface{}) {
	l.print(LogVerbose, v...)
}

func (l logger) Verbosef(fmt string, v ...interface{}) {
	l.l.Printf(LogVerbose, fmt, v...)
}

func (l logger) Debugf(fmt string, v ...interface{}) {
	l.l.Printf(LogDebug, fmt, v...)
}

func (l logger) Debug(v ...interface{}) {
	l.print(LogDebug, v...)
}

func (l logger) print(level LogLevel, v ...interface{}) {
	l.l.Printf(level, fmt.Sprint(v...))
}
