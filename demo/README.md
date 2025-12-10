# Multi-Chain Explorer Demo

A demonstration of libyaci's capabilities - query ANY Cosmos chain without compiled protobuf stubs.

## What This Demo Shows

This explorer connects to any Cosmos SDK chain's gRPC endpoint and dynamically queries:

1. **Node Information** - Chain ID, app version, SDK version
2. **Service Discovery** - Lists all available gRPC services via reflection
3. **Chain Status** - Latest block height and time
4. **Token Economics** - Total token supply
5. **Staking Overview** - Bonded/unbonded tokens, validator count
6. **Governance** - Active proposals in voting period
7. **IBC Connectivity** - Channel count and status
8. **Custom Modules** - Detects chain-specific modules (Osmosis pools, Celestia blobs, etc.)

## The Point

With **traditional gRPC clients**, building this multi-chain explorer would require:

```go
import (
    // Core SDK - dozens of packages
    "github.com/cosmos/cosmos-sdk/x/bank/types"
    "github.com/cosmos/cosmos-sdk/x/staking/types"
    "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
    // ... 20+ more

    // IBC
    "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
    "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"

    // Chain-specific (if you want Osmosis)
    "github.com/osmosis-labs/osmosis/v25/x/poolmanager/types"
    "github.com/osmosis-labs/osmosis/v25/x/gamm/types"

    // Chain-specific (if you want Celestia)
    "github.com/celestiaorg/celestia-app/x/blob/types"

    // ... and so on for every chain
)
```

**Result:** 200+ dependencies, version conflicts, recompilation for each chain.

With **libyaci**, the entire explorer is:

```go
import "github.com/Cordtus/libyaci"

client, _ := libyaci.Dial(ctx, "any-chain:9090")
resp, _ := client.Invoke("any.module.v1.Query.AnyMethod", jsonRequest)
```

**Result:** 2 dependencies. Works with every chain. Zero recompilation.

## Usage

```bash
# Build
go build -o explorer .

# Query different chains (use any public gRPC endpoint)
./explorer -addr grpc.osmosis.zone:9090
./explorer -addr cosmos-grpc.polkachu.com:14990
./explorer -addr grpc-celestia.mzonder.com:443
./explorer -addr injective-grpc.polkachu.com:14390

# For endpoints without TLS
./explorer -addr localhost:9090 -insecure
```

## Example Output

```
==============================
  LIBYACI DEMO: Multi-Chain Explorer
==============================

This demo shows how libyaci can query ANY Cosmos chain without
pre-compiled protobuf stubs or chain-specific dependencies.

1. Node Information
-------------------
  Chain ID:      osmosis-1
  Node Moniker:  my-node
  App Name:      osmosisd
  App Version:   25.0.0
  SDK Version:   v0.50.5

2. Available Services (via gRPC Reflection)
-------------------------------------------
  Found 87 services across 24 modules:

  cosmos.auth (3 services)
  cosmos.bank (4 services)
  osmosis.poolmanager (5 services)
  ...

3. Chain Status
---------------
  Latest Block:  18234567
  Block Time:    2024-01-15 10:30:45 UTC
  Block Age:     2s ago

...

8. Chain-Specific Modules
-------------------------
  Detected 5 chain-specific module(s):

  - osmosis (23 services)
    Example: Pool liquidity, swap routes, incentives

  With libyaci, you can query these custom modules
  without importing their proto definitions!
```

## Key Takeaway

The same binary works with Osmosis, Cosmos Hub, Celestia, Injective, or any other Cosmos chain - without modification, recompilation, or additional dependencies.

This is the power of dynamic gRPC reflection.
