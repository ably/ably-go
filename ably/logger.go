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

var defaultLog = Logger{
	Logger: log.New(os.Stderr, "", log.Lmicroseconds|log.Lshortfile),
	Level:  LogNone,
}

type Logger struct {
	Logger *log.Logger
	Level  int
}

func (l Logger) Print(level int, v ...interface{}) {
	if l.Level >= level {
		if len(v) != 0 {
			v[0] = fmt.Sprintf(logLevels[level]+"%v", v[0])
		}
		l.log().Print(v...)
	}
}

func (l Logger) Printf(level int, format string, v ...interface{}) {
	if l.Level >= level {
		l.log().Printf(logLevels[level]+format, v...)
	}
}

func (l Logger) log() *log.Logger {
	if l.Logger != nil {
		return l.Logger
	}
	return defaultLog.Logger
}
