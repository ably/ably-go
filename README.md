## [Ably Go](https://ably.com/)

[![.github/workflows/check.yml](https://github.com/ably/ably-go/actions/workflows/check.yml/badge.svg)](https://github.com/ably/ably-go/actions/workflows/check.yml)
[![.github/workflows/integration-test.yml](https://github.com/ably/ably-go/actions/workflows/integration-test.yml/badge.svg)](https://github.com/ably/ably-go/actions/workflows/integration-test.yml)
[![Features](https://github.com/ably/ably-go/actions/workflows/features.yml/badge.svg)](https://github.com/ably/ably-go/actions/workflows/features.yml)

[![Go Reference](https://pkg.go.dev/badge/github.com/ably/ably-go/ably.svg)](https://pkg.go.dev/github.com/ably/ably-go/ably)

_[Ably](https://ably.com) is the platform that powers synchronized digital experiences in realtime. Whether attending an event in a virtual venue, receiving realtime financial information, or monitoring live car performance data – consumers simply expect realtime digital experiences as standard. Ably provides a suite of APIs to build, extend, and deliver powerful digital experiences in realtime for more than 250 million devices across 80 countries each month. Organizations like Bloomberg, HubSpot, Verizon, and Hopin depend on Ably’s platform to offload the growing complexity of business-critical realtime data synchronization at global scale. For more information, see the [Ably documentation](https://ably.com/documentation)._

## Overview
This is a Go client library for Ably. [Supported features](#feature-support) and [known limitations](#known-limitations) are documented here.

## Installation

```bash
~ $ go get -u github.com/ably/ably-go/ably
```

See [Requirements](#requirements)

## Usage
### Using the Realtime API

#### Creating a client

```go
client, err := ably.NewRealtime(ably.WithKey("xxx:xxx"))
if err != nil {
        panic(err)
}
channel := client.Channels.Get("test")
```

#### Subscribing to events

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

#### Subscribing to a channel for all messages

```go
unsubscribe, err := channel.SubscribeAll(ctx, func(msg *ably.Message) {
        fmt.Printf("Received message: name=%s data=%v\n", msg.Name, msg.Data)
})
if err != nil {
        panic(err)
}
```

#### Subscribing to a channel for `EventName1` and `EventName2` message names

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

#### Publishing to a channel

```go
err = channel.Publish(ctx, "EventName1", "EventData1")
if err != nil {
        panic(err)
}
```

`Publish` will block until either the publish is acknowledged or failed to
deliver.

Alternatively you can use `PublishAsync` which does not block:

```go
channel.PublishAsync("EventName1", "EventData11", func(err error) {
	if err != nil {
		fmt.Println("failed to publish", err)
	} else {
		fmt.Println("publish ok")
	}
})
```

Note the `onAck` callback must not block as it would block the internal client.

#### Handling errors

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

#### Announcing presence on a channel

```go
err = channel.Presence.Enter(ctx, "presence data")
if err != nil {
        panic(err)
}
```

#### Announcing presence on a channel on behalf of other client

```go
err = channel.Presence.EnterClient(ctx, "clientID", "presence data")
if err != nil {
        panic(err)
}
```

#### Updating and leaving presence

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

#### Getting all clients present on a channel

```go
clients, err := channel.Presence.Get(ctx)
if err != nil {
        panic(err)
}
for _, client := range clients {
        fmt.Println("Present client:", client)
}
```

#### Subscribing to all presence messages

```go
unsubscribe, err = channel.Presence.SubscribeAll(ctx, func(msg *ably.PresenceMessage) {
        fmt.Printf("Presence event: action=%v data=%v", msg.Action, msg.Data)
})
if err != nil {
        panic(err)
}
```

#### Subscribing to 'Enter' presence messages only

```go
unsubscribe, err = channel.Presence.Subscribe(ctx, ably.PresenceActionEnter, func(msg *ably.PresenceMessage) {
        fmt.Printf("Presence event: action=%v data=%v", msg.Action, msg.Data)
})
if err != nil {
        panic(err)
}
```

#### Update MaxMessageSize/read limit for realtime message subscription
- The default `MaxMessageSize` is automatically configured by Ably when connection is established with Ably.
- This value defaults to [16kb for free and 64kb for PAYG account](https://faqs.ably.com/what-is-the-maximum-message-size), please [get in touch](https://ably.com/support) if you would like to request a higher limit for your account.
- Upgrading your account to higher limit will automatically update `MaxMessageSize` property and should accordingly set the client side connection read limit.
- If you are still facing issues when receiving large messages or intentionally want to reduce the limit, you can explicitly update the connection read limit:

```go
client, err = ably.NewRealtime(ably.WithKey("xxx:xxx"))
if err != nil {
        panic(err)
}
client.Connection.SetReadLimit(131072) // Set read limit to 128kb, overriding default ConnectionDetails.MaxMessageSize
```

Note - If connection read limit is less than size of received message, the client will throw an error "failed to read: read limited at {READ_LIMIT + 1} bytes" and will close the connection.

### Using the REST API

#### Introduction

All examples assume a client and/or channel has been created as follows:

```go
client, err := ably.NewREST(ably.WithKey("xxx:xxx"))
if err != nil {
        panic(err)
}
channel := client.Channels.Get("test")
```

#### Publishing a message to a channel

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

#### Querying the History

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

#### Presence on a channel

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

#### Querying the Presence History

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

#### Fetching your application's stats

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

#### Getting the channel status
```go
status, err := channel.Status(ctx)
if err != nil {
        panic(err)
}
fmt.Print(status, status.ChannelId)
```

### Configure logging
- By default, internal logger prints output to `stdout` with default logging level of `warning`.
- You need to create a custom Logger that implements `ably.Logger` interface.
- There is also an option provided to configure loglevel.

```go
type customLogger struct {
	*log.Logger
}

func (s *customLogger) Printf(level ably.LogLevel, format string, v ...interface{}) {
	s.Logger.Printf(fmt.Sprintf("[%s] %s", level, format), v...)
}

func NewCustomLogger() *customLogger {
	logger := &customLogger{}
	logger.Logger = log.New(os.Stdout, "", log.LstdFlags)
	return logger
}

client, err = ably.NewRealtime(
        ably.WithKey("xxx:xxx"),
        ably.WithLogHandler(NewCustomLogger()),
        ably.WithLogLevel(ably.LogWarning),
)
```

## Proxy support
The `ably-go` SDK doesn't provide a direct option to set a proxy in its configuration. However, you can use standard environment variables to set up a proxy for all your HTTP and HTTPS connections. The Go programming language will automatically handle these settings.

### Setting Up Proxy via Environment Variables

To configure the proxy, set the `HTTP_PROXY` and `HTTPS_PROXY` environment variables with the URL of your proxy server. Here's an example of how to set these variables:

```bash
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

- `proxy.example.com` is the domain or IP address of your proxy server.
- `8080` is the port number of your proxy server.

#### Considerations
- **Protocol:** Make sure to include the protocol (`http` or `https`) in the proxy URL.
- **Authentication:** If your proxy requires authentication, you can include the username and password in the URL. For example: `http://username:password@proxy.example.com:8080`.

After setting the environment variables, the `ably-go` SDK will route its traffic through the specified proxy for both Rest and Realtime clients.

For more details on environment variable configurations in Go, you can refer to the [official Go documentation on http.ProxyFromEnvironment](https://golang.org/pkg/net/http/#ProxyFromEnvironment).

### Setting Up Proxy via custom http client

For Rest client, you can also set proxy by providing custom http client option `ably.WithHTTPClient`:

```go
ably.WithHTTPClient(&http.Client{
        Transport: &http.Transport{
                Proxy:        proxy // custom proxy implementation
        },
})
```

**Important Note** - Connection reliability is totally dependent on health of proxy server and ably will not be responsible for errors introduced by proxy server. You may need to increase request timeouts in order to compensate for connection wait time period introduced by proxy server.
e.g use of `WithHTTPRequestTimeout`, `WithRealtimeRequestTimeout`.

## Note on usage of ablytest package
Although the `ablytest` package is available as a part of ably-go, we do not recommend using it as a sandbox for your own testing, since it's specifically intended for client library SDKs and we don’t provide any guarantees for support or that it will remain publicly accessible.
It can lead to unexpected behaviour, since some beta features may be deployed on the `sandbox` environment so that they can be tested before going into production.

You should rather use, `ably.NewRealtime` by passing the `ABLY_KEY`, which would be using the Ably production environment.

```
client, err := ably.NewRealtime(ably.WithKey("xxx:xxx"))
```

You can also use the [control api](https://ably.com/docs/control-api) to setup a test environment using https://github.com/ably/ably-control-go/.

## Resources

Demo repositories hosted at [ably-labs](https://github.com/ably-labs) which use `ably-go`.  

* [Ableye](https://github.com/ably-labs/Ableye)
* [AblyD](https://github.com/ably-labs/AblyD)
* [Ably Go Terminal Chat](https://github.com/ably-labs/go-chat)
* [Broadcasting messages in Go with Ably and Redis](https://github.com/ably-labs/broadcasting-redis-go)
* [Sync Edit](https://github.com/ably-labs/sync-edit)
* [Word Game](https://github.com/ably-labs/word-game)

Broaden your knowledge of realtime in Go with these useful materials:

* [Building realtime apps with Go and WebSockets: client-side considerations](https://ably.com/topic/websockets-golang)
* [Guide to Pub/Sub in Golang](https://ably.com/blog/pubsub-golang)

## Requirements
### Supported Versions of Go

Whenever a new version of Go is released, Ably adds support for that version. The [Go Release Policy](https://golang.org/doc/devel/release#policy) supports the last two major versions. This SDK follows the same policy of supporting the last two major versions of Go.

### Breaking API Changes in Version 1.2.x

Please see our [Upgrade / Migration Guide](UPDATING.md) for notes on changes you need to make to your code to update it to use the new API introduced by version 1.2.x.

Users updating from version 1.1.5 of this library will note that there are significant breaking changes to the API.
Our [current approach to versioning](https://ably.com/documentation/client-lib-development-guide/versioning) is not compliant with semantic versioning, which is why these changes are breaking despite presenting only a change in the `minor` component of the version number.

## Feature support

This library targets the Ably 1.2 [client library specification](https://ably.com/docs/client-lib-development-guide/features). List of available features for our client library SDKs can be found on our [feature support matrix](https://ably.com/download/sdk-feature-support-matrix) page.

## Known limitations

As of release 1.2.0, the following are not implemented and will be covered in future 1.2.x releases. If there are features that are currently missing that are a high priority for your use-case then please [contact Ably customer support](https://ably.com/support). Pull Requests are also welcomed.

### REST API

- [Push notifications admin API](https://ably.com/docs/api/rest-sdk/push-admin) is not implemented.

- [JWT authentication](https://ably.com/docs/auth/token?lang=javascript#jwt) using `auth-url` is not implemented.
See [jwt auth issue](https://github.com/ably/ably-go/issues/569) for more details.

### Realtime API

- Inband reauthentication is not supported; expiring tokens will trigger a disconnection and resume of a realtime
  connection. See [server initiated auth](https://github.com/ably/ably-go/issues/228) for more details.

- Channel suspended state is partially implemented. See [suspended channel state](https://github.com/ably/ably-go/issues/568).

- Realtime Ping function is not implemented.

- Message Delta Compression is not implemented.

- Push Notification Target functional is not applicable for the SDK and thus not implemented.

## Support, feedback and troubleshooting

Please visit https://faqs.ably.com/ for access to our knowledgebase. If you require support, please visit https://ably.com/support to submit a support ticket.

You can also view the [community reported Github issues](https://github.com/ably/ably-go/issues).

## Contributing
For guidance on how to contribute to this project, see [CONTRIBUTING.md](CONTRIBUTING.md).
