package ably

import (
	"fmt"
)

const (
	// **CANONICAL**
	// Interval unit over which statistics are gathered as minutes.
	StatGranularityMinute = "minute"
	// **CANONICAL**
	// Interval unit over which statistics are gathered as hours.
	StatGranularityHour   = "hour"
	// **CANONICAL**
	// Interval unit over which statistics are gathered as days.
	StatGranularityDay    = "day"
	// **CANONICAL**
	// Interval unit over which statistics are gathered as months.
	StatGranularityMonth  = "month"
)

type StatsResourceCount struct {
	// **CANONICAL**
	// The peak number of resources of this type used for this period.
	// TS9b
	Peak    float64 `json:"peak" codec:"peak"`
	// **CANONICAL**
	// The minimum total resources of this type used for this period.
	// TS9d
	Min     float64 `json:"min" codec:"min"`
	// **CANONICAL**
	// The average number of resources of this type used for this period.
	// TS9c
	Mean    float64 `json:"mean" codec:"mean"`
	// **CANONICAL**
	// The total number of resources opened of this type.
	// TS9a
	Opened  float64 `json:"opened" codec:"opened"`
	// **CANONICAL**
	//
	Failed  float64 `json:"failed" codec:"failed"`
	// **CANONICAL**
	// The number of resource requests refused within this period.
	// TS9e
	Refused float64 `json:"refused" codec:"refused"`
}

// **CANONICAL**
// Contains a breakdown of summary stats data for different (TLS vs non-TLS) connection types.
type StatsConnectionTypes struct {
	// **CANONICAL**
	// A [Stats.ResourceCount]{@link Stats.ResourceCount} object containing a breakdown of usage by scope over TLS connections (both TLS and non-TLS).
	// TS4c
	All   StatsResourceCount `json:"all" codec:"all"`
	// **CANONICAL**
	// A [Stats.ResourceCount]{@link Stats.ResourceCount} object containing a breakdown of usage by scope over non-TLS connections.
	// TS4b
	Plain StatsResourceCount `json:"plain" codec:"plain"`
	// **CANONICAL**
	// A [Stats.ResourceCount]{@link Stats.ResourceCount} object containing a breakdown of usage by scope over TLS connections.
	// TS4a
	TLS   StatsResourceCount `json:"tls" codec:"tls"`
}

// **CANONICAL**
// Contains the aggregate counts for messages and data transferred.
type StatsMessageCount struct {
	// **CANONICAL**
	// The count of all messages.
	// TS5a
	Count   float64 `json:"count" codec:"count"`
	// **CANONICAL**
	// The total number of bytes transferred for all messages.
	// TS5b
	Data    float64 `json:"data" codec:"data"`
	// **CANONICAL**
	Failed  float64 `json:"failed" codec:"failed"`
	// **CANONICAL**
	Refused float64 `json:"refused" codec:"refused"`
}

// **CANONICAL**
// Contains a breakdown of summary stats data for different (channel vs presence) message types.
type StatsMessageTypes struct {
	// **CANONICAL**
	// A [Stats.MessageCount]{@link Stats.MessageCount} object containing the count and byte value of messages and presence messages.
	// TS6c
	All      StatsMessageCount `json:"all" codec:"all"`
	// **CANONICAL**
	// A [Stats.MessageCount]{@link Stats.MessageCount} object containing the count and byte value of messages.
	// TS6a
	Messages StatsMessageCount `json:"messages" codec:"messages"`
	// **CANONICAL**
	// A [Stats.MessageCount]{@link Stats.MessageCount} object containing the count and byte value of presence messages.
	// TS6b
	Presence StatsMessageCount `json:"presence" codec:"presence"`
}

type StatsMessageTraffic struct {
	// **CANONICAL**
	// A [Stats.MessageTypes]{@link Stats.MessageTypes} object containing a breakdown of usage by message type for all messages (includes realtime, rest and webhook messages).
	// TS7d
	All           StatsMessageTypes `json:"all" codec:"all"`
	// **CANONICAL**
	// A [Stats.MessageTypes]{@link Stats.MessageTypes} object containing a breakdown of usage by message type for messages transferred over a realtime transport such as WebSocket.
	// TS7a
	RealTime      StatsMessageTypes `json:"realtime" codec:"realtime"`
	// **CANONICAL**
	// A [Stats.MessageTypes]{@link Stats.MessageTypes} object containing a breakdown of usage by message type for messages transferred over a rest transport such as WebSocket.
	// TS7b
	REST          StatsMessageTypes `json:"rest" codec:"rest"`
	// **CANONICAL**
	// A [Stats.MessageTypes]{@link Stats.MessageTypes} object containing a breakdown of usage by message type for messages delivered using webhooks.
	// TS7c
	Webhook       StatsMessageTypes `json:"webhook" codec:"webhook"`
	// **CANONICAL**
	Push          StatsMessageTypes `json:"push" codec:"push"`

	// **CANONICAL**
	ExternalQueue StatsMessageTypes `json:"externalQueue" codec:"externalQueue"`

	// **CANONICAL**
	SharedQueue   StatsMessageTypes `json:"sharedQueue" codec:"sharedQueue"`

	// **CANONICAL**
	HTTPEvent     StatsMessageTypes `json:"httpEvent" codec:"httpEvent"`
}

type StatsRequestCount struct {
	// **CANONICAL**
	// The number of requests that failed.
	// TS8b
	Failed    float64 `json:"failed" codec:"failed"`
	// **CANONICAL**
	// The number of requests that were refused, typically as a result of permissions or a limit being exceeded.
	// TS8c
	Refused   float64 `json:"refused" codec:"refused"`
	// **CANONICAL**
	// The number of requests that succeeded.
	// TS8a
	Succeeded float64 `json:"succeeded" codec:"succeeded"`
}

type StatsPushTransportTypeCounter struct {
	// **CANONICAL**
	Total float64 `json:"total" codec:"total"`
	// **CANONICAL**
	GCM   float64 `json:"gcm" codec:"gcm"`
	// **CANONICAL**
	FCM   float64 `json:"fcm" codec:"fcm"`
	// **CANONICAL**
	APNS  float64 `json:"apns" codec:"apns"`
	// **CANONICAL**
	Web   float64 `json:"web" codec:"web"`
}

type StatsPushNotificationFailures struct {
	// **CANONICAL**
	Retriable StatsPushTransportTypeCounter `json:"retriable" codec:"retriable"`
	// **CANONICAL**
	Final     StatsPushTransportTypeCounter `json:"final" codec:"final"`
}

type StatsPushNotifications struct {
	// **CANONICAL**
	Invalid    float64                       `json:"invalid" codec:"invalid"`
	// **CANONICAL**
	Attempted  StatsPushTransportTypeCounter `json:"attempted" codec:"attempted"`
	// **CANONICAL**
	Successful StatsPushTransportTypeCounter `json:"successful" codec:"successful"`
	// **CANONICAL**
	Failed     StatsPushNotificationFailures `json:"failed" codec:"failed"`
}

// **CANONICAL**
// Details the stats on push notifications.
type PushStats struct {
	// **CANONICAL**
	// Total number of push messages.
	// TS10a
	Messages        float64                `json:"messages" codec:"messages"`
	// **CANONICAL**
	// The count of push notifications.
	// TS10c
	Notifications   StatsPushNotifications `json:"notifications" codec:"notifications"`
	// **CANONICAL**
	// Total number of direct publishes.
	// TS10b
	DirectPublishes float64                `json:"directPublishes" codec:"directPublishes"`
}

type StatsMessageDirections struct {
	// **CANONICAL**
	// A [Stats.MessageTypes]{@link Stats.MessageTypes} object containing a breakdown of usage by message type for messages published and received.
	// TS14a
	All      StatsMessageTypes   `json:"all" codec:"all"`
	// **CANONICAL**
	// A [Stats.MessageTraffic]{@link Stats.MessageTraffic} object containing a breakdown of usage by transport type for received messages.
	// TS14b
	Inbound  StatsMessageTraffic `json:"inbound" codec:"inbound"`
	// **CANONICAL**
	// A [Stats.MessageTraffic]{@link Stats.MessageTraffic} object containing a breakdown of usage by transport type for published messages.
	// TS14c
	Outbound StatsMessageTraffic `json:"outbound" codec:"outbound"`
}

type StatsXchgMessages struct {
	// **CANONICAL**
	// A [Stats.MessageTypes]{@link Stats.MessageTypes} object containing a breakdown of usage by message type for the Ably API Streamer.
	// TS11a
	All          StatsMessageTypes      `json:"all" codec:"all"`
	// **CANONICAL**
	// A [Stats.MessageDirections]{@link Stats.MessageDirections} object containing a breakdown of usage, by messages published and received, for the API Streamer charged to the producer.
	// TS11b
	ProducerPaid StatsMessageDirections `json:"producerPaid" codec:"producerPaid"`
	// **CANONICAL**
	// A [Stats.MessageDirections]{@link Stats.MessageDirections} object containing a breakdown of usage, by messages published and received, for the API Streamer charged to the consumer.
	// TS11c
	ConsumerPaid StatsMessageDirections `json:"consumerPaid" codec:"consumerPaid"`
}

type StatsReactorRates struct {
	// **CANONICAL**
	HTTPEvent float64 `json:"httpEvent" codec:"httpEvent"`
	// **CANONICAL**
	AMQP      float64 `json:"amqp" codec:"amqp"`
}

type StatsRates struct {
	// **CANONICAL**
	Messages      float64           `json:"messages" codec:"messages"`
	// **CANONICAL**
	APIRequests   float64           `json:"apiRequests" codec:"apiRequests"`
	// **CANONICAL**
	TokenRequests float64           `json:"tokenRequests" codec:"tokenRequests"`
	// **CANONICAL**
	Reactor       StatsReactorRates `json:"reactor" codec:"reactor"`
}

// **CANONICAL**
// Contains application statistics for a specified time interval and time period.
type Stats struct {
	// **CANONICAL**
	// The UTC time at which the time period covered begins. If unit is set to minute this will be in the format YYYY-mm-dd:HH:MM, if hour it will be YYYY-mm-dd:HH, if day it will be YYYY-mm-dd:00 and if month it will be YYYY-mm-01:00.
	// TS12a
	IntervalID string  `json:"intervalId" codec:"intervalId"`
	// **CANONICAL**
	// The length of the interval the stats span. Values will be a [StatsIntervalGranularity]{@link StatsIntervalGranularity}.
	// TS12c
	Unit       string  `json:"unit" codec:"unit"`
	// **CANONICAL**
	//
	InProgress string  `json:"inProgress" codec:"inProgress"`
	// **CANONICAL**

	Count      float64 `json:"count" codec:"count"`
	// **CANONICAL**
	// A [Stats.MessageTypes]{@link Stats.MessageTypes} object containing the aggregate count of all message stats.
	// TS12e
	All           StatsMessageTypes    `json:"all" codec:"all"`
	// **CANONICAL**
	// A [Stats.MessageTraffic]{@link Stats.MessageTraffic} object containing the aggregate count of inbound message stats.
	// TS12f
	Inbound       StatsMessageTraffic  `json:"inbound" codec:"inbound"`
	// **CANONICAL**
	// A [Stats.MessageTraffic]{@link Stats.MessageTraffic} object containing the aggregate count of outbound message stats.
	// TS12g
	Outbound      StatsMessageTraffic  `json:"outbound" codec:"outbound"`
	// **CANONICAL**
	// A [Stats.MessageTypes]{@link Stats.MessageTypes} object containing the aggregate count of persisted message stats.
	// TS12h
	Persisted     StatsMessageTypes    `json:"persisted" codec:"persisted"`
	// **CANONICAL**
	// A [Stats.ConnectionTypes]{@link Stats.ConnectionTypes} object containing a breakdown of connection related stats, such as min, mean and peak connections.
	// TS12i
	Connections   StatsConnectionTypes `json:"connections" codec:"connections"`
	// **CANONICAL**
	// A [Stats.ResourceCount]{@link Stats.ResourceCount} object containing a breakdown of channels.
	// TS12j
	Channels      StatsResourceCount   `json:"channels" codec:"channels"`
	// **CANONICAL**
	// A [Stats.RequestCount]{@link Stats.RequestCount} object containing a breakdown of API Requests.
	// TS12k
	APIRequests   StatsRequestCount    `json:"apiRequests" codec:"apiRequests"`
	// **CANONICAL**
	// A [Stats.RequestCount]{@link Stats.RequestCount} object containing a breakdown of Ably Token requests.
	// TS12l
	TokenRequests StatsRequestCount    `json:"tokenRequests" codec:"tokenRequests"`
	// **CANONICAL**
	// A [Stats.PushStats]{@link Stats.PushStats} object containing a breakdown of stats on push notifications.
	// TS12m
	Push          PushStats            `json:"push" codec:"push"`
	// **CANONICAL**
	// A [Stats.XchgMessages]{@link Stats.XchgMessages} object containing data about usage of Ably API Streamer as a producer.
	// TS12n
	XchgProducer  StatsXchgMessages    `json:"xchgProducer" codec:"xchgProducer"`
	// **CANONICAL**
	// A [Stats.XchgMessages]{@link Stats.XchgMessages} object containing data about usage of Ably API Streamer as a consumer.
	// TS12o
	XchgConsumer  StatsXchgMessages    `json:"xchgConsumer" codec:"xchgConsumer"`
	// **CANONICAL**

	PeakRates     StatsRates           `json:"peakRates" codec:"peakRates"`
}

func (s Stats) String() string {
	return fmt.Sprintf("<Stats %v; unit=%v; count=%v>", s.IntervalID, s.Unit, s.Count)
}
