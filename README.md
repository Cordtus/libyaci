# libyaci

A dynamic gRPC client for Go that uses server reflection to invoke any method without precompiled protobuf stubs.

## Features

- **Zero code generation** - Call any gRPC method using just the method name and JSON
- **Automatic type resolution** - Dynamically resolves protobuf types including nested `Any` fields
- **Service discovery** - List all available services and methods at runtime
- **Thread-safe** - Safe for concurrent use across multiple goroutines
- **Cosmos SDK ready** - Built and tested with Cosmos SDK chains (v0.50+)

## Installation

```bash
go get github.com/Cordtus/libyaci
```

## Quick Start

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

    // Connect to a gRPC server with reflection enabled
    client, err := libyaci.Dial(ctx, "localhost:9090", libyaci.WithInsecure())
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Call any method with JSON
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

## Usage

### Basic Invocation

```go
// JSON request/response
resp, err := client.Invoke("service.Method", []byte(`{"field": "value"}`))

// Empty request
resp, err := client.Invoke("service.Method", nil)
```

### Using Go Structs

```go
type BalanceRequest struct {
    Address string `json:"address"`
    Denom   string `json:"denom"`
}

type BalanceResponse struct {
    Balance struct {
        Denom  string `json:"denom"`
        Amount string `json:"amount"`
    } `json:"balance"`
}

var result BalanceResponse
err := client.InvokeJSON(
    "cosmos.bank.v1beta1.Query.Balance",
    BalanceRequest{Address: "cosmos1...", Denom: "uatom"},
    &result,
)
```

### Service Discovery

```go
// List all services
services := client.ListServices()
for _, svc := range services {
    fmt.Println(svc)
}

// List methods for a service
methods, err := client.ListMethods("cosmos.bank.v1beta1.Query")

// Get method input/output types
input, output, err := client.DescribeMethod("cosmos.bank.v1beta1.Query.Balance")
// input: "cosmos.bank.v1beta1.QueryBalanceRequest"
// output: "cosmos.bank.v1beta1.QueryBalanceResponse"
```

### Raw Protobuf Access

For advanced use cases, access the underlying dynamic protobuf message:

```go
msg, err := client.InvokeRaw("cosmos.base.tendermint.v1beta1.Service.GetLatestBlock", nil)
if err != nil {
    log.Fatal(err)
}

// Access fields directly
blockField := msg.ProtoReflect().Get(msg.Descriptor().Fields().ByName("block"))
```

### Extract Specific Fields

```go
height, err := client.ExtractField(
    "cosmos.base.tendermint.v1beta1.Service.GetLatestBlock",
    nil,
    "block",
)
```

## Configuration

```go
client, err := libyaci.Dial(ctx, "grpc.example.com:443",
    // Disable TLS (for local development)
    libyaci.WithInsecure(),

    // Custom retry count (default: 3)
    libyaci.WithMaxRetries(5),

    // Increase max message size (default: 4MB)
    libyaci.WithMaxRecvMsgSize(16 * 1024 * 1024),

    // Add custom gRPC dial options
    libyaci.WithDialOptions(
        grpc.WithPerRPCCredentials(myCredentials),
    ),
)
```

## Requirements

- Go 1.21+
- gRPC server with [reflection](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md) enabled

Most Cosmos SDK chains (v0.50+) have gRPC reflection enabled by default.

## How It Works

1. **Connection** - Establishes a gRPC connection to the target server
2. **Discovery** - Uses the gRPC reflection API to fetch all available service descriptors
3. **Resolution** - Builds a local proto registry from the fetched descriptors
4. **Invocation** - Dynamically creates request messages, invokes methods, and marshals responses to JSON
5. **On-demand fetching** - If an unknown `Any` type is encountered, fetches its descriptor automatically

## Use Cases

- **CLI tools** - Build interactive tools for any gRPC service
- **Debugging** - Quickly test gRPC endpoints without code generation
- **Proxies/Gateways** - Build generic gRPC-to-REST gateways
- **Migration** - Incrementally move from REST to gRPC
- **Blockchain indexers** - Decode arbitrary Cosmos SDK transaction types
- **Prototyping** - Experiment before committing to code generation

## Performance

- Initial connection fetches all descriptors (may take a few seconds for large services)
- Subsequent calls are fast as descriptors are cached
- Thread-safe caching means descriptors are fetched only once
- Reuse the client for best performance in long-running applications

## Example CLI

The repository includes an example CLI tool:

```bash
# Build the example
go build -o grpc-cli ./example

# List all services
./grpc-cli -addr localhost:9090 -insecure -list

# Call a method
./grpc-cli -addr localhost:9090 -insecure \
    -method "cosmos.bank.v1beta1.Query.TotalSupply" \
    -request "{}"
```

## Related Projects

- [yaci](https://github.com/manifest-network/yaci) - Blockchain indexer that uses this library
- [grpcurl](https://github.com/fullstorydev/grpcurl) - CLI tool for gRPC (inspiration for this library)
- [grpc-go](https://github.com/grpc/grpc-go) - Official Go gRPC library

## License

MIT License - see [LICENSE](LICENSE) for details.
