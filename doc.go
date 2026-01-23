// Package libyaci provides a dynamic gRPC client that uses server reflection
// to invoke methods without precompiled protobuf stubs.
//
// # Basic Usage
//
//	client, err := libyaci.Dial(ctx, "localhost:9090", libyaci.WithInsecure())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	resp, err := client.Invoke(
//	    "cosmos.bank.v1beta1.Query.Balance",
//	    []byte(`{"address":"cosmos1...", "denom":"uatom"}`),
//	)
//
// # Invocation Methods
//
// Invoke returns JSON bytes:
//
//	resp, err := client.Invoke("package.Service.Method", []byte(`{}`))
//
// InvokeJSON marshals/unmarshals Go types:
//
//	var result ResponseType
//	err := client.InvokeJSON("package.Service.Method", request, &result)
//
// InvokeRaw returns the dynamic protobuf message:
//
//	msg, err := client.InvokeRaw("package.Service.Method", nil)
//	field := msg.ProtoReflect().Get(msg.Descriptor().Fields().ByName("field"))
//
// # Service Discovery
//
//	services := client.ListServices()
//	methods, err := client.ListMethods("cosmos.bank.v1beta1.Query")
//	input, output, err := client.DescribeMethod("cosmos.bank.v1beta1.Query.Balance")
//
// # Cosmos SDK Convenience Methods
//
// High-level methods for common Cosmos SDK queries:
//
//	// Block queries
//	block, err := client.GetLatestBlock()
//	height, err := client.GetLatestBlockHeight()
//	block, err := client.GetBlockByHeight(12345)
//	earliest, err := client.GetEarliestBlockHeight()
//
//	// Transaction queries
//	txs, err := client.GetTxsByHeight(12345)
//	txsParsed, err := client.GetTxsByHeightParsed(12345)
//
//	// Chain info
//	chainID, err := client.GetChainID()
//	denom, err := client.GetBondDenom()
//
//	// Account queries
//	validators, err := client.GetAllValidators()  // returns map[address]moniker
//	modules, err := client.GetModuleAccounts()    // returns map[address]name
//	balance, err := client.GetBalance(address, denom)
//	balances, err := client.GetAllBalances(address)
//	delegations, err := client.GetDelegations(delegatorAddr)
//
// # Configuration
//
//	client, err := libyaci.Dial(ctx, "grpc.example.com:443",
//	    libyaci.WithInsecure(),
//	    libyaci.WithMaxRetries(5),
//	    libyaci.WithMaxRecvMsgSize(16*1024*1024),
//	    libyaci.WithDialTimeout(30*time.Second),
//	    libyaci.WithProtoDir("./protos"),  // local protos for deprecated types
//	)
//
// # ALPN Connection Issues
//
// If connections fail with "missing selected ALPN property" errors (common with
// older Cosmos nodes), import the alpnfix subpackage before any gRPC imports:
//
//	import _ "github.com/Cordtus/libyaci/alpnfix"
//
// # Deprecated Type Resolution
//
// Historical blockchain data may contain message types from deprecated modules
// that no longer exist on the server. Use WithProtoDir to provide local .proto
// files for these types:
//
//	client, err := libyaci.Dial(ctx, addr,
//	    libyaci.WithProtoDir("./protos"),  // protos/tendermint/liquidity/v1beta1/*.proto
//	)
//
// # Thread Safety
//
// Client and Resolver are safe for concurrent use. The proto registry uses
// read-write locks when fetching descriptors on-demand.
//
// # Performance
//
// Initial connection fetches all descriptors from the server, which may take
// several seconds for servers with many services. Subsequent calls use cached
// descriptors. Reuse the client instance for best performance.
package libyaci
