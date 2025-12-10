# Traditional vs libyaci: A Real-World Comparison

This document compares the traditional approach to building Cosmos SDK gRPC clients with the dynamic reflection-based approach provided by libyaci.

## The Problem

Building applications that interact with Cosmos SDK chains via gRPC traditionally requires:

1. **Importing generated protobuf code** - massive dependency trees
2. **Codec setup** - complex interface registry and marshaler configuration
3. **Module registration** - manually registering every module you might encounter
4. **Per-query boilerplate** - creating typed clients for each module

Let's look at a real example.

---

## Real Code: escrow-checker

The [escrow-checker](https://github.com/jtieri/escrow-check) tool queries IBC escrow balances across Cosmos chains. Here's what's required with the traditional approach:

### 1. Dependencies (go.mod)

```go
require (
    cosmossdk.io/errors v1.0.1
    cosmossdk.io/store v1.0.2
    cosmossdk.io/x/feegrant v0.1.0
    cosmossdk.io/x/tx v0.13.0
    cosmossdk.io/x/upgrade v0.1.1
    github.com/cometbft/cometbft v0.38.2
    github.com/cosmos/cosmos-sdk v0.50.3
    github.com/cosmos/gogoproto v1.4.11
    github.com/cosmos/ibc-go/modules/capability v1.0.0
    github.com/cosmos/ibc-go/v8 v8.1.0
    google.golang.org/grpc v1.60.1
    // ... 150+ indirect dependencies
)
```

**Result:** ~187 dependencies, complex version management, frequent breaking changes between SDK versions.

### 2. Codec Setup (codec.go)

```go
package main

import (
    feegrant "cosmossdk.io/x/feegrant/module"
    "cosmossdk.io/x/tx/signing"
    "cosmossdk.io/x/upgrade"
    "github.com/cosmos/cosmos-sdk/client"
    "github.com/cosmos/cosmos-sdk/codec"
    "github.com/cosmos/cosmos-sdk/codec/address"
    "github.com/cosmos/cosmos-sdk/codec/types"
    "github.com/cosmos/cosmos-sdk/std"
    sdk "github.com/cosmos/cosmos-sdk/types"
    "github.com/cosmos/cosmos-sdk/types/module"
    "github.com/cosmos/cosmos-sdk/x/auth"
    "github.com/cosmos/cosmos-sdk/x/auth/tx"
    authz "github.com/cosmos/cosmos-sdk/x/authz/module"
    "github.com/cosmos/cosmos-sdk/x/bank"
    "github.com/cosmos/cosmos-sdk/x/crisis"
    "github.com/cosmos/cosmos-sdk/x/distribution"
    "github.com/cosmos/cosmos-sdk/x/gov"
    "github.com/cosmos/cosmos-sdk/x/mint"
    "github.com/cosmos/cosmos-sdk/x/params"
    "github.com/cosmos/cosmos-sdk/x/slashing"
    "github.com/cosmos/cosmos-sdk/x/staking"
    "github.com/cosmos/ibc-go/modules/capability"
    ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee"
    "github.com/cosmos/ibc-go/v8/modules/apps/transfer"
    ibc "github.com/cosmos/ibc-go/v8/modules/core"
)

var ModuleBasics = []module.AppModuleBasic{
    auth.AppModuleBasic{},
    authz.AppModuleBasic{},
    bank.AppModuleBasic{},
    capability.AppModuleBasic{},
    gov.NewAppModuleBasic([]govclient.ProposalHandler{paramsclient.ProposalHandler}),
    crisis.AppModuleBasic{},
    distribution.AppModuleBasic{},
    feegrant.AppModuleBasic{},
    mint.AppModuleBasic{},
    params.AppModuleBasic{},
    slashing.AppModuleBasic{},
    staking.AppModuleBasic{},
    upgrade.AppModuleBasic{},
    transfer.AppModuleBasic{},
    ibc.AppModuleBasic{},
    ibcfee.AppModuleBasic{},
}

type Codec struct {
    InterfaceRegistry types.InterfaceRegistry
    Marshaler         codec.Codec
    TxConfig          client.TxConfig
    Amino             *codec.LegacyAmino
}

func MakeCodec(moduleBasics []module.AppModuleBasic, accBech32Prefix, valBech32Prefix string) Codec {
    modBasic := module.NewBasicManager(moduleBasics...)
    encodingConfig := MakeCodecConfig(accBech32Prefix, valBech32Prefix)
    std.RegisterLegacyAminoCodec(encodingConfig.Amino)
    std.RegisterInterfaces(encodingConfig.InterfaceRegistry)
    modBasic.RegisterLegacyAminoCodec(encodingConfig.Amino)
    modBasic.RegisterInterfaces(encodingConfig.InterfaceRegistry)
    return encodingConfig
}

func MakeCodecConfig(accBech32Prefix, valBech32Prefix string) Codec {
    interfaceRegistry, _ := types.NewInterfaceRegistryWithOptions(types.InterfaceRegistryOptions{
        ProtoFiles: proto.HybridResolver,
        SigningOptions: signing.Options{
            AddressCodec:          address.NewBech32Codec(accBech32Prefix),
            ValidatorAddressCodec: address.NewBech32Codec(valBech32Prefix),
        },
    })
    marshaler := codec.NewProtoCodec(interfaceRegistry)
    return Codec{
        InterfaceRegistry: interfaceRegistry,
        Marshaler:         marshaler,
        TxConfig:          tx.NewTxConfig(marshaler, tx.DefaultSignModes),
        Amino:             codec.NewLegacyAmino(),
    }
}
```

**Lines of code:** ~80 just for codec setup.

### 3. Client Implementation (client.go)

```go
package main

import (
    "context"
    "reflect"

    sdkerrors "cosmossdk.io/errors"
    abci "github.com/cometbft/cometbft/abci/types"
    rpcclient "github.com/cometbft/cometbft/rpc/client"
    rpchttp "github.com/cometbft/cometbft/rpc/client/http"
    "github.com/cosmos/cosmos-sdk/codec/types"
    legacyerrors "github.com/cosmos/cosmos-sdk/types/errors"
    grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
    "google.golang.org/grpc"
    "google.golang.org/grpc/encoding"
    "google.golang.org/grpc/encoding/proto"
    "google.golang.org/grpc/metadata"
)

type Client struct {
    ChainID       string
    Address       string
    RPCClient     rpcclient.Client
    AccountPrefix string
    Cdc           Codec
    Timeout       time.Duration
}

func NewClient(chainID, rpcAddr, accountPrefix string, timeout time.Duration) *Client {
    rpcClient, _ := NewRPCClient(rpcAddr, timeout)
    return &Client{
        ChainID:       chainID,
        Address:       rpcAddr,
        RPCClient:     rpcClient,
        AccountPrefix: accountPrefix,
        Cdc:           MakeCodec(ModuleBasics, accountPrefix, accountPrefix+"valoper"),
    }
}

// Invoke implements grpc.ClientConn - required for typed query clients
func (c *Client) Invoke(ctx context.Context, method string, req, reply interface{}, opts ...grpc.CallOption) error {
    if reflect.ValueOf(req).IsNil() {
        return sdkerrors.Wrap(legacyerrors.ErrInvalidRequest, "request cannot be nil")
    }

    inMd, _ := metadata.FromOutgoingContext(ctx)
    abciRes, outMd, err := c.RunGRPCQuery(ctx, method, req, inMd)
    if err != nil {
        return err
    }

    if err = protoCodec.Unmarshal(abciRes.Value, reply); err != nil {
        return err
    }

    // Handle call options, metadata, interface unpacking...
    // (~50 more lines)

    return nil
}
```

**Lines of code:** ~120 for the client wrapper.

### 4. Query Functions (query.go)

```go
package main

import (
    "context"

    sdktypes "github.com/cosmos/cosmos-sdk/types"
    banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
    transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
    chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
)

func (c *Client) QueryBalance(ctx context.Context, addr, denom string) (sdktypes.Coin, error) {
    qc := banktypes.NewQueryClient(c)

    req := &banktypes.QueryBalanceRequest{
        Address: addr,
        Denom:   denom,
    }

    res, err := qc.Balance(ctx, req, nil)
    if err != nil {
        return sdktypes.Coin{}, err
    }

    return *res.Balance, nil
}

func (c *Client) QueryEscrowAddress(ctx context.Context, portID, channelID string) (string, error) {
    qc := transfertypes.NewQueryClient(c)

    req := &transfertypes.QueryEscrowAddressRequest{
        PortId:    portID,
        ChannelId: channelID,
    }

    res, err := qc.EscrowAddress(ctx, req, nil)
    if err != nil {
        return "", err
    }

    return res.EscrowAddress, nil
}

func (c *Client) QueryChannel(ctx context.Context, channelID string) (*chantypes.IdentifiedChannel, error) {
    qc := chantypes.NewQueryClient(c)

    req := &chantypes.QueryChannelRequest{
        PortId:    "transfer",
        ChannelId: channelID,
    }

    resp, err := qc.Channel(ctx, req, nil)
    if err != nil {
        return nil, err
    }

    ch := chantypes.NewIdentifiedChannel("transfer", channelID, *resp.Channel)
    return &ch, nil
}

// ... 200+ more lines for other queries
```

### Total: Traditional Approach

| Component | Lines of Code | Files |
|-----------|---------------|-------|
| Codec setup | ~80 | 1 |
| Client wrapper | ~120 | 1 |
| Query functions | ~300 | 1 |
| Module registration | ~60 | 1 |
| **Total** | **~560** | **4** |

Plus **187 dependencies** and version compatibility headaches.

---

## The libyaci Approach

Here's the same functionality with libyaci:

### 1. Dependencies (go.mod)

```go
require (
    github.com/manifest-network/libyaci v0.1.0
)
```

**Result:** 2 direct dependencies (grpc + protobuf). That's it.

### 2. Complete Implementation

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/manifest-network/libyaci"
)

func main() {
    ctx := context.Background()

    // Connect - that's all the setup needed
    client, err := libyaci.Dial(ctx, "localhost:9090", libyaci.WithInsecure())
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Query balance
    balance, _ := queryBalance(client, "cosmos1abc...", "uatom")
    fmt.Printf("Balance: %s\n", balance)

    // Query escrow address
    escrow, _ := queryEscrowAddress(client, "transfer", "channel-0")
    fmt.Printf("Escrow: %s\n", escrow)

    // Query channel
    channel, _ := queryChannel(client, "channel-0")
    fmt.Printf("Channel: %+v\n", channel)
}

func queryBalance(client *libyaci.Client, addr, denom string) (map[string]interface{}, error) {
    resp, err := client.Invoke(
        "cosmos.bank.v1beta1.Query.Balance",
        []byte(fmt.Sprintf(`{"address":"%s","denom":"%s"}`, addr, denom)),
    )
    if err != nil {
        return nil, err
    }

    var result map[string]interface{}
    json.Unmarshal(resp, &result)
    return result, nil
}

func queryEscrowAddress(client *libyaci.Client, portID, channelID string) (string, error) {
    resp, err := client.Invoke(
        "ibc.applications.transfer.v1.Query.EscrowAddress",
        []byte(fmt.Sprintf(`{"port_id":"%s","channel_id":"%s"}`, portID, channelID)),
    )
    if err != nil {
        return "", err
    }

    var result struct {
        EscrowAddress string `json:"escrow_address"`
    }
    json.Unmarshal(resp, &result)
    return result.EscrowAddress, nil
}

func queryChannel(client *libyaci.Client, channelID string) (map[string]interface{}, error) {
    resp, err := client.Invoke(
        "ibc.core.channel.v1.Query.Channel",
        []byte(fmt.Sprintf(`{"port_id":"transfer","channel_id":"%s"}`, channelID)),
    )
    if err != nil {
        return nil, err
    }

    var result map[string]interface{}
    json.Unmarshal(resp, &result)
    return result, nil
}
```

### Or Even Simpler with Structs

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

func queryBalance(client *libyaci.Client, addr, denom string) (*BalanceResponse, error) {
    var resp BalanceResponse
    err := client.InvokeJSON(
        "cosmos.bank.v1beta1.Query.Balance",
        BalanceRequest{Address: addr, Denom: denom},
        &resp,
    )
    return &resp, err
}
```

### Total: libyaci Approach

| Component | Lines of Code | Files |
|-----------|---------------|-------|
| All functionality | ~60 | 1 |
| **Total** | **~60** | **1** |

With **2 dependencies**.

---

## Side-by-Side Comparison

### Query Balance

**Traditional:**
```go
import (
    sdktypes "github.com/cosmos/cosmos-sdk/types"
    banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (c *Client) QueryBalance(ctx context.Context, addr, denom string) (sdktypes.Coin, error) {
    qc := banktypes.NewQueryClient(c)
    req := &banktypes.QueryBalanceRequest{Address: addr, Denom: denom}
    res, err := qc.Balance(ctx, req, nil)
    if err != nil {
        return sdktypes.Coin{}, err
    }
    return *res.Balance, nil
}
```

**libyaci:**
```go
resp, err := client.Invoke(
    "cosmos.bank.v1beta1.Query.Balance",
    []byte(`{"address":"cosmos1...","denom":"uatom"}`),
)
```

### Query IBC Channel

**Traditional:**
```go
import (
    chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
)

func (c *Client) QueryChannel(ctx context.Context, channelID string) (*chantypes.IdentifiedChannel, error) {
    qc := chantypes.NewQueryClient(c)
    req := &chantypes.QueryChannelRequest{PortId: "transfer", ChannelId: channelID}
    resp, err := qc.Channel(ctx, req, nil)
    if err != nil {
        return nil, err
    }
    ch := chantypes.NewIdentifiedChannel("transfer", channelID, *resp.Channel)
    return &ch, nil
}
```

**libyaci:**
```go
resp, err := client.Invoke(
    "ibc.core.channel.v1.Query.Channel",
    []byte(`{"port_id":"transfer","channel_id":"channel-0"}`),
)
```

---

## Feature Comparison

| Feature | Traditional | libyaci |
|---------|-------------|---------|
| Dependencies | 150-200+ | 2 |
| Setup code | 200+ lines | 3 lines |
| Per-query boilerplate | High | None |
| Type safety | Compile-time | Runtime (JSON) |
| New chain support | Recompile | Zero changes |
| SDK version changes | Breaking | Transparent |
| Proto file changes | Recompile | Automatic |
| Custom modules | Manual registration | Automatic |
| Any type handling | Manual unpacking | Automatic |
| Build time | Slow | Fast |
| Binary size | Large | Small |

---

## When to Use Each

### Use Traditional Approach When:
- You need compile-time type safety
- You're building a validator or full node
- Performance is absolutely critical (nanoseconds matter)
- You're already deep in the Cosmos SDK ecosystem

### Use libyaci When:
- Building CLI tools, explorers, or indexers
- Querying multiple different chains
- Prototyping or experimenting
- You want minimal dependencies
- You need to support chains with custom modules
- You're tired of dependency hell

---

## Migration Path

Already have traditional code? Migration is straightforward:

```go
// Before: scattered across multiple files
qc := banktypes.NewQueryClient(c)
req := &banktypes.QueryBalanceRequest{Address: addr, Denom: denom}
res, err := qc.Balance(ctx, req, nil)

// After: single call
resp, err := client.Invoke("cosmos.bank.v1beta1.Query.Balance",
    []byte(fmt.Sprintf(`{"address":"%s","denom":"%s"}`, addr, denom)))
```

You can migrate incrementally - libyaci works alongside traditional clients.

---

## Conclusion

For most use cases outside of core chain development, libyaci provides:

- **10x less code** to write and maintain
- **100x fewer dependencies** to manage
- **Zero recompilation** when protos change
- **Automatic support** for any chain with gRPC reflection

The traditional approach has its place, but for tooling, indexers, and explorers, libyaci offers a dramatically simpler alternative.
