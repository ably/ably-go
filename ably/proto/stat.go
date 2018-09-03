package proto

import "time"

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
	Min     float64 `json:"min" codec:"min"`
	Mean    float64 `json:"mean" codec:"mean"`
	Opened  float64 `json:"opened" codec:"opened"`
	Failed  float64 `json:"failed" codec:"failed"`
	Refused float64 `json:"refused" codec:"refused"`
}

type ConnectionTypes struct {
	All   ResourceCount `json:"all" codec:"all"`
	Plain ResourceCount `json:"plain" codec:"plain"`
	TLS   ResourceCount `json:"tls" codec:"tls"`
}

type MessageCount struct {
	Count   float64 `json:"count" codec:"count"`
	Data    float64 `json:"data" codec:"data"`
	Failed  float64 `json:"failed" codec:"failed"`
	Refused float64 `json:"refused" codec:"refused"`
}

type MessageTypes struct {
	All      MessageCount `json:"all" codec:"all"`
	Plain    MessageCount `json:"plain" codec:"plain"`
	Presence MessageCount `json:"presence" codec:"presence"`
}

type MessageTraffic struct {
	All           MessageTypes `json:"all" codec:"all"`
	RealTime      MessageTypes `json:"realtime" codec:"realtime"`
	Rest          MessageTypes `json:"rest" codec:"rest"`
	Webhook       MessageTypes `json:"webhook" codec:"webhook"`
	Push          MessageTypes `json:"push" codec:"push"`
	ExternalQueue MessageTypes `json:"externalQueue" codec:"externalQueue"`
	SharedQueue   MessageTypes `json:"sharedQueue" codec:"sharedQueue"`
	HTTPEvent     MessageTypes `json:"httpEvent" codec:"httpEvent"`
}

type RequestCount struct {
	Failed    float64 `json:"failed" codec:"failed"`
	Refused   float64 `json:"refused" codec:"refused"`
	Succeeded float64 `json:"succeeded" codec:"succeeded"`
}

type PushTransportTypeCounter struct {
	Total float64 `json:"total" codec:"total"`
	GCM   float64 `json:"gcm" codec:"gcm"`
	FCM   float64 `json:"fcm" codec:"fcm"`
	APNS  float64 `json:"apns" codec:"apns"`
	Web   float64 `json:"web" codec:"web"`
}

type PushNotificationFailures struct {
	Retriable PushTransportTypeCounter `json:"retriable" codec:"retriable"`
	Final     PushTransportTypeCounter `json:"final" codec:"final"`
}

type PushNotifications struct {
	Invalid    float64                  `json:"invalid" codec:"invalid"`
	Attempted  PushTransportTypeCounter `json:"attempted" codec:"attempted"`
	Successful PushTransportTypeCounter `json:"successful" codec:"successful"`
	Failed     PushNotificationFailures `json:"failed" codec:"failed"`
}

type PushStats struct {
	Messages        float64           `json:"messages" codec:"messages"`
	Notifications   PushNotifications `json:"notifications" codec:"notifications"`
	DirectPublishes float64           `json:"directPublishes" codec:"directPublishes"`
}

type MessageDirections struct {
	All      MessageTypes   `json:"all" codec:"all"`
	Inbound  MessageTraffic `json:"inbound" codec:"inbound"`
	Outbound MessageTraffic `json:"outbound" codec:"outbound"`
}

type XchgMessages struct {
	All          MessageTypes `json:"all" codec:"all"`
	ProducerPaid MessageTypes `json:"producerPaid" codec:"producerPaid"`
	ConsumerPaid MessageTypes `json:"consumerPaid" codec:"consumerPaid"`
}

type ReactorRates struct {
	HTTPEvent float64 `json:"httpEvent" codec:"httpEvent"`
	AMQP      float64 `json:"amqp" codec:"amqp"`
}

type Rates struct {
	Messages      float64      `json:"messages" codec:"messages"`
	APIRequests   float64      `json:"apiRequests" codec:"apiRequests"`
	TokenRequests float64      `json:"tokenRequests" codec:"tokenRequests"`
	Reactor       ReactorRates `json:"reactor" codec:"reactor"`
}

type Stats struct {
	IntervalID string  `json:"intervalId" codec:"intervalId"`
	Unit       string  `json:"unit" codec:"unit"`
	InProgress string  `json:"inProgress" codec:"inProgress"`
	Count      float64 `json:"count" codec:"count"`

	All           MessageTypes    `json:"all" codec:"all"`
	Inbound       MessageTraffic  `json:"inbound" codec:"inbound"`
	Outbound      MessageTraffic  `json:"outbound" codec:"outbound"`
	Persisted     MessageTypes    `json:"persisted" codec:"persisted"`
	Connections   ConnectionTypes `json:"connections" codec:"connections"`
	Channels      ResourceCount   `json:"channels" codec:"channels"`
	APIRequests   RequestCount    `json:"apiRequests" codec:"apiRequests"`
	TokenRequests RequestCount    `json:"tokenRequests" codec:"tokenRequests"`
	Push          PushStats       `json:"push" codec:"push"`
	XchgProducer  XchgMessages    `json:"xchgProducer" codec:"xchgProducer"`
	XchgConsumer  XchgMessages    `json:"xchgConsumer" codec:"xchgConsumer"`
	PeakRates     Rates           `json:"peakRates" codec:"peakRates"`
}
