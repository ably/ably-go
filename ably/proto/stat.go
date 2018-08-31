package proto

import "time"

type Stats struct {
	Count         int64           `json:"count" codec:"count"`
	Unit          string          `json:"unit" codec:"unit"`
	IntervalID    string          `json:"intervalId" codec:"intervalId"`
	All           MessageTypes    `json:"all" codec:"all"`
	APIRequests   RequestCount    `json:"apiRequests" codec:"apiRequests"`
	Channels      ResourceCount   `json:"channels" codec:"channels"`
	Connections   ConnectionTypes `json:"connections" codec:"connections"`
	Inbound       MessageTraffick `json:"inbound" codec:"inbound"`
	Outbound      MessageTraffick `json:"outbound" codec:"outbound"`
	Persisted     MessageTypes    `json:"persisted" codec:"persisted"`
	TokenRequests RequestCount    `json:"tokenRequests" codec:"tokenRequests"`
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

type ResourceCount struct {
	Peak    float64 `json:"peak" codec:"peak"`
	Mean    float64 `json:"mean" codec:"mean"`
	Opened  float64 `json:"opened" codec:"opened"`
	Min     float64 `json:"min" codec:"min"`
	Refused float64 `json:"refused" codec:"refused"`
}

type MessageCount struct {
	Count int64  `json:"count" codec:"count"`
	Data  uint64 `json:"data" codec:"data"`
}

type ConnectionTypes struct {
	All   ResourceCount `json:"all" codec:"all"`
	Plain ResourceCount `json:"plain" codec:"plain"`
	TLS   ResourceCount `json:"tls" codec:"tls"`
}

type MessageTypes struct {
	All   MessageCount `json:"all" codec:"all"`
	Plain MessageCount `json:"plain" codec:"plain"`
	TLS   MessageCount `json:"tls" codec:"tls"`
}

type MessageTraffick struct {
	All      MessageTypes `json:"all" codec:"all"`
	RealTime MessageTypes `json:"realtime" codec:"realtime"`
	Rest     MessageTypes `json:"rest" codec:"rest"`
	Webhook  MessageTypes `json:"webhook" codec:"webhook"`
}

type RequestCount struct {
	Failed    float64 `json:"failed" codec:"failed"`
	Refused   float64 `json:"refused" codec:"refused"`
	Succeeded float64 `json:"succeeded" codec:"succeeded"`
}
