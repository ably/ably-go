# Upgrade / Migration Guide

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
ctx, cancelFunc := context.WithTimeout(context.Background(), 10 * time.Second)
defer cancelFunc()
```

This way, the context can be cancelled at the close of the function.

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
channel.Publish(ctx, "EventName1", "EventData1")
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

**Starting with version 1.2.0**, you must use the blocking [Pages](https://pkg.go.dev/github.com/ably/ably-go/ably#HistoryRequest.Pages) method on the returned `HistoryRequest` instance:

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
