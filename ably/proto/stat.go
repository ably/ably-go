package proto

import "time"

type Stat struct {
	// The following fields hold statistical information only for Channels
	// and Connections fields, for the rest they are going to be always zero.
	Peak   float64 `json:"peak" codec:"peak"`
	Mean   float64 `json:"mean" codec:"mean"`
	Opened float64 `json:"opened" codec:"opened"`

	// The following fields hold statistical information only for
	// APIRequests and TokenRequests fields, for the rest they are going
	// to always be zero.
	Succeeded int64 `json:"succeeded" codec:"succeeded"`
	Failed    int64 `json:"failed" codec:"failed"`

	// The following fields hold accordinly statistical information for
	// fields other than Channels and Connections - for them they are
	// going to always be zero.
	Count int64  `json:"count" codec:"count"`
	Data  uint64 `json:"data" codec:"data"`
}

type StatSummary struct {
	All      Stat `json:"all" codec:"all"`
	Messages Stat `json:"messages" codec:"messages"`

	// The following field holds statistical information only for
	// Persisted field, for the rest it's going to be always zero.
	Presence Stat `json:"presence" codec:"presence"`
}

type StatConnSummary struct {
	All      StatSummary `json:"all" codec:"all"`
	Realtime StatSummary `json:"realtime" codec:"realtime"`
}

type Stats struct {
	Count         int64           `json:"count" codec:"count"`
	Unit          string          `json:"unit" codec:"unit"`
	IntervalID    string          `json:"intervalId" codec:"intervalId"`
	Channels      Stat            `json:"channels" codec:"channels"`
	APIRequests   Stat            `json:"apiRequests" codec:"apiRequests"`
	TokenRequests Stat            `json:"tokenRequests" codec:"tokenRequests"`
	All           StatSummary     `json:"all" codec:"all"`
	Connections   StatSummary     `json:"connections" codec:"connections"`
	Persisted     StatSummary     `json:"persisted" codec:"persisted"`
	Inbound       StatConnSummary `json:"inbound" codec:"inbound"`
	Outbound      StatConnSummary `json:"outbound" codec:"outbound"`
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
