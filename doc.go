// Package libyaci provides a dynamic gRPC client that leverages server reflection
// to invoke any gRPC method without requiring precompiled protobuf stubs.
//
// This package solves a common pain point when working with gRPC services: the need
// to generate and maintain protobuf code for every service you want to call. By using
// gRPC server reflection, this client can:
//
//   - Discover all available services and methods at runtime
//   - Automatically resolve protobuf message types, including nested Any types
//   - Accept JSON input and return JSON output for easy integration
//   - Work with any gRPC server that has reflection enabled (most Cosmos SDK chains do)
//
// # Quick Start
//
// Connect to a gRPC server and call a method:
//
//	client, err := libyaci.Dial(ctx, "localhost:9090", libyaci.WithInsecure())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Call any method with JSON request/response
//	resp, err := client.Invoke(
//	    "cosmos.bank.v1beta1.Query.Balance",
//	    []byte(`{"address":"cosmos1xyz...", "denom":"uatom"}`),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(string(resp))
//
// # Service Discovery
//
// List all available services and their methods:
//
//	services := client.ListServices()
//	for _, svc := range services {
//	    fmt.Printf("Service: %s\n", svc)
//	    methods, _ := client.ListMethods(svc)
//	    for _, m := range methods {
//	        fmt.Printf("  - %s\n", m)
//	    }
//	}
//
// # Working with Go Types
//
// For convenience, you can use Go structs instead of raw JSON:
//
//	type BalanceRequest struct {
//	    Address string `json:"address"`
//	    Denom   string `json:"denom"`
//	}
//	type BalanceResponse struct {
//	    Balance struct {
//	        Denom  string `json:"denom"`
//	        Amount string `json:"amount"`
//	    } `json:"balance"`
//	}
//
//	var resp BalanceResponse
//	err := client.InvokeJSON(
//	    "cosmos.bank.v1beta1.Query.Balance",
//	    BalanceRequest{Address: "cosmos1xyz...", Denom: "uatom"},
//	    &resp,
//	)
//
// # Raw Protobuf Access
//
// For advanced use cases, access the raw protobuf message:
//
//	msg, err := client.InvokeRaw("cosmos.base.tendermint.v1beta1.Service.GetLatestBlock", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Access fields directly from the dynamic message
//	blockField := msg.ProtoReflect().Get(msg.Descriptor().Fields().ByName("block"))
//
// # Connection Options
//
// Configure the client with various options:
//
//	client, err := libyaci.Dial(ctx, "grpc.example.com:443",
//	    libyaci.WithMaxRetries(5),
//	    libyaci.WithMaxRecvMsgSize(16*1024*1024), // 16MB
//	    libyaci.WithDialOptions(
//	        grpc.WithPerRPCCredentials(myCredentials),
//	    ),
//	)
//
// # Use Cases
//
// This package is particularly useful for:
//
//   - CLI tools that need to interact with multiple gRPC services
//   - Debugging and testing gRPC endpoints
//   - Building generic gRPC proxies or gateways
//   - Migrating REST/RPC applications to gRPC incrementally
//   - Prototyping before committing to code generation
//   - Blockchain indexers that need to decode arbitrary transaction types
//
// # Cosmos SDK Integration
//
// This package was originally developed for indexing Cosmos SDK blockchains.
// All Cosmos SDK chains (v0.50+) expose gRPC with reflection enabled, making
// this package ideal for:
//
//   - Querying bank balances, staking info, governance proposals
//   - Fetching blocks and transactions with fully decoded message types
//   - Decoding Any-wrapped messages in transaction bodies
//   - Building chain-agnostic indexers and explorers
//
// # Thread Safety
//
// The Client and Resolver types are safe for concurrent use by multiple goroutines.
// The underlying proto registry uses read-write locks to ensure thread safety when
// fetching new descriptors on-demand.
//
// # Performance Considerations
//
// On first connection, the client fetches all proto descriptors from the server.
// This may take a few seconds for servers with many services (like Cosmos SDK chains).
// Subsequent calls are fast as descriptors are cached locally.
//
// For long-running applications, consider reusing the client rather than creating
// new connections for each request.
package libyaci
