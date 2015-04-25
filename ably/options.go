package ably

import (
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

type ClientOptions struct {
	RealtimeEndpoint string
	RestEndpoint     string
	Key              string
	Token            string
	Secret           string
	ClientID         string
	Protocol         string
	UseTokenAuth     bool
	NoTLS            bool

	HTTPClient *http.Client
}

func (p *ClientOptions) Prepare() {
	if p.Key != "" {
		p.parseKey()
	}
}

func (p *ClientOptions) parseKey() {
	keyParts := strings.Split(p.Key, ":")

	if len(keyParts) != 2 {
		Log.Print(LogError, "invalid key format, ignoring")
		return
	}

	p.Token = keyParts[0]
	p.Secret = keyParts[1]
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
