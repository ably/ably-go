# How to run

1. Realtime

- Go to realtime dir `cd realtime`
- Set the `ABLY_KEY` environment variable to your [Ably API key](https://faqs.ably.com/setting-up-and-managing-api-keys)
- Realtime channel pub-sub

    `go run presence/main.go`


- Realtime channel presence

    `go run pub-sub/main.go`
    
2. REST

- Go to rest dir `cd rest`     
- Set the `ABLY_KEY` environment variable to your [Ably API key](https://faqs.ably.com/setting-up-and-managing-api-keys)
- Rest channel publish

    `go run publish/main.go`

- Rest channel presence

    `go run presence/main.go`

- Rest channel message history

    `go run history/main.go`

- Rest application stats

    `go run stats/main.go`
