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
	ClientID         string
	ApiID            string
	ApiSecret        string
	UseTokenAuth     bool
	Protocol         string
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

	p.ApiID = keyParts[0]
	p.ApiSecret = keyParts[1]
}

// This needs to use a timestamp in millisecond
// Use the previous function to generate them from a time.Time struct.
type ScopeParams struct {
	Start int64
	End   int64
	Unit  string
}

type TimestampMillisecond struct {
	time.Time
}

// Get a timestamp in millisecond
func NewTimestamp(t time.Time) int64 {
	tm := TimestampMillisecond{Time: t}
	return tm.ToInt()
}

func (t *TimestampMillisecond) ToInt() int64 {
	return int64(time.Nanosecond * time.Duration(t.UnixNano()) / time.Millisecond)
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
