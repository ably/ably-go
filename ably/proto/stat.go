package proto

import "time"

type Stat struct {
	// The following fields hold statistical information only for Channels
	// and Connections fields, for the rest they are going to be always zero.
	Peak   int64 `json:"peak" msgpack:"peak"`
	Mean   int64 `json:"mean" msgpack:"mean"`
	Opened int64 `json:"opened" msgpack:"opened"`

	// The following fields hold statistical information only for
	// APIRequests and TokenRequests fields, for the rest they are going
	// to always be zero.
	Succeeded int64 `json:"succeeded" msgpack:"succeeded"`
	Failed    int64 `json:"failed" msgpack:"failed"`

	// The following fields hold accordinly statistical information for
	// fields other than Channels and Connections - for them they are
	// going to always be zero.
	Count int64  `json:"count" msgpack:"count"`
	Data  uint64 `json:"data" msgpack:"data"`
}

type StatSummary struct {
	All      Stat `json:"all" msgpack:"all"`
	Messages Stat `json:"messages" msgpack:"messages"`

	// The following field holds statistical information only for
	// Persisted field, for the rest it's going to be always zero.
	Presence Stat `json:"presence" msgpack:"presence"`
}

type StatConnSummary struct {
	All      StatSummary `json:"all" msgpack:"all"`
	Realtime StatSummary `json:"realtime" msgpack:"realtime"`
}

type Stats struct {
	Count         int64           `json:"count" msgpack:"count"`
	Unit          string          `json:"unit" msgpack:"unit"`
	IntervalID    string          `json:"intervalId" msgpack:"intervalId"`
	Channels      Stat            `json:"channels" msgpack:"channels"`
	APIRequests   Stat            `json:"apiRequests" msgpack:"apiRequests"`
	TokenRequests Stat            `json:"tokenRequests" msgpack:"tokenRequests"`
	All           StatSummary     `json:"all" msgpack:"all"`
	Connections   StatSummary     `json:"connections" msgpack:"connections"`
	Persisted     StatSummary     `json:"persisted" msgpack"persisted"`
	Inbound       StatConnSummary `json:"inbound" msgpack:"inbound"`
	Outbound      StatConnSummary `json:"outbound" msgpack:"outbound"`
}

const (
	StatGranularityMinute = "minute"
	StatGranularityHour   = "hour"
	StatGranularityDay    = "day"
	StatGranularityMonth  = "month"
)

var (
	intervalFormats = map[string]string{
		StatGranularityMinute: "2006-01-02:15:04",
		StatGranularityHour:   "2006-01-02:15",
		StatGranularityDay:    "2006-01-02",
		StatGranularityMonth:  "2006-01",
	}
)

func IntervalFormatFor(t time.Time, granulatity string) string {
	return t.Format(intervalFormats[granulatity])
}
