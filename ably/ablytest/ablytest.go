package ablytest

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"
)

var Timeout = 30 * time.Second
var NoBinaryProtocol bool
var DefaultLogger = ably.Logger{Level: ably.LogNone}
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
		DefaultLogger.Level = n
	}
	if s := os.Getenv("ABLY_ENV"); s != "" {
		Environment = s
	}
}

func MergeOptions(opts ...*ably.ClientOptions) *ably.ClientOptions {
	switch len(opts) {
	case 0:
		return nil
	case 1:
		return opts[0]
	}
	mergedOpts := opts[0]
	for _, opt := range opts[1:] {
		mergedOpts = mergeOpts(mergedOpts, opt)
	}
	return mergedOpts
}

func encode(typ string, in interface{}) ([]byte, error) {
	switch typ {
	case "application/json":
		return json.Marshal(in)
	case "application/x-msgpack":
		return ablyutil.Marshal(in)
	case "text/plain":
		return []byte(fmt.Sprintf("%v", in)), nil
	default:
		return nil, &ably.Error{
			StatusCode: 400,
			Code:       40000,
			Err:        fmt.Errorf("encoding error: unrecognized Content-Type: %q", typ),
		}
	}
}

func mergeOpts(opts, extra *ably.ClientOptions) *ably.ClientOptions {
	if extra == nil {
		return opts
	}
	ablyutil.Merge(opts, extra, false)
	ablyutil.Merge(&opts.AuthOptions, &extra.AuthOptions, false)
	ablyutil.Merge(&opts.Logger, &extra.Logger, false)
	return opts
}

func protocol(opts *ably.ClientOptions) string {
	if opts.NoBinaryProtocol {
		return "application/json"
	}
	return "application/x-msgpack"
}
