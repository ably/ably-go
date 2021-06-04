package ably

import (
	"fmt"
)

const (
	StatGranularityMinute = "minute"
	StatGranularityHour   = "hour"
	StatGranularityDay    = "day"
	StatGranularityMonth  = "month"
)

type StatsResourceCount struct {
	Peak    float64 `json:"peak" codec:"peak"`
	Min     float64 `json:"min" codec:"min"`
	Mean    float64 `json:"mean" codec:"mean"`
	Opened  float64 `json:"opened" codec:"opened"`
	Failed  float64 `json:"failed" codec:"failed"`
	Refused float64 `json:"refused" codec:"refused"`
}

type StatsConnectionTypes struct {
	All   StatsResourceCount `json:"all" codec:"all"`
	Plain StatsResourceCount `json:"plain" codec:"plain"`
	TLS   StatsResourceCount `json:"tls" codec:"tls"`
}

type StatsMessageCount struct {
	Count   float64 `json:"count" codec:"count"`
	Data    float64 `json:"data" codec:"data"`
	Failed  float64 `json:"failed" codec:"failed"`
	Refused float64 `json:"refused" codec:"refused"`
}

type StatsMessageTypes struct {
	All      StatsMessageCount `json:"all" codec:"all"`
	Messages StatsMessageCount `json:"messages" codec:"messages"`
	Presence StatsMessageCount `json:"presence" codec:"presence"`
}

type StatsMessageTraffic struct {
	All           StatsMessageTypes `json:"all" codec:"all"`
	RealTime      StatsMessageTypes `json:"realtime" codec:"realtime"`
	REST          StatsMessageTypes `json:"rest" codec:"rest"`
	Webhook       StatsMessageTypes `json:"webhook" codec:"webhook"`
	Push          StatsMessageTypes `json:"push" codec:"push"`
	ExternalQueue StatsMessageTypes `json:"externalQueue" codec:"externalQueue"`
	SharedQueue   StatsMessageTypes `json:"sharedQueue" codec:"sharedQueue"`
	HTTPEvent     StatsMessageTypes `json:"httpEvent" codec:"httpEvent"`
}

type StatsRequestCount struct {
	Failed    float64 `json:"failed" codec:"failed"`
	Refused   float64 `json:"refused" codec:"refused"`
	Succeeded float64 `json:"succeeded" codec:"succeeded"`
}

type StatsPushTransportTypeCounter struct {
	Total float64 `json:"total" codec:"total"`
	GCM   float64 `json:"gcm" codec:"gcm"`
	FCM   float64 `json:"fcm" codec:"fcm"`
	APNS  float64 `json:"apns" codec:"apns"`
	Web   float64 `json:"web" codec:"web"`
}

type StatsPushNotificationFailures struct {
	Retriable StatsPushTransportTypeCounter `json:"retriable" codec:"retriable"`
	Final     StatsPushTransportTypeCounter `json:"final" codec:"final"`
}

type StatsPushNotifications struct {
	Invalid    float64                       `json:"invalid" codec:"invalid"`
	Attempted  StatsPushTransportTypeCounter `json:"attempted" codec:"attempted"`
	Successful StatsPushTransportTypeCounter `json:"successful" codec:"successful"`
	Failed     StatsPushNotificationFailures `json:"failed" codec:"failed"`
}

type PushStats struct {
	Messages        float64                `json:"messages" codec:"messages"`
	Notifications   StatsPushNotifications `json:"notifications" codec:"notifications"`
	DirectPublishes float64                `json:"directPublishes" codec:"directPublishes"`
}

type StatsMessageDirections struct {
	All      StatsMessageTypes   `json:"all" codec:"all"`
	Inbound  StatsMessageTraffic `json:"inbound" codec:"inbound"`
	Outbound StatsMessageTraffic `json:"outbound" codec:"outbound"`
}

type StatsXchgMessages struct {
	All          StatsMessageTypes      `json:"all" codec:"all"`
	ProducerPaid StatsMessageDirections `json:"producerPaid" codec:"producerPaid"`
	ConsumerPaid StatsMessageDirections `json:"consumerPaid" codec:"consumerPaid"`
}

type StatsReactorRates struct {
	HTTPEvent float64 `json:"httpEvent" codec:"httpEvent"`
	AMQP      float64 `json:"amqp" codec:"amqp"`
}

type StatsRates struct {
	Messages      float64           `json:"messages" codec:"messages"`
	APIRequests   float64           `json:"apiRequests" codec:"apiRequests"`
	TokenRequests float64           `json:"tokenRequests" codec:"tokenRequests"`
	Reactor       StatsReactorRates `json:"reactor" codec:"reactor"`
}

type Stats struct {
	IntervalID string  `json:"intervalId" codec:"intervalId"`
	Unit       string  `json:"unit" codec:"unit"`
	InProgress string  `json:"inProgress" codec:"inProgress"`
	Count      float64 `json:"count" codec:"count"`

	All           StatsMessageTypes    `json:"all" codec:"all"`
	Inbound       StatsMessageTraffic  `json:"inbound" codec:"inbound"`
	Outbound      StatsMessageTraffic  `json:"outbound" codec:"outbound"`
	Persisted     StatsMessageTypes    `json:"persisted" codec:"persisted"`
	Connections   StatsConnectionTypes `json:"connections" codec:"connections"`
	Channels      StatsResourceCount   `json:"channels" codec:"channels"`
	APIRequests   StatsRequestCount    `json:"apiRequests" codec:"apiRequests"`
	TokenRequests StatsRequestCount    `json:"tokenRequests" codec:"tokenRequests"`
	Push          PushStats            `json:"push" codec:"push"`
	XchgProducer  StatsXchgMessages    `json:"xchgProducer" codec:"xchgProducer"`
	XchgConsumer  StatsXchgMessages    `json:"xchgConsumer" codec:"xchgConsumer"`
	PeakRates     StatsRates           `json:"peakRates" codec:"peakRates"`
}

func (s Stats) String() string {
	return fmt.Sprintf("<Stats %v; unit=%v; count=%v>", s.IntervalID, s.Unit, s.Count)
}
