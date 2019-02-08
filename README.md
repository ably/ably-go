## [Ably Go](https://www.ably.io)

A Go client library for [www.ably.io](https://ably.io), the realtime messaging service.

## Installation

```bash
~ $ go get -u github.com/ably/ably-go/ably
```

## Feature support

This library implements the Ably REST and Realtime client APIs.

### REST API

In respect of the Ably REST API, this library targets the Ably 1.1 client library specification,
with some omissions as follows (see [the client library specification](https://docs.ably.io/client-lib-development-guide/features/) for specification references):

| Feature | Spec reference |
| --- | --- |
| Push notifications admin API | RSH1 |
| Push notifications target API | RSH2 |
| JWT authentication | multiple |
| Exception reporting | RSC20 |

It is intended that this library is upgraded incrementally, with 1.1 feature support expanded in successive minor
releases. If there are features that are currently missing that are a high priority for your use-case then please
[contact Ably customer support](https://support.ably.io). Pull Requests are also welcomed.

### Realtime API

In respect of the Realtime API, this is an early experimental implementation that targets the (now superseded) 0.8
library specification. This means that there are significant shortfalls in functionality; the principal issues are:

- there is no channel `suspended` state; this means that the client will not automatically reattach to channels if a
connection becomes `suspended` and then resumes, and presence members associated with the client will not be
automatically re-entered;

- transient realtime publishing is not supported, so a call to `publish()` on a realtime channel will trigger attachment
 of the channel;

- inband reauthentication is not supported; expiring tokens will trigger a disconnection and resume of a realtime
connection.

As with the REST API, it is intended that this library is upgraded incrementally and brought into line with the 1.1
specification. If there are features that are currently missing that are a high priority for your use-case then please
[contact Ably customer support](https://support.ably.io). Pull Requests are also welcomed.

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

channel := client.Channels.Get("test", nil)
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

## Release process

This library uses [semantic versioning](http://semver.org/). For each release, the following needs to be done:

* Create a branch for the release, named like `release-1.1.2`
* Replace all references of the current version number with the new version number and commit the changes
* Run [`github_changelog_generator`](https://github.com/skywinder/Github-Changelog-Generator) to update the [CHANGELOG](./CHANGELOG.md): `github_changelog_generator -u ably -p ably-go --header-label="# Changelog" --release-branch=release-1.1.2 --future-release=v1.1.2` 
* Commit [CHANGELOG](./CHANGELOG.md)
* Add a tag and push to origin such as `git tag v1.1.2; git push origin v1.1.2`
* Make a PR against `develop`
* Once the PR is approved, merge it into `develop`
* Fast-forward the master branch: `git checkout master && git merge --ff-only develop && git push origin master`

## Support and feedback

Please visit http://support.ably.io/ for access to our knowledgebase and to ask for any assistance.

You can also view the [community reported Github issues](https://github.com/ably/ably-go/issues).

## Contributing

Because this package uses `internal` packages, all fork development has to happen under `$GOPATH/src/github.com/ably/ably-go` to prevent `use of internal package not allowed` errors.

1. Fork `github.com/ably/ably-go`
2. go to the `ably-go` directory: `cd $GOPATH/src/github.com/ably/ably-go`
3. add your fork as a remote: `git remote add fork git@github.com:your-username/ably-go`
4. create your feature branch: `git checkout -b my-new-feature`
5. commit your changes (`git commit -am 'Add some feature'`)
6. ensure you have added suitable tests and the test suite is passing: `make test`
7. push to the branch: `git push fork my-new-feature`
8. create a new Pull Request

## License

Copyright (c) 2016-2019 Ably Real-time Ltd, Licensed under the Apache License, Version 2.0.  Refer to [LICENSE](LICENSE) for the license terms.
