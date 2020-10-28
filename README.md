## [Ably Go](https://www.ably.io)

A Go client library for [www.ably.io](https://ably.io), the realtime messaging service.

## Installation

```bash
~ $ go get -u github.com/ably/ably-go/ably
```

## Feature support

This library implements the Ably REST and Realtime client APIs.

### REST API

In respect of the Ably REST API, this library targets the Ably 1.2 client library specification,
with some omissions as follows (see [the client library specification](https://www.ably.io/documentation/client-lib-development-guide/features) for specification references):

| Feature |
| --- |
| [Push notifications admin API](https://www.ably.io/documentation/general/push/admin) |
| [JWT authentication](https://www.ably.io/documentation/core-features/authentication#ably-jwt-process) |

It is intended that this library is upgraded incrementally, with 1.1 feature support expanded in successive minor
releases. If there are features that are currently missing that are a high priority for your use-case then please
[contact Ably customer support](https://support.ably.io). Pull Requests are also welcomed.

### Realtime API

- there is no channel `suspended` state; this means that the client will not automatically reattach to channels if a
connection becomes `suspended` and then resumes, and presence members associated with the client will not be
automatically re-entered;

- transient realtime publishing is not supported, so a call to `publish()` on a realtime channel will trigger attachment
 of the channel;

- inband reauthentication is not supported; expiring tokens will trigger a disconnection and resume of a realtime
connection.

As with the REST API, it is intended that this library is upgraded incrementally and brought into line with the 1.2
specification. If there are features that are currently missing that are a high priority for your use-case then please
[contact Ably customer support](https://support.ably.io). Pull Requests are also welcomed.

## Using the Realtime API

### Creating a client

<!-- GO EXAMPLE
import "fmt"

ctx := context.Background()
-->

```go
client, err := ably.NewRealtime(ably.WithKey("xxx:xxx"))
if err != nil {
	panic(err)
}

channel := client.Channels.Get("test")
```

### Subscribing to a channel for all messages

```go
unsubscribe, err := channel.SubscribeAll(ctx, func(msg *ably.Message) {
	fmt.Println("Received message:", msg)
})
if err != nil {
	panic(err)
}
```

<!-- GO EXAMPLE
unsubscribe()
-->

### Subscribing to a channel for `EventName1` and `EventName2` message names

```go
unsubscribe1, err := channel.Subscribe(ctx, "EventName1", func(msg *ably.Message) {
	fmt.Println("Received message:", msg)
})
if err != nil {
	panic(err)
}

unsubscribe2, err := channel.Subscribe(ctx, "EventName2", func(msg *ably.Message) {
	fmt.Println("Received message:", msg)
})
if err != nil {
	panic(err)
}
```

<!-- GO EXAMPLE
unsubscribe1()
unsubscribe2()
-->

### Publishing to a channel

```go
err = channel.Publish(ctx, "EventName1", "EventData1")
if err != nil {
	panic(err)
}
```

### Announcing presence on a channel

```go
err = channel.Presence.Enter(ctx, "presence data")
if err != nil {
	panic(err)
}
```

### Announcing presence on a channel on behalf of other client

```go
err = channel.Presence.EnterClient(ctx, "clientID", "presence data")
if err != nil {
	panic(err)
}
```

### Getting all clients present on a channel

```go
clients, err := channel.Presence.Get(ctx)
if err != nil {
	panic(err)
}

for _, client := range clients {
	fmt.Println("Present client:", client)
}
```

### Subscribing to all presence messages

```go
unsubscribe, err = channel.Presence.SubscribeAll(ctx, func(msg *ably.PresenceMessage) {
	fmt.Println("Presence event:", msg)
})
if err != nil {
	panic(err)
}
```

<!-- GO EXAMPLE
unsubscribe()
-->

### Subscribing to 'Enter' presence messages only

```go
unsubscribe, err = channel.Presence.Subscribe(ctx, ably.PresenceActionEnter, func(msg *ably.PresenceMessage) {
	fmt.Println("Presence event:", msg)
})
if err != nil {
	panic(err)
}
```

<!-- GO EXAMPLE
unsubscribe()
-->

## Using the REST API

### Introduction

All examples assume a client and/or channel has been created as follows:

```go
client, err := ably.NewREST(ably.WithKey("xxx:xxx"))
if err != nil {
	panic(err)
}

channel := client.Channels.Get("test")
```

### Publishing a message to a channel

```go
err = channel.Publish(ctx, "HelloEvent", "Hello!")
if err != nil {
	panic(err)
}
```

### Querying the History

```go
page, err := channel.History(nil)
for ; err == nil && page != nil; page, err = page.Next() {
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
page, err = channel.Presence.Get(nil)
for ; err == nil && page != nil; page, err = page.Next() {
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
page, err = channel.Presence.History(nil)
for ; err == nil && page != nil; page, err = page.Next() {
	for _, presence := range page.PresenceMessages() {
		fmt.Println(presence)
	}
}
if err != nil {
	panic(err)
}
```

### Fetching your application's stats

```go
page, err = client.Stats(&ably.PaginateParams{})
for ; err == nil && page != nil; page, err = page.Next() {
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

- Realtime connection failure handling is partly implemented
- Realtime Ping function is not implemented

## Release process

Starting with release 1.2, this library uses [semantic versioning](http://semver.org/). For each release, the following needs to be done:

* Create a branch for the release, named like `release/1.1.6`
* Replace all references of the current version number with the new version number and commit the changes
* Run [`github_changelog_generator`](https://github.com/github-changelog-generator/github-changelog-generator) to automate the update of the [CHANGELOG](./CHANGELOG.md). This may require some manual intervention, both in terms of how the command is run and how the change log file is modified. Your mileage may vary:
    * The command you will need to run will look something like this: `github_changelog_generator -u ably -p ably-go --since-tag v1.1.4 --output delta.md`
    * Using the command above, `--output delta.md` writes changes made after `--since-tag` to a new file
    * The contents of that new file (`delta.md`) then need to be manually inserted at the top of the `CHANGELOG.md`, changing the "Unreleased" heading and linking with the current version numbers
    * Also ensure that the "Full Changelog" link points to the new version tag instead of the `HEAD`
    * Commit this change: `git add CHANGELOG.md && git commit -m "Update change log."`
* Commit [CHANGELOG](./CHANGELOG.md)
* Make a PR against `main`
* Once the PR is approved, merge it into `main`
* Add a tag to the new `main` head commit and push to origin such as `git tag v1.1.6 && git push origin v1.1.6`

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
