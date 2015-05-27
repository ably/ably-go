[Ably](https://ably.io) [![Build Status](https://travis-ci.org/ably/ably-go.png)](https://travis-ci.org/ably/ably-go)  [![Coverage Status](https://coveralls.io/repos/ably/ably-go/badge.svg)](https://coveralls.io/r/ably/ably-go)
----


A Go client library for [ably.io](https://ably.io), the real-time messaging service.

## Installation

```bash
~ $ go get -u github.com/ably/ably-go/ably
```

## Using the Realtime API

### Creating a client

```go
client, err := ably.NewRealtimeClient(&ably.ClientOptions{Key: "xxx:xxx"})
if err != nil {
	panic(err)
}

channel := client.Channel.Get("test")
```

### Subscribing to a channel for all events

```go
sub, err := channel.Subscribe()
if err != nil {
	panic(err)
}

for msg := range sub.MessageChannel() {
	fmt.Println("Received message:", msg)
}
```

### Subscribing to a channel for `EventName1` and `EventName2` events

```go
sub, err := channel.Subscribe("EventName1", "EventName2")
if err != nil {
	panic(err)
}

for msg := range sub.MessageChannel() {
	fmt.Println("Received message:", msg)
}
```

### Publishing to a channel

```go
// send request to a server
res, err := channel.Publish("EventName1", "EventData1")
if err != nil {
	panic(err)
}

// await confirmation
if err = res.Wait(); err != nil {
	panic(err)
}
```

### Announcing presence on a channel

```go
// send request to a server
res, err := channel.Presence.Enter("presence data")
if err != nil {
	panic(err)
}

// await confirmation
if err = res.Wait(); err != nil {
	panic(err)
}
```

### Announcing presence on a channel on behalf of other client

```go
// send request to a server
res, err := channel.Presence.EnterClient("clientID", "presence data")
if err != nil {
	panic(err)
}

// await confirmation
if err = res.Wait(); err != nil {
	panic(err)
}
```

### Getting all clients present on a channel

```go
clients := channel.Presence.Get(true)

for _, client := range clients {
	fmt.Println("Present client:", client)
}
```

### Subscribing to all presence messages

```go
sub, err := channel.Presence.Subscribe()
if err != nil {
	panic(err)
}

for msg := range sub.PresenceChannel() {
	fmt.Println("Presence event:", msg)
}
```

### Subscribing to 'Enter' presence messages only

```go
sub, err := channel.Presence.Subscribe(proto.PresenceEnter)
if err != nil {
	panic(err)
}

for msg := range sub.PresenceChannel() {
	fmt.Println("Presence event:", msg)
}
```

## Using the REST API

### Introduction

All examples assume a client and/or channel has been created as follows:

```go
client, err := ably.NewRestClient(&ably.ClientOptions{Key: "xxx:xxx"})
if err != nil {
	panic(err)
}

channel := client.Channel("test")
```

### Publishing a message to a channel

```go
err := channel.Publish("HelloEvent", "Hello!")
if err != nil {
	panic(err)
}
```

### Querying the History

```go
page, err := channel.History(nil)
for ; err == nil; page, err = page.Next() {
	for _, message := range page.Messages() {
		fmt.Println(message)
	}
}
if err != nil {
	panic(err)
}
```

### Presence on a channel

```go
page, err := channel.Presence.Get(nil)
for ; err == nil; page, err = page.Next() {
	for _, presence := range page.PresenceMessages() {
		fmt.Println(presence)
	}
}
if err != nil {
	panic(err)
}
```

### Querying the Presence History

```go
page, err := channel.Presence.History(nil)
for ; err == nil; page, err = page.Next() {
	for _, presence := range page.PresenceMessages() {
		fmt.Println(presence)
	}
}
if err != nil {
	panic(err)
}
```

### Generate Token and Token Request

```go
client.Auth.RequestToken()
client.Auth.CreateTokenRequest()
```

### Fetching your application's stats

```go
page, err := client.Stats()
for ; err == nil; page, err = page.Next() {
	for _, stat := range page.Stats() {
		fmt.Println(stat)
	}
}
if err != nil {
	panic(err)
}
```

## Known limitations (work in progress)

As the library is actively developed couple of features are not there yet:

- REST client does not use token authentication
- Realtime connection recovery is not implemented
- Realtime connection failures handling is not implemented
- Realtime Ping function is missing

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
