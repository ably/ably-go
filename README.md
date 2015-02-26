# [Ably](https://ably.io)

[![Build Status](https://travis-ci.org/ably/ably-go.png)](https://travis-ci.org/ably/ably-go)

A Go client library for [ably.io](https://ably.io), the real-time messaging service.

## Installation

```bash
go get github.com/ably/ably-go
```

## Using the Realtime API

### Subscribing to a channel

Given:

```go
client := ably.NewRealtimeClient(ably.ClientOptions{
  ApiKey: "xxxx"
})

channel := client.Channel('test')
```

Subscribe to all events:

```go
listener := ably.NewMessageListener(func(message protocol.Message) {
  message.Data
  message.Name
})

channel.Subscribe(listener)
```

Only certain events:

```go
listener := ably.NewMessageListener(func(message protocol.Message) {
  message.Data
  message.Name
})

channel.SubscribeTo("myEvent", listener)
```

### Publishing to a channel

```go
client := ably.NewRealtimeClient(ably.ClientOptions{ApiKey: "xxxx"})
channel := client.Channel("test")
channel.Publish("greeting", "Hello World!")
```

### Presence on a channel

```go
client := ably.NewRealtimeClient(ably.ClientOptions{ApiKey: "xxxx"})
channel := client.Channel("test")

message := PresenceMessage{ClientData: "john.doe"}
listener := ably.NewPresenceListener(func(presence *ably.PresenceMessage) {
  presence.Get() // => []realtime.Member
})

channel.Presence.Enter(message, listener)
```

## Using the REST API

### Publishing a message to a channel

```go
client := ably.NewRestClient(ably.ClientOptions{ApiKey: "xxxx"})
channel := client.Channel("test")
channel.Publish("myEvent", "Hello!")
```

### Fetching a channel's history

```go
client := ably.NewRestClient(ably.ClientOptions{ApiKey: "xxxx"})
channel := client.Channel("test")
channel.History() // =>  []Message
```

### Authentication with a token

```go
client := ably.NewRestClient(ably.ClientOptions{ApiKey: "xxxx"})
client.Auth.Authorize() // creates a token and will use token authentication moving forwards
client.Auth.CurrentToken() // => ably.Auth.Token
channel := client.Channel("test")
channel.Publish("myEvent", "Hello!") // => true, sent using token authentication
```

### Fetching your application's stats

```go
client := ably.NewRestClient(ably.ClientOptions{ApiKey: "xxxx"})
client.Stats() // => []Stat
```

### Fetching the Ably service time

```go
client := ably.NewRestClient(ably.ClientOptions{ApiKey: "xxxx"})
client.Time() // => time.Time
```

## Contributing

1. Fork it
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create new Pull Request
