package ably

import (
	"log"
	"os"
)

const (
	LogError = 1 + iota
	LogWarn
	LogInfo
	LogVerbose
)

var logLevels = map[int]string{
	LogError:   "ERROR: ",
	LogWarn:    "WARN: ",
	LogInfo:    "INFO: ",
	LogVerbose: "VERBOSE: ",
}

var Log = Logger{
	Logger: log.New(os.Stdout, "", log.Lmicroseconds|log.Lshortfile),
	Level:  LogError,
}

type Logger struct {
	Logger *log.Logger
	Level  int
}

func (l Logger) Print(level int, v ...interface{}) {
	if l.Level >= level {
		if prefix, ok := logLevels[level]; ok {
			v = append(v, interface{}(nil))
			copy(v[1:], v)
			v[0] = prefix
		}
		l.Logger.Print(v...)
	}
}

func (l Logger) Printf(level int, format string, v ...interface{}) {
	if l.Level >= level {
		l.Logger.Printf(logLevels[level]+format, v...)
	}
}
