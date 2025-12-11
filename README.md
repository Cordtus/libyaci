# libyaci

A dynamic gRPC client for Go that uses server reflection to invoke methods without precompiled protobuf stubs.

## Installation

```bash
go get github.com/Cordtus/libyaci
```

## Requirements

- Go 1.24+
- gRPC server with [reflection](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md) enabled

## Usage

### Connect and Invoke

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/Cordtus/libyaci"
)

func main() {
    ctx := context.Background()

    client, err := libyaci.Dial(ctx, "localhost:9090", libyaci.WithInsecure())
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Call any method with JSON request/response
    resp, err := client.Invoke(
        "cosmos.bank.v1beta1.Query.TotalSupply",
        []byte(`{}`),
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(string(resp))
}
```

### Invocation Methods

```go
// JSON request/response
resp, err := client.Invoke("package.Service.Method", []byte(`{"field": "value"}`))

// Empty request
resp, err := client.Invoke("package.Service.Method", nil)

// Using Go structs
var result ResponseType
err := client.InvokeJSON("package.Service.Method", RequestType{Field: "value"}, &result)

// Raw protobuf message access
msg, err := client.InvokeRaw("package.Service.Method", nil)
blockField := msg.ProtoReflect().Get(msg.Descriptor().Fields().ByName("block"))

// Extract a specific field
value, err := client.ExtractField("package.Service.Method", nil, "fieldName")
```

### Service Discovery

```go
// List all services
services := client.ListServices()

// List methods for a service
methods, err := client.ListMethods("cosmos.bank.v1beta1.Query")

// Get method input/output type names
input, output, err := client.DescribeMethod("cosmos.bank.v1beta1.Query.Balance")
```

### Configuration Options

```go
client, err := libyaci.Dial(ctx, "grpc.example.com:443",
    libyaci.WithInsecure(),                         // Disable TLS
    libyaci.WithMaxRetries(5),                      // Retry count (default: 3)
    libyaci.WithMaxRecvMsgSize(16 * 1024 * 1024),   // Max message size (default: 4MB)
    libyaci.WithDialOptions(grpc.WithPerRPCCredentials(creds)), // Custom gRPC options
)
```

## How It Works

1. Connects to the gRPC server
2. Fetches all service descriptors via the reflection API
3. Builds a local proto registry from the descriptors
4. Creates dynamic request messages from JSON input
5. Invokes methods and marshals responses back to JSON
6. Fetches additional descriptors on-demand for unknown `Any` types

## Example CLI

```bash
go build -o grpc-cli ./example

./grpc-cli -addr localhost:9090 -insecure -list
./grpc-cli -addr localhost:9090 -insecure -method "cosmos.bank.v1beta1.Query.TotalSupply" -request "{}"
```

## License

MIT License - see [LICENSE](LICENSE) for details.
