# VCDiff Plugin for Ably Go SDK

This document describes how to use VCDiff (RFC 3284) delta compression with the Ably Go SDK.

## Overview

The VCDiff plugin enables automatic decoding of delta-compressed messages sent by Ably. This reduces bandwidth usage and improves performance for channels that transmit similar messages over time.

## Quick Start

```go
package main

import (
    "context"
    "github.com/ably/ably-go/ably"
)

func main() {
    // Create the VCDiff plugin
    vcdiffPlugin := ably.NewVCDiffPlugin()

    // Create Ably client with the plugin
    client, err := ably.NewRealtime(
        ably.WithKey("your-api-key"),
        ably.WithVCDiffPlugin(vcdiffPlugin),
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Get a channel with delta encoding enabled
    channel := client.Channels.Get("my-channel",
        ably.ChannelWithVCDiff())

    // Subscribe to messages - they will be automatically decoded
    ctx := context.Background()
    unsubscribe, err := channel.Subscribe(ctx, "", func(msg *ably.Message) {
        // Messages are automatically decoded by the VCDiff plugin
        fmt.Printf("Received decoded message: %v\n", msg.Data)
    })
    if err != nil {
        panic(err)
    }
    defer unsubscribe()

    // Attach to the channel
    err = channel.Attach(ctx)
    if err != nil {
        panic(err)
    }

    // Publish messages - they will be delta-encoded by the server
    err = channel.Publish(ctx, "event", map[string]interface{}{
        "temperature": 23.5,
        "humidity": 45.2,
        "timestamp": time.Now().Unix(),
    })
    if err != nil {
        panic(err)
    }
}
```

## Implementation Details

### Plugin Interface

The VCDiff plugin implements the `VCDiffDecoder` interface:

```go
type VCDiffDecoder interface {
    Decode(delta []byte, base []byte) ([]byte, error)
}
```

### VCDiffPlugin

- **File**: `vcdiff_plugin.go`
- **Usage**: `ably.NewVCDiffPlugin()`
- **Description**: Production-ready implementation using the [ably/vcdiff-go](https://github.com/ably/vcdiff-go) library

### Client Configuration

Enable VCDiff support by:

1. **Adding the plugin to the client**:
   ```go
   client, err := ably.NewRealtime(
       ably.WithKey("your-api-key"),
       ably.WithVCDiffPlugin(ably.NewVCDiffPlugin()),
   )
   ```

2. **Enabling delta on channels**:
   ```go
   channel := client.Channels.Get("my-channel",
       ably.ChannelWithVCDiff())
   ```

## Error Handling

The VCDiff plugin handles several error conditions according to the Ably specification:

### Error Code 40018: Delta Decode Failure
- **Cause**: VCDiff decode operation failed
- **Recovery**: Channel automatically reattaches to recover (RTL18)
- **Example**: Corrupted delta data, version mismatch

### Error Code 40019: Missing VCDiff Plugin
- **Cause**: Delta message received but no VCDiff plugin configured
- **Recovery**: Channel transitions to FAILED state
- **Solution**: Add VCDiff plugin to client configuration

## Specification Compliance

This implementation follows the Ably specification:

- **RTL18**: Delta decode failure recovery
- **RTL19**: Base payload management for delta decoding
- **RTL20**: Message ID validation for delta reference
- **RTL4k**: Channel parameter transmission (`delta=vcdiff`)
- **RTL4k1**: Channel parameter exposure
- **PC3**: Plugin system for delta decoders
- **VD1-VD2a**: VCDiff decoder interface and implementation

## Testing

Run the VCDiff plugin tests:

```bash
go test ./ably -run TestVCDiffPlugin -v
```

Run all delta-related tests:

```bash
go test ./ably -run TestDelta -v
```

## Dependencies

The production VCDiff plugin requires:

- `github.com/ably/vcdiff-go`: Official VCDiff decoder implementation

This dependency is automatically managed by Go modules when you use the plugin.

## Performance

VCDiff delta compression provides significant benefits:

- **Bandwidth**: Reduces message size by transmitting only differences
- **CPU**: Minimal decode overhead for typical message patterns
- **Memory**: Maintains small base payload cache per channel

## Limitations

- **Server Support**: Requires Ably server-side delta encoding
- **Channel Scope**: Delta state is per-channel, not cross-channel
- **Message Order**: Out-of-order messages may cause decode failures
- **Base Payload**: Limited to recent message for delta reference

## See Also

- [Ably Delta Compression Documentation](https://ably.com/docs/realtime/types#delta-compression)
- [VCDiff RFC 3284](https://tools.ietf.org/rfc/rfc3284.txt)
- [ably/vcdiff-go Library](https://github.com/ably/vcdiff-go)