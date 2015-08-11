package proto

import "time"

type Stat struct {
	All           map[string]map[string]int            `json:"all" msgpack:"all"`
	Inbound       map[string]map[string]map[string]int `json:"inbound" msgpack:"inbound"`
	Outbound      map[string]map[string]map[string]int `json:"outbound" msgpack:"outbound"`
	Persisted     map[string]map[string]int            `json:"persisted" msgpack:"persisted"`
	Connections   map[string]map[string]int            `json:"connections" msgpack:"connections"`
	Channels      map[string]float32                   `json:"channels" msgpack:"channels"`
	ApiRequests   map[string]int                       `json:"apiRequests" msgpack:"apiRequests"`
	TokenRequests map[string]int                       `json:"tokenRequests" msgpack:"tokenRequests"`
	Count         int                                  `json:"count" msgpack:"count"`
	IntervalId    string                               `json:"intervalId" msgpack:"intervalId"`
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
