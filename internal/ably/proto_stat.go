package ably

import (
	"fmt"
)

const (
	// StatGranularityMinute specifies interval unit over which statistics are gathered as minutes.
	StatGranularityMinute = "minute"
	// StatGranularityHour specifies interval unit over which statistics are gathered as hours.
	StatGranularityHour = "hour"
	// StatGranularityDay specifies interval unit over which statistics are gathered as days.
	StatGranularityDay = "day"
	// StatGranularityMonth specifies interval unit over which statistics are gathered as months.
	StatGranularityMonth = "month"
)

type StatsResourceCount struct {
	// Peak is the peak number of resources of this type used for this period (TS9b).
	Peak float64 `json:"peak" codec:"peak"`
	// Min is the minimum total resources of this type used for this period (TS9d).
	Min float64 `json:"min" codec:"min"`
	// Mean is the average number of resources of this type used for this period (TS9c).
	Mean float64 `json:"mean" codec:"mean"`
	// Opened is the total number of resources opened of this type (TS9a).
	Opened float64 `json:"opened" codec:"opened"`
	Failed float64 `json:"failed" codec:"failed"`
	// Refused is the number of resource requests refused within this period (TS9e).
	Refused float64 `json:"refused" codec:"refused"`
}

// StatsConnectionTypes contains a breakdown of summary stats data for different (TLS vs non-TLS) connection types.
type StatsConnectionTypes struct {
	// All is a [ably.StatsResourceCount] object containing a breakdown of usage by scope over
	// TLS connections (both TLS and non-TLS) (TS4c).
	All StatsResourceCount `json:"all" codec:"all"`
	// Plain is a [ably.StatsResourceCount] object containing a breakdown of usage by scope over non-TLS connections (TS4b).
	Plain StatsResourceCount `json:"plain" codec:"plain"`
	// TLS is a [ably.StatsResourceCount] object containing a breakdown of usage by scope over TLS connections (TS4a).
	TLS StatsResourceCount `json:"tls" codec:"tls"`
}

// StatsMessageCount contains the aggregate counts for messages and data transferred.
type StatsMessageCount struct {
	// Count is the count of all messages (TS5a).
	Count float64 `json:"count" codec:"count"`
	// Data is the total number of bytes transferred for all messages (TS5b).
	Data    float64 `json:"data" codec:"data"`
	Failed  float64 `json:"failed" codec:"failed"`
	Refused float64 `json:"refused" codec:"refused"`
}

// StatsMessageTypes contains a breakdown of summary stats data for different (channel vs presence) message types.
type StatsMessageTypes struct {
	// All is a [ably.StatsMessageCount] object containing the count and byte value of messages and presence messages (TS6c).
	All StatsMessageCount `json:"all" codec:"all"`
	// Messages is a [ably.StatsMessageCount] object containing the count and byte value of messages (TS6a).
	Messages StatsMessageCount `json:"messages" codec:"messages"`
	// Presence is a [ably.StatsMessageCount] object containing the count and byte value of presence messages (TS6b).
	Presence StatsMessageCount `json:"presence" codec:"presence"`
}

type StatsMessageTraffic struct {
	// All is a [ably.StatsMessageTypes] object containing a breakdown of usage by message type for all messages
	// (includes realtime, rest and webhook messages) (TS7d).
	All StatsMessageTypes `json:"all" codec:"all"`
	// RealTime is a [ably.StatsMessageTypes] object containing a breakdown of usage by message type for messages
	// transferred over a realtime transport such as WebSocket (TS7a).
	RealTime StatsMessageTypes `json:"realtime" codec:"realtime"`
	// REST is a [ably.StatsMessageTypes] object containing a breakdown of usage by message type for messages
	// transferred over a rest transport such as WebSocket (TS7b).
	REST StatsMessageTypes `json:"rest" codec:"rest"`
	// Webhook is a [ably.StatsMessageTypes] object containing a breakdown of usage by message type for
	// messages delivered using webhooks (TS7c).
	Webhook StatsMessageTypes `json:"webhook" codec:"webhook"`

	Push StatsMessageTypes `json:"push" codec:"push"`

	ExternalQueue StatsMessageTypes `json:"externalQueue" codec:"externalQueue"`

	SharedQueue StatsMessageTypes `json:"sharedQueue" codec:"sharedQueue"`

	HTTPEvent StatsMessageTypes `json:"httpEvent" codec:"httpEvent"`
}

type StatsRequestCount struct {
	// Failed is the number of requests that failed (TS8b).
	Failed float64 `json:"failed" codec:"failed"`
	// Refused is the number of requests that were refused, typically as a result of permissions or
	// a limit being exceeded (TS8c).
	Refused float64 `json:"refused" codec:"refused"`
	// Succeeded is the number of requests that succeeded (TS8a).
	Succeeded float64 `json:"succeeded" codec:"succeeded"`
}

type StatsPushTransportTypeCounter struct {
	Total float64 `json:"total" codec:"total"`

	GCM float64 `json:"gcm" codec:"gcm"`

	FCM float64 `json:"fcm" codec:"fcm"`

	APNS float64 `json:"apns" codec:"apns"`

	Web float64 `json:"web" codec:"web"`
}

type StatsPushNotificationFailures struct {
	Retriable StatsPushTransportTypeCounter `json:"retriable" codec:"retriable"`

	Final StatsPushTransportTypeCounter `json:"final" codec:"final"`
}

type StatsPushNotifications struct {
	Invalid float64 `json:"invalid" codec:"invalid"`

	Attempted StatsPushTransportTypeCounter `json:"attempted" codec:"attempted"`

	Successful StatsPushTransportTypeCounter `json:"successful" codec:"successful"`

	Failed StatsPushNotificationFailures `json:"failed" codec:"failed"`
}

// PushStats provides details the stats on push notifications.
type PushStats struct {
	// Messages are total number of push messages (TS10a).
	Messages float64 `json:"messages" codec:"messages"`
	// Notifications is the count of push notifications (TS10c).
	Notifications StatsPushNotifications `json:"notifications" codec:"notifications"`
	// DirectPublishes is the total number of direct publishes (TS10b).
	DirectPublishes float64 `json:"directPublishes" codec:"directPublishes"`
}

type StatsMessageDirections struct {
	// All is a [ably.StatsMessageTypes] object containing a breakdown of usage by message type for
	// messages published and received (TS14a).
	All StatsMessageTypes `json:"all" codec:"all"`
	// Inbound is a [ably.StatsMessageTraffic] object containing a breakdown of usage by transport type
	// for received messages (TS14b).
	Inbound StatsMessageTraffic `json:"inbound" codec:"inbound"`
	// Outbound is a [ably.StatsMessageTraffic] object containing a breakdown of usage by transport type
	// for published messages (TS14c).
	Outbound StatsMessageTraffic `json:"outbound" codec:"outbound"`
}

type StatsXchgMessages struct {
	// All is a [ably.StatsMessageTypes] object containing a breakdown of usage by message type
	// for the Ably API Streamer (TS11a).
	All StatsMessageTypes `json:"all" codec:"all"`

	// ProducerPaid is a [ably.StatsMessageDirections] object containing a breakdown of usage, by messages
	// published and received, for the API Streamer charged to the producer (TS11b).
	ProducerPaid StatsMessageDirections `json:"producerPaid" codec:"producerPaid"`

	// ConsumerPaid is a [ably.StatsMessageDirections] object containing a breakdown of usage, by messages
	// published and received, for the API Streamer charged to the consumer (TS11c).
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

// Stats contains application statistics for a specified time interval and time period.
type Stats struct {

	// IntervalID is the UTC time at which the time period covered begins.
	// If unit is set to minute this will be in the format YYYY-mm-dd:HH:MM, if hour it will be YYYY-mm-dd:HH,
	// if day it will be YYYY-mm-dd:00 and if month it will be YYYY-mm-01:00 (TS12a).
	IntervalID string `json:"intervalId" codec:"intervalId"`

	// Unit is the length of the interval the stats span.
	// Values will be a [ably.StatGranularityMinute], [ably.StatGranularityMinute], [ably.StatGranularityMinute] or
	// [ably.StatGranularityMinute] (TS12c).
	Unit string `json:"unit" codec:"unit"`

	InProgress string `json:"inProgress" codec:"inProgress"`

	Count float64 `json:"count" codec:"count"`

	// All is a [ably.StatsMessageTypes] object containing the aggregate count of
	// all message stats (TS12e).
	All StatsMessageTypes `json:"all" codec:"all"`

	// Inbound is a [ably.StatsMessageTraffic] object containing the aggregate count of
	// inbound message stats (TS12f).
	Inbound StatsMessageTraffic `json:"inbound" codec:"inbound"`

	// Outbound is a [ably.StatsMessageTraffic] object containing the aggregate count of
	// outbound message stats (TS12g).
	Outbound StatsMessageTraffic `json:"outbound" codec:"outbound"`

	// Persisted is a [ably.StatsMessageTypes] object containing the aggregate count of
	// persisted message stats (TS12h).
	Persisted StatsMessageTypes `json:"persisted" codec:"persisted"`

	// Connections is a [ably.StatsConnectionTypes] object containing a breakdown of connection related stats, such as min,
	// mean and peak connections (TS12i).
	Connections StatsConnectionTypes `json:"connections" codec:"connections"`

	// Channels is a [ably.StatsResourceCount] object containing a breakdown of channels (TS12j).
	Channels StatsResourceCount `json:"channels" codec:"channels"`

	// APIRequests is a [ably.StatsRequestCount] object containing a breakdown of API Requests (TS12k).
	APIRequests StatsRequestCount `json:"apiRequests" codec:"apiRequests"`

	// TokenRequests is a [ably.StatsRequestCount] object containing a breakdown of Ably Token requests (TS12l).
	TokenRequests StatsRequestCount `json:"tokenRequests" codec:"tokenRequests"`

	// Push is a [ably.PushStats] object containing a breakdown of stats on push notifications (TS12m).
	Push PushStats `json:"push" codec:"push"`

	// XchgProducer is a [ably.StatsXchgMessages] is a object containing data about usage of
	// Ably API Streamer as a producer (TS12n).
	XchgProducer StatsXchgMessages `json:"xchgProducer" codec:"xchgProducer"`

	// XchgConsumer is a [ably.StatsXchgMessages] object containing data about usage of
	// Ably API Streamer as a consumer (TS12o).
	XchgConsumer StatsXchgMessages `json:"xchgConsumer" codec:"xchgConsumer"`

	PeakRates StatsRates `json:"peakRates" codec:"peakRates"`
}

func (s Stats) String() string {
	return fmt.Sprintf("<Stats %v; unit=%v; count=%v>", s.IntervalID, s.Unit, s.Count)
}
