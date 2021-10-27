# How to run

1. Realtime

- Go to realtime dir `cd realtime`
- Set the `ABLY_KEY` environment variable to your [Ably API key](https://faqs.ably.com/setting-up-and-managing-api-keys)
- Realtime channel pub-sub

    `go run presence.go constants.go`


- Realtime channel presence

    `go run pub-sub.go constants.go`
    
2. REST

- Go to rest dir `cd rest`     
- Set the `ABLY_KEY` environment variable to your [Ably API key](https://faqs.ably.com/setting-up-and-managing-api-keys)
- Rest channel publish

    `go run publish.go constants.go utils.go`

- Rest channel presence

    `go run presence.go constants.go utils.go`

- Rest channel message history

    `go run history.go constants.go utils.go`

- Rest application stats

    `go run stats.go utils.go constants.go`
