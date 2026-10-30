![Ably Pub/Sub Go Header](/image/goSDK-github.png)

[![Go Reference: server](https://pkg.go.dev/badge/github.com/ably/ably-pubsub-go/v2/server/pubsub.svg)](https://pkg.go.dev/github.com/ably/ably-pubsub-go/v2/server/pubsub)
[![Go Reference: device](https://pkg.go.dev/badge/github.com/ably/ably-pubsub-go/v2/device/pubsub.svg)](https://pkg.go.dev/github.com/ably/ably-pubsub-go/v2/device/pubsub)
[![License](https://badgen.net/github/license/ably/ably-go)](./LICENSE)

# Ably Pub/Sub Go SDK

The Go SDK for [Ably Pub/Sub](https://ably.com/docs/basics): publish and subscribe to channels,
and use message history and presence, from servers and from applications on end-user devices.

## Overview

The SDK has two entry points, one for each side of the connection. You choose the entry point by
where your code runs, and create your client with its constructor.

| Package | Use it for | Entry point |
|---------|-----------|-------------|
| `github.com/ably/ably-pubsub-go/v2/server/pubsub` | Servers and other trusted backend environments | `NewHTTPClient`, `NewRealtimeClient` |
| `github.com/ably/ably-pubsub-go/v2/device/pubsub` | Applications on end-user devices | `NewClient` |

## Requirements

Go 1.25 or later.

## Installation

Use exactly one of `server/pubsub` or `device/pubsub` in a program.

### Server

```bash
go get github.com/ably/ably-pubsub-go/v2/server/pubsub
```

### Device

```bash
go get github.com/ably/ably-pubsub-go/v2/device/pubsub
```

### Migrating from 1.x

Version 2 ships under a new module path, and the `github.com/ably/ably-go/ably` package no longer
exists. Add the entry point you need, remove the 1.x requirement, then change each import and the
`ably.` qualifiers to `pubsub.`:

```bash
go get github.com/ably/ably-pubsub-go/v2/server/pubsub   # or .../device/pubsub
go mod edit -droprequire github.com/ably/ably-go
```

See [UPDATING.md](./UPDATING.md#version-14x-to-20) for the import mapping, the `REST` to `HTTP`
renames, behaviour changes and removed APIs.

## Quick start

### Server

Use an API key on a server. The following code connects, subscribes to a channel, publishes a
message to it and waits for the message to arrive.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ably/ably-pubsub-go/v2/server/pubsub"
)

func main() {
	ctx := context.Background()

	client, err := pubsub.NewRealtimeClient(pubsub.WithKey("xxxxxx.yyyyyy:zzzzzzzz"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Subscribing attaches the channel. Publishing while the client is still
	// connecting queues the message until it's connected.
	channel := client.Channels.Get("test-channel")
	received := make(chan *pubsub.Message, 1)
	unsubscribe, err := channel.SubscribeAll(ctx, func(msg *pubsub.Message) {
		select {
		case received <- msg:
		default:
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	defer unsubscribe()

	if err := channel.Publish(ctx, "test-event", "hello world"); err != nil {
		log.Fatal(err)
	}
	msg := <-received
	fmt.Printf("Received: %v\n", msg.Data)
}
```

To publish over HTTP without a realtime connection, create an HTTP client instead:

```go
package main

import (
	"context"
	"log"

	"github.com/ably/ably-pubsub-go/v2/server/pubsub"
)

func main() {
	client, err := pubsub.NewHTTPClient(pubsub.WithKey("xxxxxx.yyyyyy:zzzzzzzz"))
	if err != nil {
		log.Fatal(err)
	}
	err = client.Channels.Get("test-channel").Publish(context.Background(), "test-event", "hello world")
	if err != nil {
		log.Fatal(err)
	}
}
```

> [!IMPORTANT]
> Clients from `server/pubsub` declare themselves as servers. Authenticate them with an API key.
> Token authentication works only with a token that carries the signed `x-ably-clientType=server`
> claim, which today means a JWT; with any other token, Ably rejects the connection or request with
> error 40167. Clients from `device/pubsub` are not affected. See
> [Server clients and token authentication](./UPDATING.md#server-clients-and-token-authentication).

### Device

Authenticate a device with tokens issued by your own server, through `WithAuthURL` or
`WithAuthCallback`. Never embed an API key in an application that runs on an end-user device.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ably/ably-pubsub-go/v2/device/pubsub"
)

func main() {
	ctx := context.Background()

	client, err := pubsub.NewClient(
		pubsub.WithAuthURL("https://your-server.example.com/ably-auth"),
		pubsub.WithClientID("me"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// The client connects automatically. Subscribing attaches the channel.
	channel := client.Channels.Get("test-channel")
	received := make(chan *pubsub.Message, 1)
	unsubscribe, err := channel.SubscribeAll(ctx, func(msg *pubsub.Message) {
		select {
		case received <- msg:
		default:
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	defer unsubscribe()

	if err := channel.Publish(ctx, "test-event", "hello world"); err != nil {
		log.Fatal(err)
	}
	msg := <-received
	fmt.Printf("Received: %v\n", msg.Data)
}
```

To get tokens from your own code instead of a URL, use `WithAuthCallback`. It returns a token, a
token request or token details (`pubsub.Tokener`):

```go
client, err := pubsub.NewClient(
	pubsub.WithAuthCallback(func(ctx context.Context, params pubsub.TokenParams) (pubsub.Tokener, error) {
		return requestTokenFromYourServer(ctx, params) // your function
	}),
	pubsub.WithClientID("me"),
)
```

## Common configuration

Both packages export the same `With...` options. Pass them to the constructors.

### Endpoint

`WithEndpoint` selects the Ably host for both HTTP requests and the realtime connection. It
replaces the 1.x `WithEnvironment`, `WithRESTHost` and `WithRealtimeHost` options.

```go
client, err := pubsub.NewRealtimeClient(
	pubsub.WithKey("xxxxxx.yyyyyy:zzzzzzzz"),
	pubsub.WithEndpoint("nonprod:sandbox"),
)
```

Never put a port in the endpoint: `localhost:8080` is not a hostname and does not resolve. Set the
port with `WithTLSPort` instead, or with `WithPort` when TLS is off (`WithTLS(false)`, which needs
token authentication). See
[Host options are replaced by WithEndpoint](./UPDATING.md#host-options-are-replaced-by-withendpoint)
for how each value resolves.

### Proxy

The SDK uses Go's standard proxy settings. Set the `HTTP_PROXY` and `HTTPS_PROXY` environment
variables, including the scheme, and both HTTP requests and the realtime connection go through the
proxy:

```bash
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

If the proxy needs authentication, put the credentials in the URL, for example
`http://username:password@proxy.example.com:8080`. See
[`http.ProxyFromEnvironment`](https://pkg.go.dev/net/http#ProxyFromEnvironment) for the details.

To set a proxy in code, pass your own `http.Client` with `WithHTTPClient`. It applies to HTTP
requests only; the realtime connection still uses the environment variables.

```go
proxyURL, err := url.Parse("http://proxy.example.com:8080")
if err != nil {
	log.Fatal(err)
}
client, err := pubsub.NewHTTPClient(
	pubsub.WithKey("xxxxxx.yyyyyy:zzzzzzzz"),
	pubsub.WithHTTPClient(&http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}),
)
```

Connection reliability then depends on the proxy, and Ably can't help with errors the proxy
introduces.

### Logging

Set the level with `WithLogLevel`: `LogNone`, `LogError`, `LogWarning`, `LogInfo`, `LogVerbose`
or `LogDebug` (the most verbose). The default is `LogWarning`. Logs go to standard error unless
you pass your own `Logger` with `WithLogHandler`:

```go
type myLogger struct{}

func (myLogger) Printf(level pubsub.LogLevel, format string, v ...interface{}) {
	log.Printf(format, v...)
}

client, err := pubsub.NewRealtimeClient(
	pubsub.WithKey("xxxxxx.yyyyyy:zzzzzzzz"),
	pubsub.WithLogLevel(pubsub.LogDebug),
	pubsub.WithLogHandler(myLogger{}),
)
```

## Known limitations

- [Push notification targets](https://ably.com/docs/account/app/notifications#push-notification-target) don't apply to this SDK.
- The Push Notifications Admin API isn't supported ([REST](https://ably.com/docs/api/rest-sdk/push-admin), [realtime](https://ably.com/docs/api/realtime-sdk/push-admin)).
- [Channel SUSPENDED state](https://github.com/ably/ably-go/issues/568) is only partly supported.
- Ping isn't implemented.

## Further reading

- [Getting started with Pub/Sub using Go](https://ably.com/docs/getting-started/go)
- [SDK setup for Go](https://ably.com/docs/getting-started/setup?lang=go)
- [Ably Pub/Sub documentation](https://ably.com/docs/basics) and [examples](https://ably.com/examples?product=pubsub)
- [UPDATING.md](./UPDATING.md): migrating from 1.x
- [CHANGELOG.md](./CHANGELOG.md): release notes; all Ably releases are listed on [changelog.ably.com](https://changelog.ably.com)
- [examples/](./examples): example programs

## Contributing

Read [CONTRIBUTING.md](./CONTRIBUTING.md) before contributing.

## Support, feedback and troubleshooting

For help, visit Ably's [support page](https://ably.com/support). Report bugs and compatibility
problems on [GitHub Issues](https://github.com/ably/ably-go/issues).

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE).
