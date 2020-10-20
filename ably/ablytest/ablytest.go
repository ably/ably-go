package ablytest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
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
