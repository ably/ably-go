# Upgrade / Migration Guide

## Version 1.4.x to 2.0

### The module is now `github.com/ably/ably-pubsub-go`

v2 ships under a new module path. Add the entry point your application uses
(see the next section) and drop the old requirement:

```bash
go get github.com/ably/ably-pubsub-go/server/pubsub   # or .../device/pubsub
go mod edit -droprequire github.com/ably/ably-go
```

`go mod edit -droprequire` does not rewrite your imports. Replace each import of
`github.com/ably/ably-go/ably` with one of the two packages below, and change the
`ably.` qualifiers to `pubsub.`.

### The `ably` package is replaced by two entry points

`github.com/ably/ably-go/ably` no longer exists. The implementation is internal
to the module, and an application imports one of two packages instead,
depending on where it runs:

| Import path | Use it for |
| --- | --- |
| `github.com/ably/ably-pubsub-go/server/pubsub` | Trusted environments that authenticate with an API key. Connections are exempt from monthly-active-user counting. Offers `HTTPClient` and `RealtimeClient`. |
| `github.com/ably/ably-pubsub-go/device/pubsub` | Applications on end-user devices, identified by a `clientId` and counted on accounts with monthly-active-user billing. Offers `Client`. |

Both packages are named `pubsub`, so only the import path differs between them.
Each one re-exports the whole API it needs — channels, messages, presence,
options, errors — so an application imports one of them and nothing else,
unless it uses LiveObjects: the LiveObjects types are in the shared
`github.com/ably/ably-pubsub-go/objects` package (see the list of differences
below). The re-exports are type aliases, so the types are the same across both
packages: the `Message` of one is the `Message` of the other.

```go
// Before
import "github.com/ably/ably-go/ably"

client, err := ably.NewRealtime(ably.WithKey(key))
channel := client.Channels.Get("some-channel")
channel.SubscribeAll(ctx, func(msg *ably.Message) { ... })

// After, on a server
import "github.com/ably/ably-pubsub-go/server/pubsub"

client, err := pubsub.NewRealtimeClient(pubsub.WithKey(key))
channel := client.Channels.Get("some-channel")
channel.SubscribeAll(ctx, func(msg *pubsub.Message) { ... })

// After, on a device
import "github.com/ably/ably-pubsub-go/device/pubsub"

client, err := pubsub.NewClient(
	pubsub.WithAuthURL("https://your-server.example.com/ably/token"),
	pubsub.WithClientID("user-123"),
)
```

Never embed an API key in an application that runs on an end-user device:
authenticate it with tokens issued by your own server, as above.

Differences to be aware of:

- The `device` entry point omits the part of the API that only an HTTP client
  can reach:
  - `HTTPClient` and `NewHTTPClient`, `HTTPChannels`, `HTTPChannel`,
    `HTTPPresence`, `HTTPRequest`, `HTTPPaginatedItems` and
    `HTTPPaginatedResponse`;
  - the `Request` options (`RequestOption`, `RequestWith…`);
  - the presence `Get` and presence history options (`GetPresence…`,
    `PresenceHistory…`), `PresenceRequest` and the presence paginated results;
  - the channel status types (`ChannelDetails`, `ChannelStatus`,
    `ChannelOccupancy`, `ChannelMetrics`);
  - the HTTP-only publish options (`PublishWithParams`,
    `PublishWithConnectionKey` and related).

  A device `Client` still has `Stats` and `Time`, and the HTTP options that
  apply to the requests it makes (`WithHTTPClient`, `WithHTTPRequestTimeout`,
  `WithIdempotentHTTPPublishing`).
- The old `ably/objects` package moved, unchanged, to
  `github.com/ably/ably-pubsub-go/objects`, which both entry points share.
  Only the import path changes: `objects.Plugin` and
  `objects.Operation_MapSet` keep their names. `WithExperimentalObjectsPlugin`
  stays in `pubsub`.
- `ably.WithDial` is not re-exported. Its signature names an unexported type,
  so no caller outside the implementation package could ever have supplied a
  usable argument to it.

The re-exports (`device/pubsub/api.go`, `server/pubsub/api.go`) are written by
hand. `server/pubsub/reexport_test.go` fails when `internal/ably` exports a
name that an entry point neither re-exports nor lists as deliberately left out.

### The `REST` prefix is now `HTTP`

The `REST` prefix has been removed from the public API. Every exported identifier that
named the stateless, HTTP-based client now uses an `HTTP` prefix instead, so that the
Go type names describe the transport rather than the Ably REST API they talk to.

These are **breaking changes**: the old names have been removed outright rather than kept
as deprecated aliases. The new names are exported from
`github.com/ably/ably-pubsub-go/server/pubsub`.

#### Renamed types

| Before | After |
| --- | --- |
| `ably.REST` | `pubsub.HTTPClient` |
| `ably.RESTChannels` | `pubsub.HTTPChannels` |
| `ably.RESTChannel` | `pubsub.HTTPChannel` |
| `ably.RESTPresence` | `pubsub.HTTPPresence` |
| `ably.RESTRequest` | `pubsub.HTTPRequest` |
| `ably.RESTPaginatedItems` | `pubsub.HTTPPaginatedItems` |

#### Renamed functions

| Before | After |
| --- | --- |
| `ably.NewREST` | `pubsub.NewHTTPClient` |
| `ably.WithIdempotentRESTPublishing` | `pubsub.WithIdempotentHTTPPublishing` |

#### Example

```go
// Before
import "github.com/ably/ably-go/ably"

client, err := ably.NewREST(ably.WithKey(key))
var channel *ably.RESTChannel = client.Channels.Get("some-channel")

// After
import "github.com/ably/ably-pubsub-go/server/pubsub"

client, err := pubsub.NewHTTPClient(pubsub.WithKey(key))
var channel *pubsub.HTTPChannel = client.Channels.Get("some-channel")
```

### `Channels.Release` no longer detaches the channel

`Channels.Release` no longer takes a `context.Context`, and it no longer
detaches the channel for you (RTS4c-e). A channel can be released only in the
`INITIALIZED`, `DETACHED` or `FAILED` state. In any other state, `Release`
returns an `ErrorInfo` with code `90011` (status code 400) and leaves the
channel as it is. Detach the channel and wait for that to finish first:

```go
// Before
err := client.Channels.Release(ctx, "some-channel")

// After
if err := client.Channels.Get("some-channel").Detach(ctx); err != nil {
	return err
}
err := client.Channels.Release("some-channel")
```

### Requests identify as `ably-pubsub-go`

The `Ably-Agent` header now starts with `ably-pubsub-go/<version>` instead of
`ably-go/<version>`, and also carries `ably-pubsub-server` or
`ably-pubsub-device`, depending on the entry point the client was created
from. Update anything that matches on `ably-go/`, such as proxy rules or log
queries.

`WithAgents`, which is meant for Ably-authored SDKs built on this one, now adds
its entries to the agents already configured instead of replacing them. A
repeated product name overrides the earlier version, the `ably-pubsub-server`
or `ably-pubsub-device` entry cannot be removed, and agents appear in
alphabetical order.

### Smaller changes

- Log messages from the HTTP client are prefixed `HTTPClient:` instead of
  `RestClient:`.
- The examples in `examples/rest` moved to `examples/http`.

### Host options are replaced by `WithEndpoint`

`ably.WithEnvironment`, `ably.WithRESTHost`, `ably.WithRealtimeHost` and
`ably.WithFallbackHostsUseDefault` have been removed. `WithEndpoint` selects the host for both HTTP
requests and the realtime connection, so code that still uses the old options fails to compile
instead of silently connecting to a different host.

| Before | After |
| --- | --- |
| `ably.WithEnvironment("sandbox")` | `pubsub.WithEndpoint("nonprod:sandbox")` |
| `ably.WithEnvironment("acme")` (a routing policy name) | `pubsub.WithEndpoint("acme")` |
| `ably.WithRESTHost(h)` and `ably.WithRealtimeHost(h)` set to the same host `h` | `pubsub.WithEndpoint(h)` |
| `ably.WithRESTHost("localhost:8080")` (TLS, the default) | `pubsub.WithEndpoint("localhost")` and `pubsub.WithTLSPort(8080)` |
| `ably.WithRESTHost("localhost:8080")` with `ably.WithTLS(false)` | `pubsub.WithEndpoint("localhost")`, `pubsub.WithTLS(false)` and `pubsub.WithPort(8080)` |
| `ably.WithRESTHost(a)` and `ably.WithRealtimeHost(b)` with different hosts | Not supported. Use one host that serves both, or contact Ably. |
| `ably.WithFallbackHostsUseDefault(true)` | Remove it. Default fallback hosts apply automatically, unless the endpoint is a hostname (next row). |
| `ably.WithRESTHost(h)` with `ably.WithFallbackHostsUseDefault(true)` | `pubsub.WithEndpoint(h)` and `pubsub.WithFallbackHosts(...)` listing the default fallback hosts `main.a.fallback.ably-realtime.com` … `main.e.fallback.ably-realtime.com`. A hostname endpoint has no default fallback hosts, so they must now be passed explicitly. |
| `ably.WithFallbackHosts(hosts)` | `pubsub.WithFallbackHosts(hosts)`, unchanged |

How the endpoint is resolved:

- Unset: the primary host is `main.realtime.ably.net`, with the default fallback hosts
  `main.[a-e].fallback.ably-realtime.com`.
- A routing policy name such as `acme`: `acme.realtime.ably.net`, with fallback hosts
  `acme.[a-e].fallback.ably-realtime.com`.
- `nonprod:` plus a name, such as `nonprod:sandbox`: `sandbox.realtime.ably-nonprod.net`, with
  fallback hosts `sandbox.[a-e].fallback.ably-realtime-nonprod.com`.
- A hostname, meaning a value that contains `.` or `::`, or is `localhost`: used as given, with no
  default fallback hosts. Do not include a port in the value; use `WithTLSPort`, or `WithPort` when
  TLS is disabled with `WithTLS(false)`.

An explicit `WithFallbackHosts` always replaces the default fallback hosts. A custom `WithPort` or
`WithTLSPort` no longer disables the default fallback hosts.

```go
// Before
client, err := ably.NewRealtime(
	ably.WithKey(key),
	ably.WithEnvironment("sandbox"),
)

// After
client, err := pubsub.NewRealtimeClient(
	pubsub.WithKey(key),
	pubsub.WithEndpoint("nonprod:sandbox"),
)
```

If you used any of the removed options, check your firewall or proxy rules before upgrading. The
client now connects to:

- production endpoints: `<name>.realtime.ably.net` and `<name>.[a-e].fallback.ably-realtime.com`,
  so allow `*.realtime.ably.net` and `*.fallback.ably-realtime.com`;
- non-production endpoints (`nonprod:<name>`, such as `nonprod:sandbox`):
  `<name>.realtime.ably-nonprod.net` and `<name>.[a-e].fallback.ably-realtime-nonprod.com`, so allow
  `*.realtime.ably-nonprod.net` and `*.fallback.ably-realtime-nonprod.com`.

The default hosts are unchanged for clients that set no host option.

### Deprecated API removed

Deprecated symbols that had a replacement have been removed.

| Removed | Use instead |
| --- | --- |
| `ably.WithEnvironment` | `pubsub.WithEndpoint` |
| `ably.WithRESTHost` | `pubsub.WithEndpoint` |
| `ably.WithRealtimeHost` | `pubsub.WithEndpoint` |
| `ably.WithFallbackHostsUseDefault` | Remove it. Default fallback hosts apply automatically, except with a hostname endpoint, where they must be passed with `pubsub.WithFallbackHosts`. |
| `ably.Connection.RecoveryKey()` | `pubsub.Connection.CreateRecoveryKey()` |
| `ably.RESTChannel.PublishMultipleWithOptions` | `pubsub.HTTPChannel.PublishMultiple` |
| `ably.PublishMultipleWithParams` | `pubsub.PublishWithParams` |

## Version 1.1.5 to 1.2.0

We have made many **breaking changes** in the version 1.2 release of this SDK.

In this guide we aim to highlight the main differences you will encounter when migrating your code from the interfaces we were offering prior to the [version 1.2.0 release](https://github.com/ably/ably-go/releases/tag/v1.2.0).

These include:

- Changes to numerous function and method signatures - these are breaking changes, in that your code will need to change to use them in their new form
- Adoption of the [Context Concurrency Pattern](https://blog.golang.org/context) using the [context package](https://pkg.go.dev/context) - e.g. [ably.RealtimeChannel.Subscribe](https://pkg.go.dev/github.com/ably/ably-go/ably#RealtimeChannel.Subscribe)
- Adoption of the [Functional Options Pattern](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis) - e.g. [ably.ClientOption](https://pkg.go.dev/github.com/ably/ably-go/ably#ClientOption)

### Asynchronous Operations and the `Context` Concurrency Pattern

We've now moved to a model where methods are blocking in nature, cancellable with a [Context](https://pkg.go.dev/context#Context).

We use `ctx` in examples to refer to a `Context` instance that you create in your code.

For robust, production-ready applications you will rarely (actually, probably _never_) want to create your `Context` using the basic [Background](https://pkg.go.dev/context#Background) function, because it cannot be cancelled and remains for the lifecycle of the program. Instead, you should use [WithTimeout](https://pkg.go.dev/context#WithTimeout) or [WithDeadline](https://pkg.go.dev/context#WithDeadline).

For example, a context can be created with a 10-second timeout like this:

```go
context.WithTimeout(context.Background(), 10 * time.Second)
```

Adding the necessary code to stay on top of cancellation, you will need something like this:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
defer cancel()
```

This way, the context can be cancelled at the close of the function.

**Note**:
For Realtime operations, cancellation isn't supported by the underlying protocol.
A context cancellation in this case just means that the call returns immediately, without waiting for the operation to finish, with the error provided by the context; but the operation carries on in the background.
Timeouts for these operations are handled separately, with [a configurable realtime request timeout duration](https://pkg.go.dev/github.com/ably/ably-go/ably#WithRealtimeRequestTimeout).

### Client Options now has a Functional Interface

Before version 1.2.0, you instantiated a client using a `ClientOptions` instance created with the `NewClientOptions` function:

```go
client, err := ably.NewRealtime(ably.NewClientOptions("xxx:xxx"))
```

**Starting with version 1.2.0**, you must use the new functional interface:

```go
client, err := ably.NewRealtime(ably.WithKey("xxx:xxx"))
```

For a full list of client options, see all functions prefixed by `With` that return a `ClientOption` function
[in the API reference](https://pkg.go.dev/github.com/ably/ably-go/ably#ClientOption).

### Subscription now uses Message Handlers

Before version 1.2.0, you subscribed to receive all messages from a Go channel like this:

```go
sub, _ := channel.Subscribe()
for msg := range sub.MessageChannel() {
    fmt.Println("Received message: ", msg)
}
```

**Starting with version 1.2.0**, you must supply a context and your own message handler function to the new [SubscribeAll](https://pkg.go.dev/github.com/ably/ably-go/ably#RealtimeChannel.SubscribeAll) method:

```go
unsubscribe, _ := channel.SubscribeAll(ctx, func(msg *ably.Message) {
    fmt.Println("Received message: ", msg)
})
```

The signature of the [Subscribe](https://pkg.go.dev/github.com/ably/ably-go/ably#RealtimeChannel.Subscribe) method has also changed. It now requires a [Context](https://pkg.go.dev/context#Context) as well as the channel name and your message handler.

Both `Subscribe` and `SubscribeAll` are now blocking methods.

Detail around the concurrency and routing of calls to message and event handlers is described in the package documentation
under [Event Emitters](https://pkg.go.dev/github.com/ably/ably-go/ably#hdr-Event_Emitters).

### The `Publish` Method now Blocks

Before version 1.2.0, you published messages to a channel by calling the `Publish` method and then waiting for the `Result`:

```go
result, _ := channel.Publish("EventName1", "EventData1")

// block until the publish operation completes
result.Wait()
```

**Starting with version 1.2.0**, you must supply a context, as this method is now blocking:

```go
// block until the publish operation completes or is cancelled
err := channel.Publish(ctx, "EventName1", "EventData1")
```

### Querying History

Before version 1.2.0, you queried history as shown below, receiving a `PaginatedResult` from the `History` function:

```go
page, err := channel.History(nil)
for ; err == nil; page, err = page.Next() {
    for _, message := range page.Messages() {
        fmt.Println("Message from History: ", message)
    }
}
if err != nil {
    panic(err)
}
```

**Starting with version 1.2.0**, you must first call `History`, which can be passed [functional options](https://pkg.go.dev/github.com/ably/ably-go/ably#HistoryOption), and then use the [Pages](https://pkg.go.dev/github.com/ably/ably-go/ably#HistoryRequest.Pages) (or [Items](https://pkg.go.dev/github.com/ably/ably-go/ably#HistoryRequest.Items), if you prefer) method on the returned `HistoryRequest` instance

```go
pages, err := channel.History().Pages(ctx)
if err != nil {
    panic(err)
}
for pages.Next(ctx) {
    for _, message := range pages.Items() {
        fmt.Println("Message from History: ", message)
    }
}
if err := pages.Err(); err != nil {
    panic(err)
}
```
