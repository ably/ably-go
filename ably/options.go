package ably

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ProtocolJSON    = "json"
	ProtocolMsgPack = "msgpack"
)

var errInvalidKey = errors.New("invalid key format")

var DefaultOptions = &ClientOptions{
	RestHost:     "rest.ably.io",
	RealtimeHost: "realtime.ably.io",
	Protocol:     ProtocolJSON, // TODO: make it ProtocolMsgPack
}

type ClientOptions struct {
	RestHost     string
	RealtimeHost string
	Key          string
	ClientID     string
	Protocol     string // either ProtocolJSON or ProtocolMsgPack
	UseTokenAuth bool
	NoTLS        bool

	HTTPClient *http.Client
}

func (opts *ClientOptions) restURL() string {
	host := opts.RestHost
	if host == "" {
		host = DefaultOptions.RestHost
	}
	if opts.NoTLS {
		return "http://" + host
	}
	return "https://" + host
}

func (opts *ClientOptions) realtimeURL() string {
	host := opts.RealtimeHost
	if host == "" {
		host = DefaultOptions.RealtimeHost
	}
	if opts.NoTLS {
		return "ws://" + host + ":80"
	}
	return "wss://" + host + ":443"
}

func (opts *ClientOptions) KeyName() string {
	if i := strings.IndexRune(opts.Key, ':'); i != -1 {
		return opts.Key[:i]
	}
	return ""
}

func (opts *ClientOptions) KeySecret() string {
	if i := strings.IndexRune(opts.Key, ':'); i != -1 {
		return opts.Key[i+1:]
	}
	return ""
}

func (opts *ClientOptions) httpclient() *http.Client {
	if opts.HTTPClient != nil {
		return opts.HTTPClient
	}
	return http.DefaultClient
}

func (opts *ClientOptions) protocol() string {
	if opts.Protocol != "" {
		return opts.Protocol
	}
	return ProtocolJSON // TODO: make it ProtocolMsgPack
}

// Timestamp returns the given time as a timestamp in milliseconds since epoch.
func Timestamp(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

// TimestampNow returns current time as a timestamp in milliseconds since epoch.
func TimestampNow() int64 {
	return Timestamp(time.Now())
}

// This needs to use a timestamp in millisecond
// Use the previous function to generate them from a time.Time struct.
type ScopeParams struct {
	Start int64
	End   int64
	Unit  string
}

func (s *ScopeParams) EncodeValues(out *url.Values) error {
	if s.Start != 0 && s.End != 0 && s.Start > s.End {
		return fmt.Errorf("start must be before end")
	}
	if s.Start != 0 {
		out.Set("start", strconv.FormatInt(s.Start, 10))
	}
	if s.End != 0 {
		out.Set("end", strconv.FormatInt(s.End, 10))
	}
	if s.Unit != "" {
		out.Set("unit", s.Unit)
	}
	return nil
}

type PaginateParams struct {
	Limit     int
	Direction string
	ScopeParams
}

func (p *PaginateParams) EncodeValues(out *url.Values) error {
	if p.Limit < 0 {
		out.Set("limit", strconv.Itoa(100))
	} else if p.Limit != 0 {
		out.Set("limit", strconv.Itoa(p.Limit))
	}
	switch p.Direction {
	case "":
		break
	case "backwards", "forwards":
		out.Set("direction", p.Direction)
		break
	default:
		return fmt.Errorf("Invalid value for direction: %s", p.Direction)
	}
	p.ScopeParams.EncodeValues(out)
	return nil
}
