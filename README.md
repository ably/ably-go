## [Ably Go](https://www.ably.io)

A Go client library for [www.ably.io](https://ably.io), the realtime messaging service.

## Installation

```bash
~ $ go get -u github.com/ably/ably-go/ably
```

## Using the Realtime API

### Creating a client

```go
client, err := ably.NewRealtimeClient(ably.NewClientOptions("xxx:xxx"))
if err != nil {
	panic(err)
}

channel := client.Channels.Get("test")
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
clients, err := channel.Presence.Get(true)
if err != nil {
	panic(err)
}

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
client, err := ably.NewRestClient(ably.NewClientOptions("xxx:xxx"))
if err != nil {
	panic(err)
}

channel := client.Channel("test")
```

### Publishing a message to a channel

```go
err = channel.Publish("HelloEvent", "Hello!")
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
page, err := client.Stats(&ably.PaginateParams{})
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

- Realtime connection recovery is not implemented
- Realtime connection failure handling is not implemented
- ChannelsOptions and CipherParams are not supported when creating a Channel
- Realtime Ping function is not implemented

## Support and feedback

Please visit http://support.ably.io/ for access to our knowledgebase and to ask for any assistance.

You can also view the [community reported Github issues](https://github.com/ably/ably-go/issues).

## Contributing

1. Fork it
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Ensure you have added suitable tests and the test suite is passing (`make test`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create a new Pull Request

## License

Copyright (c) 2016 Ably Real-time Ltd, Licensed under the Apache License, Version 2.0.  Refer to [LICENSE](LICENSE) for the license terms.
