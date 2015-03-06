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

### Introduction

Unlike the Realtime API, all calls are synchronous and are not run within an [EventMachine](https://github.com/eventmachine/eventmachine) [reactor](https://github.com/eventmachine/eventmachine/wiki/General-Introduction).

All examples assume a client and/or channel has been created as follows:

```ruby
client = ably.NewRestClient(api_key: "xxxxx")
channel = client.Channel('test')
```

### Publishing a message to a channel

```ruby
channel.Publish("myEvent", "Hello!") #=> true
```

### Querying the History

```ruby
channel.History()
```

### Presence on a channel

```ruby
channel.Presence.Get(nil) # => PaginatedResource
```

### Querying the Presence History

```ruby
channel.Presence.History(nil) # => PaginatedResource
```

### Generate Token and Token Request

```ruby
client.Auth.RequestToken()
client.Auth.CreateTokenRequest()
```

### Fetching your application's stats

```ruby
client.Stats() #=> PaginatedResource
```

## Support and feedback

Please visit https://support.ably.io/ for access to our knowledgebase and to ask for any assistance.

## Contributing

1. Fork it
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Ensure you have added suitable tests and the test suite is passing(`bundle exec rspec`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create a new Pull Request

## License

Copyright (c) 2015 Ably, Licensed under an MIT license.  Refer to [LICENSE.txt](LICENSE.txt) for the license terms.
