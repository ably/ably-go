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
	RestHost:          "rest.ably.io",
	RealtimeHost:      "realtime.ably.io",
	Protocol:          ProtocolMsgPack,
	TimeoutConnect:    15 * time.Second,
	TimeoutDisconnect: 30 * time.Second,
	TimeoutSuspended:  2 * time.Minute,
}

type ClientOptions struct {
	RestHost     string // optional; overwrite endpoint hostname for REST client
	RealtimeHost string // optional; overwrite endpoint hostname for Realtime client
	Environment  string // optional; prefixes both hostname with the environment string
	Key          string // an authorization key in the 'name:secret' format
	ClientID     string // optional; required for realtime presence
	Protocol     string // optional; either ProtocolJSON or ProtocolMsgPack
	Recover      string // optional; used to recover client state
	Token        *Token // optional; is used for authorization when UseTokenAuth is true

	UseBinaryProtocol bool // when true uses msgpack for network serialization protocol
	UseTokenAuth      bool // when true REST and realtime client will use token authentication

	NoTLS      bool // when true REST and realtime client won't use TLS
	NoConnect  bool // when true realtime client will not attempt to connect automatically
	NoEcho     bool // when true published messages will not be echoed back
	NoQueueing bool // when true drops messages published during regaining connection

	TimeoutConnect    time.Duration // time period after which connect request is failed
	TimeoutDisconnect time.Duration // time period after which disconnect request is failed
	TimeoutSuspended  time.Duration // time period after which no more reconnection attempts are performed

	// Dial specifies the dial function for creating message connections used
	// by RealtimeClient.
	// If Dial is nil, the default websocket connection is used.
	Dial func(protocol string, u *url.URL) (MsgConn, error)

	// Listener if set, will be automatically registered with On method for every
	// realtime connection and realtime channel created by realtime client.
	// The listener will receive events for all state transitions.
	Listener chan<- State

	// HTTPClient specifies the client used for HTTP communication by RestClient.
	// If HTTPClient is nil, the http.DefaultClient is used.
	HTTPClient *http.Client
}

func (opts *ClientOptions) timeoutConnect() time.Duration {
	if opts.TimeoutConnect != 0 {
		return opts.TimeoutConnect
	}
	return DefaultOptions.TimeoutConnect
}

func (opts *ClientOptions) timeoutDisconnect() time.Duration {
	if opts.TimeoutDisconnect != 0 {
		return opts.TimeoutDisconnect
	}
	return DefaultOptions.TimeoutDisconnect
}

func (opts *ClientOptions) timeoutSuspended() time.Duration {
	if opts.TimeoutSuspended != 0 {
		return opts.TimeoutSuspended
	}
	return DefaultOptions.TimeoutSuspended
}

func (opts *ClientOptions) restURL() string {
	host := opts.RestHost
	if host == "" {
		host = DefaultOptions.RestHost
		if opts.Environment != "" {
			host = opts.Environment + "-" + host
		}
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
		if opts.Environment != "" {
			host = opts.Environment + "-" + host
		}
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
	return DefaultOptions.Protocol
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
