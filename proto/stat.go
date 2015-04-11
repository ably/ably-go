package proto

import "time"

type Stat struct {
	All           map[string]map[string]int            `json:"all"`
	Inbound       map[string]map[string]map[string]int `json:"inbound"`
	Outbound      map[string]map[string]map[string]int `json:"outbound"`
	Persisted     map[string]map[string]int            `json:"persisted"`
	Connections   map[string]map[string]int            `json:"connections"`
	Channels      map[string]float32                   `json:"channels"`
	ApiRequests   map[string]int                       `json:"apiRequests"`
	TokenRequests map[string]int                       `json:"tokenRequests"`
	Count         int                                  `json:"count"`
	IntervalId    string                               `json:"intervalId"`
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
