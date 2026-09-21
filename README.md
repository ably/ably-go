![Ably Pub/Sub Go Header](/image/goSDK-github.png)
[![Go Reference](https://pkg.go.dev/badge/github.com/ably/ably-pubsub-go/server.svg)](https://pkg.go.dev/github.com/ably/ably-pubsub-go/server)
[![License](https://badgen.net/github/license/ably/ably-go)](https://github.com/ably/ably-go/blob/main/LICENSE)

---

# Ably Pub/Sub Go SDK

Build any realtime experience using Ably’s Pub/Sub Go SDK, supported on all popular platforms and frameworks.

Ably Pub/Sub provides flexible APIs that deliver features such as pub-sub messaging, message history, presence, and push notifications. Utilizing Ably’s realtime messaging platform, applications benefit from its highly performant, reliable, and scalable infrastructure.

Find out more:

* [Ably Pub/Sub docs.](https://ably.com/docs/basics)
* [Ably Pub/Sub examples.](https://ably.com/examples?product=pubsub)

---

## Getting started

Everything you need to get started with Ably:

* [Getting started in Pub/Sub using Go.](https://ably.com/docs/getting-started/go)
* [SDK Setup for Go.](https://ably.com/docs/getting-started/setup?lang=go)

---

## Supported platforms

Ably aims to support a wide range of platforms. If you experience any compatibility issues, open an issue in the repository or contact [Ably support](https://ably.com/support).

> [!IMPORTANT]
> Go SDK versions < 1.2.14 will be [deprecated](https://ably.com/docs/platform/deprecate/protocol-v1) from November 1, 2025.

---

## Installation

To get started with your project, install the package:

```bash
~ $ go get -u github.com/ably/ably-pubsub-go
```

> [!NOTE]
> The module is being renamed from `github.com/ably/ably-go` to
> `github.com/ably/ably-pubsub-go` for v2. Until the repository move lands,
> this path does not resolve and `go get` will fail; build against a local
> checkout with a `replace` directive in the meantime.

The SDK has two entry points, and which one you import depends on where your
code runs:

Both are named `pubsub`, so only the import path differs between them — an
import of either path binds the name `pubsub`, not `server` or `device`:

| Import path | Use it for |
| --- | --- |
| `github.com/ably/ably-pubsub-go/server` | Trusted environments that authenticate with an API key. Connections are exempt from monthly-active-user counting. Offers both an HTTP client and a realtime client. |
| `github.com/ably/ably-pubsub-go/device` | Applications on end-user devices, identified by a `clientId` and counted on accounts with monthly-active-user billing. Offers a realtime client. |

Each entry point exposes the whole API it needs — channels, messages, presence,
options, errors — so an application imports one of them and nothing else.

---

## Usage

The following code connects to Ably's realtime messaging service, subscribes to a channel to receive messages, and publishes a test message to that same channel:

```go
import "github.com/ably/ably-pubsub-go/server"

// Initialize an Ably realtime client
client, err := pubsub.NewRealtimeClient(
        pubsub.WithKey("your-ably-api-key"),
        pubsub.WithClientID("me"),
)
if err != nil {
        return err
}

// Wait for connection to be established
ch := make(chan pubsub.ConnectionStateChange, 1)
client.Connection.On(pubsub.ConnectionEventConnected, func(change pubsub.ConnectionStateChange) {
        ch <- change
})
<-ch
fmt.Println("Connected to Ably")

// Get a reference to the 'test-channel' channel
channel := client.Channels.Get("test-channel")

// Subscribe to all messages published to this channel
channel.SubscribeAll(context.Background(), func(msg *pubsub.Message) {
        fmt.Printf("Received message: %s\n", msg.Data)
})

// Publish a test message to the channel
channel.Publish(context.Background(), "test-event", "hello world")
```

On an end-user device, the same code imports
`github.com/ably/ably-pubsub-go/device` instead and calls `pubsub.NewClient`.

---

## Proxy
The `ably-go` SDK doesn't provide a direct option to set a proxy in its configuration. However, you can use standard environment variables to set up a proxy for all your HTTP and HTTPS connections. The Go programming language will automatically handle these settings.

<details>
<summary>Set up proxy via environment variables details.</summary>


To configure the proxy, set the `HTTP_PROXY` and `HTTPS_PROXY` environment variables with the URL of your proxy server. Here's an example of how to set these variables:

```bash
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

The `proxy.example.com` is the domain or IP address of your proxy server and `8080` is the port number of your proxy server.


Include the protocol (`http` or `https`) in the proxy URL. If your proxy requires authentication, you can include the username and password in the URL, for example: `http://username:password@proxy.example.com:8080`.

After setting the environment variables, the `ably-go` SDK will route its traffic through the specified proxy for both HTTP and realtime clients.

For more details on environment variable configurations in Go, see [ Go documentation on http.ProxyFromEnvironment](https://golang.org/pkg/net/http/#ProxyFromEnvironment).

</details>


<details>
<summary>Set up proxy via custom http client details.</summary>


For the HTTP client, you can also set a proxy by providing the custom http client option `pubsub.WithHTTPClient`:

```go
pubsub.WithHTTPClient(&http.Client{
        Transport: &http.Transport{
                Proxy:        proxy // custom proxy implementation
        },
})
```

Connection reliability is totally dependent on health of proxy server and ably will not be responsible for errors introduced by proxy server.

</details>

---

## Releases

The [CHANGELOG.md](./CHANGELOG.md) contains details of the latest releases for this SDK. You can also view all Ably releases on [changelog.ably.com](https://changelog.ably.com).

---

## Contributing

Read the [CONTRIBUTING.md](./CONTRIBUTING.md) guidelines to contribute to Ably.

---

## Support, feedback and troubleshooting

For help or technical support, visit Ably's [support page](https://ably.com/support) or [GitHub Issues](https://github.com/ably/ably-go/issues) for community-reported bugs and discussions.

---

### Breaking API Changes in Version 1.2.x

Version 1.2 introduced significant breaking changes from 1.1.5. See the [Upgrade / Migration Guide](UPDATING.md) for details on what changes are required in your code.

#### REST API

- [Push notification target](https://ably.com/docs/account/app/notifications#push-notification-target) functionality is not applicable to this SDK.
- No support for [Push Notifications Admin API.](https://ably.com/docs/api/rest-sdk/push-admin)

#### Realtime API

- Partial support for [Channel Suspended State.](https://github.com/ably/ably-go/issues/568)
- Ping functionality is not implemented.
- [Push notification target](https://ably.com/docs/account/app/notifications#push-notification-target) functionality is not applicable to this SDK.
- No support for [Push Notifications Admin API.](https://ably.com/docs/api/realtime-sdk/push-admin)