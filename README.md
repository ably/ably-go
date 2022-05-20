## [Ably Go](https://ably.com/)

[![.github/workflows/check.yml](https://github.com/ably/ably-go/actions/workflows/check.yml/badge.svg)](https://github.com/ably/ably-go/actions/workflows/check.yml)

[![Go Reference](https://pkg.go.dev/badge/github.com/ably/ably-go/ably.svg)](https://pkg.go.dev/github.com/ably/ably-go/ably)

_[Ably](https://ably.com) is the platform that powers synchronized digital experiences in realtime. Whether attending an event in a virtual venue, receiving realtime financial information, or monitoring live car performance data – consumers simply expect realtime digital experiences as standard. Ably provides a suite of APIs to build, extend, and deliver powerful digital experiences in realtime for more than 250 million devices across 80 countries each month. Organizations like Bloomberg, HubSpot, Verizon, and Hopin depend on Ably’s platform to offload the growing complexity of business-critical realtime data synchronization at global scale. For more information, see the [Ably documentation](https://ably.com/documentation)._

This is a Go client library for Ably.

## Installation

```bash
~ $ go get -u github.com/ably/ably-go/ably
```

## Supported Versions of Go

Whenever a new version of Go is released, Ably adds support for that version. The [Go Release Policy](https://golang.org/doc/devel/release#policy) supports the last two major versions. This SDK follows the same policy of supporting the last two major versions of Go.

## Breaking API Changes in Version 1.2.x

Please see our [Upgrade / Migration Guide](UPDATING.md) for notes on changes you need to make to your code to update it to use the new API introduced by version 1.2.x.

Users updating from version 1.1.5 of this library will note that there are significant breaking changes to the API.
Our [current approach to versioning](https://ably.com/documentation/client-lib-development-guide/versioning) is not compliant with semantic versioning, which is why these changes are breaking despite presenting only a change in the `minor` component of the version number.

## Using the Realtime API

### Creating a client

```go
client, err := ably.NewRealtime(ably.WithKey("xxx:xxx"))
if err != nil {
        panic(err)
}
channel := client.Channels.Get("test")
```

### Subscribing to events

You may monitor events on connections and channels.

```go
client, err = ably.NewRealtime(
ably.WithKey("xxx:xxx"),
ably.WithAutoConnect(false), // Set this option to avoid missing state changes.
)
if err != nil {
        panic(err)
}
// Set up connection events handler.
client.Connection.OnAll(func(change ably.ConnectionStateChange) {
        fmt.Printf("Connection event: %s state=%s reason=%s", change.Event, change.Current, change.Reason)
})
// Then connect.
client.Connect()
channel = client.Channels.Get("test")
channel.OnAll(func(change ably.ChannelStateChange) {
        fmt.Printf("Channel event event: %s channel=%s state=%s reason=%s", channel.Name, change.Event, change.Current, change.Reason)
})
```

### Subscribing to a channel for all messages

```go
unsubscribe, err := channel.SubscribeAll(ctx, func(msg *ably.Message) {
        fmt.Printf("Received message: name=%s data=%v\n", msg.Name, msg.Data)
})
if err != nil {
        panic(err)
}
```

### Subscribing to a channel for `EventName1` and `EventName2` message names

```go
unsubscribe1, err := channel.Subscribe(ctx, "EventName1", func(msg *ably.Message) {
        fmt.Printf("Received message: name=%s data=%v\n", msg.Name, msg.Data)
})
if err != nil {
        panic(err)
}
unsubscribe2, err := channel.Subscribe(ctx, "EventName2", func(msg *ably.Message) {
        fmt.Printf("Received message: name=%s data=%v\n", msg.Name, msg.Data)
})
if err != nil {
        panic(err)
}
```

### Publishing to a channel

```go
err = channel.Publish(ctx, "EventName1", "EventData1")
if err != nil {
        panic(err)
}
```

### Handling errors

Errors returned by this library may have an underlying `*ErrorInfo` type.

[See Ably documentation for ErrorInfo.](https://ably.com/docs/realtime/types#error-info)

```go
badClient, err := ably.NewRealtime(ably.WithKey("invalid:key"))
if err != nil {
        panic(err)
}
err = badClient.Channels.Get("test").Publish(ctx, "event", "data")
if errInfo := (*ably.ErrorInfo)(nil); errors.As(err, &errInfo) {
        fmt.Printf("Error publishing message: code=%v status=%v cause=%v", errInfo.Code, errInfo.StatusCode, errInfo.Cause)
} else if err != nil {
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

### Updating and leaving presence

```go
// Update also has an UpdateClient variant.
err = channel.Presence.Update(ctx, "new presence data")
if err != nil {
        panic(err)
}
// Leave also has an LeaveClient variant.
err = channel.Presence.Leave(ctx, "last presence data")
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
        fmt.Printf("Presence event: action=%v data=%v", msg.Action, msg.Data)
})
if err != nil {
        panic(err)
}
```

### Subscribing to 'Enter' presence messages only

```go
unsubscribe, err = channel.Presence.Subscribe(ctx, ably.PresenceActionEnter, func(msg *ably.PresenceMessage) {
        fmt.Printf("Presence event: action=%v data=%v", msg.Action, msg.Data)
})
if err != nil {
        panic(err)
}
```

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
// You can also publish multiple messages in a single request.
err = channel.PublishMultiple(ctx, []*ably.Message{
        {Name: "HelloEvent", Data: "Hello!"},
        {Name: "ByeEvent", Data: "Bye!"},
})
if err != nil {
        panic(err)
}
// A REST client can publish messages on behalf of another client
// by providing the connection key of that client.
err := channel.Publish(ctx, "temperature", "12.7", ably.PublishWithConnectionKey("connectionKeyOfAnotherClient"))
if err != nil {
        panic(err)
}
```

### Querying the History

```go
pages, err := channel.History().Pages(ctx)
if err != nil {
        panic(err)
}
for pages.Next(ctx) {
        for _, message := range pages.Items() {
                fmt.Println(message)
        }
}
if err := pages.Err(); err != nil {
        panic(err)
}
```

### Presence on a channel

```go
pages, err := channel.Presence.Get().Pages(ctx)
if err != nil {
        panic(err)
}
for pages.Next(ctx) {
        for _, presence := range pages.Items() {
                fmt.Println(presence)
        }
}
if err := pages.Err(); err != nil {
        panic(err)
}
```

### Querying the Presence History

```go
pages, err := channel.Presence.History().Pages(ctx)
if err != nil {
        panic(err)
}
for pages.Next(ctx) {
        for _, presence := range pages.Items() {
                fmt.Println(presence)
        }
}
if err := pages.Err(); err != nil {
        panic(err)
}
```

### Fetching your application's stats

```go
pages, err := client.Stats().Pages(ctx)
if err != nil {
        panic(err)
}
for pages.Next(ctx) {
        for _, stat := range pages.Items() {
                fmt.Println(stat)
        }
}
if err := pages.Err(); err != nil {
        panic(err)
}
```

### Getting the channel status
```go
status, err := channel.Status(ctx)
if err != nil {
        panic(err)
}
fmt.Print(status, status.ChannelId)
```

## Feature support

This library targets the Ably 1.2 [client library specification](https://ably.com/docs/client-lib-development-guide/features).

### Known limitations

As of release 1.2.0, the following are not implemented and will be covered in future 1.2.x releases. If there are features that are currently missing that are a high priority for your use-case then please [contact Ably customer support](https://ably.com/support). Pull Requests are also welcomed.

### REST API

- [Push notifications admin API](https://ably.com/docs/general/push/admin) is not implemented.

- [JWT authentication](https://ably.com/docs/core-features/authentication#ably-jwt-process) is not implemented.

### Realtime API

- There is no channel `suspended` state; this means that the client will not automatically reattach to channels if a
  connection becomes `suspended` and then resumes, and presence members associated with the client will not be
  automatically re-entered.

- Transient realtime publishing is not supported, so a call to `publish()` on a realtime channel will trigger attachment
  of the channel.

- Inband reauthentication is not supported; expiring tokens will trigger a disconnection and resume of a realtime
  connection.

- Realtime connection failure handling is partially implemented.

- Realtime Ping function is not implemented.

- Message Delta Compression is not implemented.

- Push Notification Target functional is not applicable for the SDK and thus not implemented.

### Tests

This project contains two types of test. Test which use the `ablytest` package and tests which dont. 

At each stage of the CI pipeline, test results are uploaded to a [test observability server](https://github.com/ably-labs/test-observability-server)

#### Unit tests

The tests which don't make use of the `ablytest` package are considered unit tests. These tests exist in files which are suffixed `_test.go`. They run in the CI pipeline at the step `Unit Tests`. They can be run locally with the command: 

```
go test -v -tags=unit ./...
``` 

When adding new unit tests, the following build tag must be included at the top of the file to exclude these tests from running in CI as part of the Integration test step. 

```
//go:build !integration
// +build !integration
```

#### Integration tests

The tests which use the package `ablytest` are considered integration tests. These tests take longer to run than the unit tests and are mostly run in a sandbox environment. They are dependent on the sandbox environment being available and will fail if the environment is experiencing issues. While effort has been made to turn off some of these tests which frequently experience random failures (flaky) some of these tests may still fail unexpectedly from time to time. 

Please note that these tests are not true integration tests as rather than using the public API, they rely on `export_test.go` to expose private functionality so that it can be tested. These tests exist in files which are suffixed `_integration_test.go`. They run twice in the CI pipeline at the steps `Integration Tests with JSON Protocol` and `Integration Tests with MessagePack Protocol`. To run these tests locally, they have a dependency on a git submodule being present, so it is necessary clone the project with:

```
git clone git@github.com:ably/ably-go.git --recurse-submodules
```
 
A bash script is used to run these tests with the commands: 
 
```
scripts/test.sh --protocol application/json
scripts/test.sh --protocol application/x-msgpack
``` 

Depending on which protocol they are to be run for. It is also necessary to clean the test cache in between runs of these tests which can be done with the command: 

```
go clean -testcache
``` 

When adding new integration tests, the following build tag must be included at the top of the file to exclude these tests for running in CI as part of the Unit test step.

```
//go:build !unit
// +build !unit
```

## Release process

Starting with release 1.2, this library uses [semantic versioning](http://semver.org/). For each release, the following needs to be done:

1. Create a branch for the release, named like `release/1.2.3` (where `1.2.3` is the new version number)
2. Replace all references of the current version number with the new version number and commit the changes
3. Run [`github_changelog_generator`](https://github.com/github-changelog-generator/github-changelog-generator) to automate the update of the [CHANGELOG](./CHANGELOG.md). This may require some manual intervention, both in terms of how the command is run and how the change log file is modified. Your mileage may vary:
  - The command you will need to run will look something like this: `github_changelog_generator -u ably -p ably-go --since-tag v1.2.3 --output delta.md`
  - Using the command above, `--output delta.md` writes changes made after `--since-tag` to a new file
  - The contents of that new file (`delta.md`) then need to be manually inserted at the top of the `CHANGELOG.md`, changing the "Unreleased" heading and linking with the current version numbers
  - Also ensure that the "Full Changelog" link points to the new version tag instead of the `HEAD`
4. Commit this change: `git add CHANGELOG.md && git commit -m "Update change log."`
5. Make a PR against `main`
6. Once the PR is approved, merge it into `main`
7. Add a tag to the new `main` head commit and push to origin such as `git tag v1.2.3 && git push origin v1.2.3`
8. Create the release on Github, from the new tag, including populating the release notes
9. Create the entry on the [Ably Changelog](https://changelog.ably.com/) (via [headwayapp](https://headwayapp.co/))

## Further information

Broaden your knowledge of realtime in Go with these useful materials:

* [Building realtime apps with Go and WebSockets: client-side considerations](https://ably.com/topic/websockets-golang)
* [Guide to Pub/Sub in Golang](https://ably.com/blog/pubsub-golang)

## Support and feedback

Please visit https://knowledge.ably.com/ for access to our knowledgebase and to ask for any assistance.

You can also view the [community reported Github issues](https://github.com/ably/ably-go/issues).

## Contributing

Because this package uses `internal` packages, all fork development has to happen under `$GOPATH/src/github.com/ably/ably-go` to prevent `use of internal package not allowed` errors.

1. Fork `github.com/ably/ably-go`
2. go to the `ably-go` directory: `cd $GOPATH/src/github.com/ably/ably-go`
3. add your fork as a remote: `git remote add fork git@github.com:your-username/ably-go`
4. create your feature branch: `git checkout -b my-new-feature`
5. commit your changes (`git commit -am 'Add some feature'`)
6. ensure you have added suitable tests and the test suite is passing for both JSON and MessagePack protocols (see [workflow](.github/workflows/check.yml) for the test commands executed in the CI environment).
7. push to the branch: `git push fork my-new-feature`
8. create a new Pull Request.
