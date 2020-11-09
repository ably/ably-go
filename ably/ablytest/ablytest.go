package ablytest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
)

var Timeout = 30 * time.Second
var NoBinaryProtocol bool
var DefaultLogger = ably.LoggerOptions{Level: ably.LogNone}
var Environment = "sandbox"

func nonil(err ...error) error {
	for _, err := range err {
		if err != nil {
			return err
		}
	}
	return nil
}

func init() {
	if s := os.Getenv("ABLY_TIMEOUT"); s != "" {
		if t, err := time.ParseDuration(s); err == nil {
			Timeout = t
		}
	}
	if os.Getenv("ABLY_PROTOCOL") == "application/json" {
		NoBinaryProtocol = true
	}
	if n, err := strconv.Atoi(os.Getenv("ABLY_LOGLEVEL")); err == nil {
		DefaultLogger.Level = ably.LogLevel(n)
	}
	if s := os.Getenv("ABLY_ENV"); s != "" {
		Environment = s
	}
}

func MergeOptions(opts ...[]ably.ClientOption) []ably.ClientOption {
	var merged []ably.ClientOption
	for _, opt := range opts {
		merged = append(merged, opt...)
	}
	return merged
}

func encode(typ string, in interface{}) ([]byte, error) {
	switch typ {
	case "application/json":
		return json.Marshal(in)
	case "application/x-msgpack":
		return ablyutil.MarshalMsgpack(in)
	case "text/plain":
		return []byte(fmt.Sprintf("%v", in)), nil
	default:
		return nil, fmt.Errorf("encoding error: unrecognized Content-Type: %q", typ)
	}
}

var ClientOptionsInspector struct {
	UseBinaryProtocol func([]ably.ClientOption) bool
	HTTPClient        func([]ably.ClientOption) *http.Client
}

func protocol(opts []ably.ClientOption) string {
	if ClientOptionsInspector.UseBinaryProtocol(opts) {
		return "application/x-msgpack"
	}
	return "application/json"
}

func Contains(s, sub interface{}) bool {
	return reflectContains(reflect.ValueOf(s), reflect.ValueOf(sub))
}

func reflectContains(s, sub reflect.Value) bool {
	for i := 0; i+sub.Len() <= s.Len(); i++ {
		if reflect.DeepEqual(
			s.Slice(i, i+sub.Len()).Interface(),
			sub.Interface(),
		) {
			return true
		}
	}
	return false
}

type MessageChannel chan *ably.Message

func (ch MessageChannel) Receive(m *ably.Message) {
	ch <- m
}

func ReceiveMessages(channel *ably.RealtimeChannel, name string) (messages <-chan *ably.Message, unsubscribe func(), err error) {
	ch := make(MessageChannel, 100)
	if name == "" {
		unsubscribe, err = channel.SubscribeAll(context.Background(), ch.Receive)
	} else {
		unsubscribe, err = channel.Subscribe(context.Background(), name, ch.Receive)
	}
	return ch, unsubscribe, err
}

type PresenceChannel chan *ably.PresenceMessage

func (ch PresenceChannel) Receive(m *ably.PresenceMessage) {
	ch <- m
}

func ReceivePresenceMessages(channel *ably.RealtimeChannel, action *ably.PresenceAction) (messages <-chan *ably.PresenceMessage, unsubscribe func(), err error) {
	ch := make(PresenceChannel, 100)
	if action == nil {
		unsubscribe, err = channel.Presence.SubscribeAll(context.Background(), ch.Receive)
	} else {
		unsubscribe, err = channel.Presence.Subscribe(context.Background(), *action, ch.Receive)
	}
	return ch, unsubscribe, err
}

type AfterCall struct {
	Ctx      context.Context
	D        time.Duration
	Deadline time.Time
	Time     chan<- time.Time
}

func (c AfterCall) Fire() {
	c.Time <- c.Deadline
}

// TimeFuncs returns time functions to be passed as options.
//
// Now returns a stable time that is only updated with the times that the
// returned After produces.
//
// After forwards calls to the given channel. The receiver is in charge of
// sending the resulting time to the AfterCall.Time channel.
func TimeFuncs(afterCalls chan<- AfterCall) (
	now func() time.Time,
	after func(context.Context, time.Duration) <-chan time.Time,
) {
	var mtx sync.Mutex
	currentTime := time.Now()
	now = func() time.Time {
		mtx.Lock()
		defer mtx.Unlock()
		return currentTime
	}

	after = func(ctx context.Context, d time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)

		timeUpdate := make(chan time.Time, 1)
		go func() {
			mtx.Lock()
			t := currentTime
			mtx.Unlock()

			select {
			case afterCalls <- AfterCall{Ctx: ctx, D: d, Deadline: t.Add(d), Time: timeUpdate}:
			case <-ctx.Done():
				// This allows tests to ignore a call if they expect the timer to
				// be cancelled.
				return
			}

			select {
			case <-ctx.Done():
				close(ch)

			case t, ok := <-timeUpdate:
				if !ok {
					close(ch)
					return
				}
				mtx.Lock()
				currentTime = t
				mtx.Unlock()
				ch <- t
			}
		}()

		return ch
	}

	return now, after
}
