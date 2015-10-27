package ablytest

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/internal/ablyutil"

	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
)

var Timeout = 5 * time.Second

func init() {
	ably.Log.Level = ably.LogVerbose
	if s := os.Getenv("ABLY_TIMEOUT"); s != "" {
		if t, err := time.ParseDuration(s); err == nil {
			Timeout = t
		}
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
		return msgpack.Marshal(in)
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
	return opts
}
