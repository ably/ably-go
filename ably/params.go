package ably

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type ProtocolType string

const (
	ProtocolJSON    = "json"
	ProtocolMsgPack = "msgpack"
)

type Params struct {
	RealtimeEndpoint string
	RestEndpoint     string

	ApiKey       string
	ClientID     string
	AppID        string
	AppSecret    string
	UseTokenAuth bool

	Protocol ProtocolType
	Tls      bool

	HTTPClient *http.Client

	AblyLogger *AblyLogger
	LogLevel   string
}

func (p *Params) Prepare() {
	p.setLogger()

	if p.ApiKey != "" {
		p.parseApiKey()
	}
}

func (p *Params) parseApiKey() {
	keyParts := strings.Split(p.ApiKey, ":")

	if len(keyParts) != 2 {
		p.AblyLogger.Error("ApiKey doesn't use the right format. Ignoring this parameter.")
		return
	}

	p.AppID = keyParts[0]
	p.AppSecret = keyParts[1]
}

func (p *Params) setLogger() {
	if p.AblyLogger == nil {
		p.AblyLogger = &AblyLogger{
			Logger: log.New(os.Stdout, "", log.Lmicroseconds|log.Lshortfile),
		}
	}
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
		return fmt.Errorf("ScopeParams.Start can not be after ScopeParams.End")
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
