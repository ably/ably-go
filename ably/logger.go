package ably

import (
	"fmt"
	"log"
	"os"
)

const (
	LogNone = iota
	LogError
	LogWarning
	LogInfo
	LogVerbose
	LogDebug
)

var logLevels = map[int]string{
	LogError:   "[ERROR] ",
	LogWarning: "[WARN] ",
	LogInfo:    "[INFO] ",
	LogVerbose: "[VERBOSE] ",
	LogDebug:   "[DEBUG] ",
}

var defaultLog = LoggerOptions{
	Logger: log.New(os.Stderr, "", log.LstdFlags),
	Level:  LogNone,
}

type LoggerOptions struct {
	Logger *log.Logger
	Level  int
}

func (l LoggerOptions) Is(level int) bool {
	return l.Level >= level
}

func (l LoggerOptions) Print(level int, v ...interface{}) {
	if l.Is(level) {
		if len(v) != 0 {
			v[0] = fmt.Sprintf(logLevels[level]+"%v", v[0])
		}
		l.logger().Print(v...)
	}
}

func (l LoggerOptions) Printf(level int, format string, v ...interface{}) {
	if l.Is(level) {
		l.logger().Printf(logLevels[level]+format, v...)
	}
}

func (l LoggerOptions) logger() *log.Logger {
	if l.Logger != nil {
		return l.Logger
	}
	return defaultLog.Logger
}
