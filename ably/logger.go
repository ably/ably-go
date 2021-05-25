package ably

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync/atomic"
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
	LogError:   "ERRO",
	LogWarning: "WARN",
	LogInfo:    "INFO",
	LogVerbose: "VBOS",
	LogDebug:   "DBUG",
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

// logger is the internal logger type, with helper methods that wrap the raw
// Logger interface.
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

// Begin starts a log scope. A log scope has a beginning and an end, and
// contains a number of log messages; at least, a begin and end message. Each
// scope has a unique ID that is appended to each log message in the scope.
//
// Log scopes can be nested, in which case each scope ID will be appended to
// each log message in it.
//
// A log message will be logged with the scope name, and a (maybe empty) list
// of arguments as key-value pairs.
//
// Log scope IDs are kept in contexts. The returned context will carry the
// new scope ID, plus any scope IDs from the provided context.
//
// Use logger.scoped to get a logger that will include its log messages in the
// scopes associated with the context. For convenience, it's also returned.
//
// When the scope is finished, the end function should be called. It takes a
// pointer to an error, which, if non-nil, will be logged along with any other
// provided result values as key-value pairs. The result values should be
// pointers, so the end function can be called with defer. The log level if the
// error is non-nil will be ERROR if level is INFO, WARNING otherwise.
func (l logger) Begin(ctx context.Context, level LogLevel, name string, args ...interface{}) (
	scopedCtx context.Context,
	scopedLogger logger,
	end func(err *error, results ...interface{}),
) {
	parent, _ := ctx.Value(logScopeCtxKey{}).(*logScope)
	ctx = context.WithValue(ctx, logScopeCtxKey{}, &logScope{
		parent: parent,
		id:     nextLogScopeID(),
		name:   name,
	})

	l = l.scoped(ctx)
	l.l.Printf(level, "%s:begin%s", name, logKeyValues(false, args...))

	return ctx, l, func(err *error, results ...interface{}) {
		endLevel := level
		if err != nil && *err != nil {
			if level == LogInfo {
				endLevel = LogError
			} else {
				endLevel = LogWarning
			}
			results = append([]interface{}{"err", err}, results...)
		}
		l.l.Printf(endLevel, "%s:end%s", name, logKeyValues(true, results...))
	}
}

func logKeyValues(indirect bool, kvs ...interface{}) string {
	if len(kvs) == 0 {
		return ""
	}
	var s strings.Builder
	s.WriteString(" (")
	for i := 0; i < len(kvs); i += 2 {
		k, v := kvs[i], kvs[i+1]
		if i > 0 {
			s.WriteString(" ")
		}
		if indirect {
			v = reflect.Indirect(reflect.ValueOf(v)).Interface()
		}
		fmt.Fprintf(&s, "%s=%q", k, fmt.Sprint(v))
	}
	s.WriteString(")")
	return s.String()
}

func (l logger) scoped(ctx context.Context) logger {
	scope, _ := ctx.Value(logScopeCtxKey{}).(*logScope)
	if scope == nil {
		return l
	}
	return logger{
		l: scopedLogger{l: l.l, scope: scope.String()},
	}
}

type logScopeCtxKey struct{}

type logScope struct {
	id     logScopeID
	name   string
	parent *logScope
}

func (scope *logScope) String() string {
	var s strings.Builder
	for scope != nil {
		if s.Len() > 0 {
			s.WriteString("←")
		}
		fmt.Fprintf(&s, "%s:%s", scope.name, scope.id)
		scope = scope.parent
	}
	return s.String()
}

var nextLogScopeID = func() func() logScopeID {
	var counter uint32
	return func() logScopeID {
		return logScopeID(atomic.AddUint32(&counter, 1))
	}
}()

type logScopeID uint32

func (id logScopeID) String() string {
	// Take only the last 2 bytes (4 hex characters); 8 characters is too noise for
	// logs. This gives ~65k unique IDs, which should be good enough for
	// correlating logs. We "waste" the other 2 bytes of the uint32 but that's the
	// smallest uint for which there's an atomic add.
	return fmt.Sprintf("%04x", (1<<16-1)&uint32(id))
}

type scopedLogger struct {
	l     Logger
	scope string
}

func (l scopedLogger) Printf(level LogLevel, fmt string, v ...interface{}) {
	l.l.Printf(level, fmt+" @ "+l.scope, v...)
}
