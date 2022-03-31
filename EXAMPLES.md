# Examples

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

[See Ably documentation for ErrorInfo.](https://www.ably.io/documentation/realtime/types#error-info)

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

