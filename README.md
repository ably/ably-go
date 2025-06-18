![Ably Pub/Sub Go Header](/image/goSDK-github.png)
[![Go Reference](https://pkg.go.dev/badge/github.com/ably/ably-go/ably.svg)](https://pkg.go.dev/github.com/ably/ably-go/ably)
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

- [Quickstart in Pub/Sub using Go.](https://ably.com/docs/getting-started/quickstart?lang=go)

---

## Releases

The [CHANGELOG.md](/ably/ably-go/blob/main/CONTRIBUTING.md) contains details of the latest releases for this SDK. You can also view all Ably releases on [changelog.ably.com](https://changelog.ably.com).

---

## Contributing

Read the [CONTRIBUTING.md](./CONTRIBUTING.md) guidelines to contribute to Ably.

---

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

**Important Note** - Connection reliability is totally dependent on health of proxy server and ably will not be responsible for errors introduced by proxy server.

</details>


## Support, feedback and troubleshooting

For help or technical support, visit Ably's [support page](https://ably.com/support) or [GitHub Issues](https://github.com/ably/ably-go/issues) for community-reported bugs and discussions.

---

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

### Realtime API

- Channel suspended state is partially implemented. See [suspended channel state](https://github.com/ably/ably-go/issues/568).

- Realtime Ping function is not implemented.

- Message Delta Compression is not implemented.

- Push Notification Target functional is not applicable for the SDK and thus not implemented.

## Support, feedback and troubleshooting

Please visit https://faqs.ably.com/ for access to our knowledgebase. If you require support, please visit https://ably.com/support to submit a support ticket.

You can also view the [community reported Github issues](https://github.com/ably/ably-go/issues).

## Contributing
For guidance on how to contribute to this project, see [CONTRIBUTING.md](CONTRIBUTING.md).
