# How to run

1. Realtime

- Go to realtime dir `cd realtime`
- Set the `ABLY_KEY` environment variable to your [Ably API key](https://faqs.ably.com/setting-up-and-managing-api-keys)
- Realtime channel pub-sub

    `go run presence/main.go`


- Realtime channel presence

    `go run pub-sub/main.go`
    
2. HTTP

- Go to http dir `cd http`
- Set the `ABLY_KEY` environment variable to your [Ably API key](https://faqs.ably.com/setting-up-and-managing-api-keys)
- HTTP channel publish

    `go run publish/main.go`

- HTTP channel presence

    `go run presence/main.go`

- HTTP channel message history

    `go run history/main.go`

- HTTP application stats

    `go run stats/main.go`
