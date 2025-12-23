# libyaci

A dynamic gRPC client for Go that uses server reflection to invoke methods without precompiled protobuf stubs. Designed for Cosmos SDK blockchains but works with any reflection-enabled gRPC server.

## Features

- Dynamic gRPC invocation without compiled protobuf stubs
- Server reflection for automatic service discovery
- On-demand type resolution for `Any` fields
- Fallback registry for deprecated module types (e.g., Cosmos Hub liquidity)
- Thread-safe concurrent access
- Raw protobuf transaction decoding (Cosmos SDK)
- Comprehensive query methods for all Cosmos SDK modules

## Installation

```bash
go get github.com/Cordtus/libyaci
```

## Requirements

- Go 1.24+
- gRPC server with [reflection](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md) enabled
  - Supports both `grpc.reflection.v1` and `grpc.reflection.v1alpha` (automatic fallback)

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

## Configuration Options

```go
client, err := libyaci.Dial(ctx, "grpc.example.com:443",
    libyaci.WithInsecure(),                         // Disable TLS
    libyaci.WithMaxRetries(5),                      // Retry count (default: 3)
    libyaci.WithMaxRecvMsgSize(16 * 1024 * 1024),   // Max message size (default: 4MB)
    libyaci.WithDialTimeout(30 * time.Second),      // Connection timeout (default: no timeout)
    libyaci.WithDialOptions(grpc.WithPerRPCCredentials(creds)), // Custom gRPC options
    libyaci.WithDeprecatedCosmosModules(),          // Enable fallback for deprecated modules
)
```

## Fallback Registry (Deprecated Modules)

Some Cosmos SDK modules have been deprecated and removed from chains, but their transaction data still exists in historical blocks. Server reflection cannot provide descriptors for these removed modules, causing decode failures.

The fallback registry provides pre-compiled proto descriptors for these deprecated types, enabling successful decoding of historical transactions.

### Enabling Deprecated Module Support

```go
// Simple: Use global fallback with deprecated Cosmos modules
client, err := libyaci.Dial(ctx, endpoint,
    libyaci.WithDeprecatedCosmosModules(),
)
```

### Currently Supported Deprecated Modules

| Module | Package | Description |
|--------|---------|-------------|
| Liquidity (Gravity DEX) | `tendermint.liquidity.v1beta1` | AMM/DEX module removed from Cosmos Hub |

Supported message types for the liquidity module:
- `MsgCreatePool` / `MsgCreatePoolResponse`
- `MsgDepositWithinBatch` / `MsgDepositWithinBatchResponse`
- `MsgWithdrawWithinBatch` / `MsgWithdrawWithinBatchResponse`
- `MsgSwapWithinBatch` / `MsgSwapWithinBatchResponse`

### Advanced: Custom Fallback Registry

```go
// Create a custom fallback registry
fb := libyaci.NewFallbackRegistry()

// Register your own deprecated types
fb.RegisterFileDescriptor(myDeprecatedProto)

// Use the custom registry
client, err := libyaci.Dial(ctx, endpoint,
    libyaci.WithFallbackRegistry(fb),
)
```

### Shared Global Fallback

Multiple clients can share the same fallback registry:

```go
// Register once at startup
libyaci.GlobalFallback().RegisterDeprecatedCosmosModules()

// All clients using WithGlobalFallback() share the same descriptors
client1, _ := libyaci.Dial(ctx, endpoint1, libyaci.WithGlobalFallback())
client2, _ := libyaci.Dial(ctx, endpoint2, libyaci.WithGlobalFallback())
```

### Fallback Options

| Option | Description |
|--------|-------------|
| `WithDeprecatedCosmosModules()` | Register deprecated Cosmos SDK modules (liquidity, etc.) to the global fallback |
| `WithGlobalFallback()` | Use the shared global fallback registry |
| `WithFallbackRegistry(fb)` | Use a custom fallback registry |

## Core Methods

### Generic Invocation

| Method | Description |
|--------|-------------|
| `Invoke(method string, request []byte) ([]byte, error)` | Call any gRPC method with JSON request/response |
| `InvokeRaw(method string, request []byte) (*dynamicpb.Message, error)` | Call method and return raw protobuf message |
| `InvokeWithRetry(method string, request []byte, maxRetries uint) ([]byte, error)` | Call with custom retry count |

### Service Discovery

| Method | Description |
|--------|-------------|
| `ListServices() []string` | List all available gRPC services |
| `ListMethods(service string) ([]string, error)` | List methods for a service |
| `DescribeMethod(method string) (input, output string, error)` | Get method input/output types |

### Utilities

| Method | Description |
|--------|-------------|
| `ExtractField(method string, request []byte, field string) (any, error)` | Call method and extract specific field |
| `DecodeTxBytes(txBytes []byte) ([]byte, error)` | Decode raw protobuf transaction bytes to JSON |
| `Resolver() *Resolver` | Get underlying type resolver |
| `Conn() *grpc.ClientConn` | Get underlying gRPC connection |
| `Close() error` | Close client connection |

---

## Cosmos SDK Methods Index

Complete reference of all Cosmos SDK convenience methods organized by module.

### Tendermint/CometBFT Service

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetLatestBlock() (*BlockResponse, error)` | Get latest block | `cosmos.base.tendermint.v1beta1.Service.GetLatestBlock` |
| `GetLatestBlockHeight() (int64, error)` | Get latest block height | - |
| `GetBlockByHeight(height int64) (*BlockResponse, error)` | Get block at height | `cosmos.base.tendermint.v1beta1.Service.GetBlockByHeight` |
| `GetEarliestBlockHeight() (int64, error)` | Find earliest available block (handles pruning) | - |
| `GetNodeInfo() (*NodeInfoResponse, error)` | Get node info | `cosmos.base.tendermint.v1beta1.Service.GetNodeInfo` |
| `GetChainID() (string, error)` | Get chain ID | - |
| `GetSyncing() (bool, error)` | Check if node is syncing | `cosmos.base.tendermint.v1beta1.Service.GetSyncing` |
| `GetLatestValidatorSet() (*ValidatorSetResponse, error)` | Get latest validator set | `cosmos.base.tendermint.v1beta1.Service.GetLatestValidatorSet` |
| `GetValidatorSetByHeight(height int64) (*ValidatorSetResponse, error)` | Get validator set at height | `cosmos.base.tendermint.v1beta1.Service.GetValidatorSetByHeight` |

### Transaction Service

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetTx(hash string) (*TxResponse, error)` | Get transaction by hash | `cosmos.tx.v1beta1.Service.GetTx` |
| `GetTxsByHeight(height int64) ([]byte, error)` | Get transactions at height (raw JSON) | `cosmos.tx.v1beta1.Service.GetTxsEvent` |
| `GetTxsByHeightParsed(height int64) (*TxsEventResponse, error)` | Get transactions at height (parsed) | `cosmos.tx.v1beta1.Service.GetTxsEvent` |
| `GetBlockWithTxs(height int64) (*BlockWithTxsResponse, error)` | Get block with full transactions | `cosmos.tx.v1beta1.Service.GetBlockWithTxs` |

### Auth Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetAccount(address string) (*AccountResponse, error)` | Get account by address | `cosmos.auth.v1beta1.Query.Account` |
| `GetAccounts(paginationKey string) (*AccountsResponse, error)` | List all accounts | `cosmos.auth.v1beta1.Query.Accounts` |
| `GetAccountInfo(address string) (*AccountInfoResponse, error)` | Get account info | `cosmos.auth.v1beta1.Query.AccountInfo` |
| `GetModuleAccounts() (map[string]string, error)` | Get all module accounts (address -> name) | `cosmos.auth.v1beta1.Query.ModuleAccounts` |
| `GetModuleAccountByName(name string) (*AccountResponse, error)` | Get module account by name | `cosmos.auth.v1beta1.Query.ModuleAccountByName` |
| `GetBech32Prefix() (string, error)` | Get bech32 address prefix | `cosmos.auth.v1beta1.Query.Bech32Prefix` |
| `GetAuthParams() (*AuthParamsResponse, error)` | Get auth module params | `cosmos.auth.v1beta1.Query.Params` |

### Authz Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetGrants(granter, grantee string) (*GrantsResponse, error)` | Get grants between accounts | `cosmos.authz.v1beta1.Query.Grants` |
| `GetGranterGrants(granter string) (*GrantsResponse, error)` | Get all grants by granter | `cosmos.authz.v1beta1.Query.GranterGrants` |
| `GetGranteeGrants(grantee string) (*GrantsResponse, error)` | Get all grants to grantee | `cosmos.authz.v1beta1.Query.GranteeGrants` |

### Bank Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetBalance(address, denom string) (*BalanceResponse, error)` | Get balance of specific denom | `cosmos.bank.v1beta1.Query.Balance` |
| `GetAllBalances(address string) (*AllBalancesResponse, error)` | Get all balances for address | `cosmos.bank.v1beta1.Query.AllBalances` |
| `GetSpendableBalances(address string) (*SpendableBalancesResponse, error)` | Get spendable balances | `cosmos.bank.v1beta1.Query.SpendableBalances` |
| `GetTotalSupply() (*TotalSupplyResponse, error)` | Get total supply of all coins | `cosmos.bank.v1beta1.Query.TotalSupply` |
| `GetSupplyOf(denom string) (*SupplyOfResponse, error)` | Get supply of specific denom | `cosmos.bank.v1beta1.Query.SupplyOf` |
| `GetDenomMetadata(denom string) (*DenomMetadataResponse, error)` | Get denom metadata | `cosmos.bank.v1beta1.Query.DenomMetadata` |
| `GetDenomsMetadata() (*DenomsMetadataResponse, error)` | Get all denom metadata | `cosmos.bank.v1beta1.Query.DenomsMetadata` |
| `GetDenomOwners(denom string) (*DenomOwnersResponse, error)` | Get all owners of a denom | `cosmos.bank.v1beta1.Query.DenomOwners` |
| `GetBankParams() (*BankParamsResponse, error)` | Get bank module params | `cosmos.bank.v1beta1.Query.Params` |

### Staking Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetValidators(status string) (*ValidatorsResponse, error)` | Get validators by status | `cosmos.staking.v1beta1.Query.Validators` |
| `GetAllValidators() (map[string]string, error)` | Get all validators (handles pagination) | - |
| `GetValidator(validatorAddr string) (*ValidatorResponse, error)` | Get single validator | `cosmos.staking.v1beta1.Query.Validator` |
| `GetStakingPool() (*StakingPoolResponse, error)` | Get staking pool info | `cosmos.staking.v1beta1.Query.Pool` |
| `GetStakingParams() (*StakingParamsResponse, error)` | Get staking params | `cosmos.staking.v1beta1.Query.Params` |
| `GetBondDenom() (string, error)` | Get bond denom | - |
| `GetDelegation(delegator, validator string) (*DelegationResponse, error)` | Get specific delegation | `cosmos.staking.v1beta1.Query.Delegation` |
| `GetDelegations(delegatorAddr string) (*DelegationsResponse, error)` | Get delegator's delegations | `cosmos.staking.v1beta1.Query.DelegatorDelegations` |
| `GetValidatorDelegations(validatorAddr string) (*DelegationsResponse, error)` | Get validator's delegations | `cosmos.staking.v1beta1.Query.ValidatorDelegations` |
| `GetDelegatorValidators(delegatorAddr string) (*ValidatorsResponse, error)` | Get delegator's validators | `cosmos.staking.v1beta1.Query.DelegatorValidators` |
| `GetUnbondingDelegation(delegator, validator string) (*UnbondingDelegationResponse, error)` | Get unbonding delegation | `cosmos.staking.v1beta1.Query.UnbondingDelegation` |
| `GetDelegatorUnbondingDelegations(delegatorAddr string) (*UnbondingDelegationsResponse, error)` | Get all unbonding delegations | `cosmos.staking.v1beta1.Query.DelegatorUnbondingDelegations` |
| `GetRedelegations(delegatorAddr string) (*RedelegationsResponse, error)` | Get redelegations | `cosmos.staking.v1beta1.Query.Redelegations` |

### Distribution Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetCommunityPool() (*CommunityPoolResponse, error)` | Get community pool balance | `cosmos.distribution.v1beta1.Query.CommunityPool` |
| `GetDelegationRewards(delegator, validator string) (*DelegationRewardsResponse, error)` | Get delegation rewards | `cosmos.distribution.v1beta1.Query.DelegationRewards` |
| `GetDelegationTotalRewards(delegatorAddr string) (*DelegationTotalRewardsResponse, error)` | Get total delegation rewards | `cosmos.distribution.v1beta1.Query.DelegationTotalRewards` |
| `GetDelegatorWithdrawAddress(delegatorAddr string) (string, error)` | Get withdraw address | `cosmos.distribution.v1beta1.Query.DelegatorWithdrawAddress` |
| `GetValidatorCommission(validatorAddr string) (*ValidatorCommissionResponse, error)` | Get validator commission | `cosmos.distribution.v1beta1.Query.ValidatorCommission` |
| `GetValidatorOutstandingRewards(validatorAddr string) (*ValidatorOutstandingRewardsResponse, error)` | Get outstanding rewards | `cosmos.distribution.v1beta1.Query.ValidatorOutstandingRewards` |
| `GetDistributionParams() (*DistributionParamsResponse, error)` | Get distribution params | `cosmos.distribution.v1beta1.Query.Params` |

### Governance Module (v1)

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetProposals(status string) (*ProposalsResponse, error)` | Get proposals by status | `cosmos.gov.v1.Query.Proposals` |
| `GetProposal(proposalID uint64) (*ProposalResponse, error)` | Get single proposal | `cosmos.gov.v1.Query.Proposal` |
| `GetProposalVotes(proposalID uint64) (*VotesResponse, error)` | Get proposal votes | `cosmos.gov.v1.Query.Votes` |
| `GetProposalDeposits(proposalID uint64) (*DepositsResponse, error)` | Get proposal deposits | `cosmos.gov.v1.Query.Deposits` |
| `GetTallyResult(proposalID uint64) (*TallyResultResponse, error)` | Get proposal tally | `cosmos.gov.v1.Query.TallyResult` |
| `GetGovParams() (*GovParamsResponse, error)` | Get governance params | `cosmos.gov.v1.Query.Params` |

### Mint Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetInflation() (string, error)` | Get current inflation rate | `cosmos.mint.v1beta1.Query.Inflation` |
| `GetAnnualProvisions() (string, error)` | Get annual provisions | `cosmos.mint.v1beta1.Query.AnnualProvisions` |
| `GetMintParams() (*MintParamsResponse, error)` | Get mint params | `cosmos.mint.v1beta1.Query.Params` |

### Slashing Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetSigningInfo(consAddr string) (*SigningInfoResponse, error)` | Get validator signing info | `cosmos.slashing.v1beta1.Query.SigningInfo` |
| `GetSigningInfos() (*SigningInfosResponse, error)` | Get all signing infos | `cosmos.slashing.v1beta1.Query.SigningInfos` |
| `GetSlashingParams() (*SlashingParamsResponse, error)` | Get slashing params | `cosmos.slashing.v1beta1.Query.Params` |

### Evidence Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetEvidence(hash string) (*EvidenceResponse, error)` | Get evidence by hash | `cosmos.evidence.v1beta1.Query.Evidence` |
| `GetAllEvidence() (*AllEvidenceResponse, error)` | Get all evidence | `cosmos.evidence.v1beta1.Query.AllEvidence` |

### Feegrant Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetAllowance(granter, grantee string) (*AllowanceResponse, error)` | Get fee allowance | `cosmos.feegrant.v1beta1.Query.Allowance` |
| `GetAllowances(grantee string) (*AllowancesResponse, error)` | Get all allowances for grantee | `cosmos.feegrant.v1beta1.Query.Allowances` |
| `GetAllowancesByGranter(granter string) (*AllowancesResponse, error)` | Get all allowances by granter | `cosmos.feegrant.v1beta1.Query.AllowancesByGranter` |

### Upgrade Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetCurrentPlan() (*CurrentPlanResponse, error)` | Get current upgrade plan | `cosmos.upgrade.v1beta1.Query.CurrentPlan` |
| `GetAppliedPlan(name string) (*AppliedPlanResponse, error)` | Get applied plan height | `cosmos.upgrade.v1beta1.Query.AppliedPlan` |
| `GetModuleVersions() (*ModuleVersionsResponse, error)` | Get module versions | `cosmos.upgrade.v1beta1.Query.ModuleVersions` |

### IBC Client Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetIBCClientStates() (*IBCClientStatesResponse, error)` | Get all IBC client states | `ibc.core.client.v1.Query.ClientStates` |
| `GetIBCClientState(clientID string) (*IBCClientStateResponse, error)` | Get IBC client state | `ibc.core.client.v1.Query.ClientState` |

### IBC Connection Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetIBCConnections() (*IBCConnectionsResponse, error)` | Get all IBC connections | `ibc.core.connection.v1.Query.Connections` |
| `GetIBCConnection(connectionID string) (*IBCConnectionResponse, error)` | Get IBC connection | `ibc.core.connection.v1.Query.Connection` |
| `GetIBCClientConnections(clientID string) ([]string, error)` | Get connections for client | `ibc.core.connection.v1.Query.ClientConnections` |

### IBC Channel Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetIBCChannels() (*IBCChannelsResponse, error)` | Get all IBC channels | `ibc.core.channel.v1.Query.Channels` |
| `GetIBCChannel(portID, channelID string) (*IBCChannelResponse, error)` | Get IBC channel | `ibc.core.channel.v1.Query.Channel` |
| `GetIBCConnectionChannels(connectionID string) (*IBCChannelsResponse, error)` | Get channels for connection | `ibc.core.channel.v1.Query.ConnectionChannels` |

### IBC Transfer Module

| Method | Description | gRPC Method |
|--------|-------------|-------------|
| `GetIBCDenomTrace(hash string) (*IBCDenomTraceResponse, error)` | Get denom trace by hash | `ibc.applications.transfer.v1.Query.Denom` |
| `GetIBCDenomTraces() (*IBCDenomTracesResponse, error)` | Get all denom traces | `ibc.applications.transfer.v1.Query.Denoms` |
| `GetIBCDenomHash(trace string) (string, error)` | Calculate denom hash | `ibc.applications.transfer.v1.Query.DenomHash` |
| `GetIBCEscrowAddress(portID, channelID string) (string, error)` | Get escrow address | `ibc.applications.transfer.v1.Query.EscrowAddress` |
| `GetIBCTotalEscrowForDenom(denom string) (*IBCTotalEscrowResponse, error)` | Get total escrowed amount | `ibc.applications.transfer.v1.Query.TotalEscrowForDenom` |
| `GetIBCTransferParams() (*IBCTransferParamsResponse, error)` | Get IBC transfer params | `ibc.applications.transfer.v1.Query.Params` |

---

## gRPC Method Reference

For reference, here are the underlying gRPC method paths used by the convenience methods. These can also be called directly via `client.Invoke()`.

### Tendermint/CometBFT Service
```
cosmos.base.tendermint.v1beta1.Service.GetLatestBlock
cosmos.base.tendermint.v1beta1.Service.GetBlockByHeight
cosmos.base.tendermint.v1beta1.Service.GetNodeInfo
cosmos.base.tendermint.v1beta1.Service.GetSyncing
cosmos.base.tendermint.v1beta1.Service.GetLatestValidatorSet
cosmos.base.tendermint.v1beta1.Service.GetValidatorSetByHeight
```

### Transaction Service
```
cosmos.tx.v1beta1.Service.GetTx
cosmos.tx.v1beta1.Service.GetTxsEvent
cosmos.tx.v1beta1.Service.GetBlockWithTxs
cosmos.tx.v1beta1.Service.BroadcastTx
cosmos.tx.v1beta1.Service.Simulate
```

### Auth Module
```
cosmos.auth.v1beta1.Query.Account
cosmos.auth.v1beta1.Query.Accounts
cosmos.auth.v1beta1.Query.AccountInfo
cosmos.auth.v1beta1.Query.ModuleAccounts
cosmos.auth.v1beta1.Query.ModuleAccountByName
cosmos.auth.v1beta1.Query.Bech32Prefix
cosmos.auth.v1beta1.Query.Params
```

### Authz Module
```
cosmos.authz.v1beta1.Query.Grants
cosmos.authz.v1beta1.Query.GranterGrants
cosmos.authz.v1beta1.Query.GranteeGrants
```

### Bank Module
```
cosmos.bank.v1beta1.Query.Balance
cosmos.bank.v1beta1.Query.AllBalances
cosmos.bank.v1beta1.Query.SpendableBalances
cosmos.bank.v1beta1.Query.TotalSupply
cosmos.bank.v1beta1.Query.SupplyOf
cosmos.bank.v1beta1.Query.DenomMetadata
cosmos.bank.v1beta1.Query.DenomsMetadata
cosmos.bank.v1beta1.Query.DenomOwners
cosmos.bank.v1beta1.Query.Params
```

### Staking Module
```
cosmos.staking.v1beta1.Query.Validators
cosmos.staking.v1beta1.Query.Validator
cosmos.staking.v1beta1.Query.Pool
cosmos.staking.v1beta1.Query.Params
cosmos.staking.v1beta1.Query.Delegation
cosmos.staking.v1beta1.Query.DelegatorDelegations
cosmos.staking.v1beta1.Query.DelegatorUnbondingDelegations
cosmos.staking.v1beta1.Query.DelegatorValidators
cosmos.staking.v1beta1.Query.ValidatorDelegations
cosmos.staking.v1beta1.Query.UnbondingDelegation
cosmos.staking.v1beta1.Query.Redelegations
cosmos.staking.v1beta1.Query.HistoricalInfo
```

### Distribution Module
```
cosmos.distribution.v1beta1.Query.CommunityPool
cosmos.distribution.v1beta1.Query.DelegationRewards
cosmos.distribution.v1beta1.Query.DelegationTotalRewards
cosmos.distribution.v1beta1.Query.DelegatorWithdrawAddress
cosmos.distribution.v1beta1.Query.ValidatorCommission
cosmos.distribution.v1beta1.Query.ValidatorOutstandingRewards
cosmos.distribution.v1beta1.Query.ValidatorSlashes
cosmos.distribution.v1beta1.Query.Params
```

### Governance Module (v1)
```
cosmos.gov.v1.Query.Proposals
cosmos.gov.v1.Query.Proposal
cosmos.gov.v1.Query.Votes
cosmos.gov.v1.Query.Deposits
cosmos.gov.v1.Query.TallyResult
cosmos.gov.v1.Query.Params
```

### Mint Module
```
cosmos.mint.v1beta1.Query.Inflation
cosmos.mint.v1beta1.Query.AnnualProvisions
cosmos.mint.v1beta1.Query.Params
```

### Slashing Module
```
cosmos.slashing.v1beta1.Query.SigningInfo
cosmos.slashing.v1beta1.Query.SigningInfos
cosmos.slashing.v1beta1.Query.Params
```

### Evidence Module
```
cosmos.evidence.v1beta1.Query.Evidence
cosmos.evidence.v1beta1.Query.AllEvidence
```

### Feegrant Module
```
cosmos.feegrant.v1beta1.Query.Allowance
cosmos.feegrant.v1beta1.Query.Allowances
cosmos.feegrant.v1beta1.Query.AllowancesByGranter
```

### Upgrade Module
```
cosmos.upgrade.v1beta1.Query.CurrentPlan
cosmos.upgrade.v1beta1.Query.AppliedPlan
cosmos.upgrade.v1beta1.Query.ModuleVersions
```

### IBC Core - Client
```
ibc.core.client.v1.Query.ClientStates
ibc.core.client.v1.Query.ClientState
ibc.core.client.v1.Query.ConsensusStates
ibc.core.client.v1.Query.ClientParams
```

### IBC Core - Connection
```
ibc.core.connection.v1.Query.Connections
ibc.core.connection.v1.Query.Connection
ibc.core.connection.v1.Query.ClientConnections
ibc.core.connection.v1.Query.ConnectionParams
```

### IBC Core - Channel
```
ibc.core.channel.v1.Query.Channels
ibc.core.channel.v1.Query.Channel
ibc.core.channel.v1.Query.ConnectionChannels
ibc.core.channel.v1.Query.PacketCommitments
ibc.core.channel.v1.Query.PacketAcknowledgements
ibc.core.channel.v1.Query.UnreceivedPackets
ibc.core.channel.v1.Query.UnreceivedAcks
```

### IBC Applications - Transfer
```
ibc.applications.transfer.v1.Query.Denom
ibc.applications.transfer.v1.Query.Denoms
ibc.applications.transfer.v1.Query.DenomHash
ibc.applications.transfer.v1.Query.EscrowAddress
ibc.applications.transfer.v1.Query.TotalEscrowForDenom
ibc.applications.transfer.v1.Query.Params
```

---

## Thread Safety

The client is safe for concurrent use from multiple goroutines. The internal type resolver uses proper synchronization to handle concurrent symbol lookups, ensuring that multiple goroutines fetching the same type will coordinate correctly without race conditions.

## How It Works

1. Connects to the gRPC server
2. Fetches all service descriptors via the reflection API
   - Tries `grpc.reflection.v1` first, falls back to `grpc.reflection.v1alpha` for older servers
   - Caches the detected version per connection for performance
3. Builds a local proto registry from the descriptors
4. Creates dynamic request messages from JSON input
5. Invokes methods and marshals responses back to JSON
6. Fetches additional descriptors on-demand for unknown `Any` types
7. Falls back to pre-compiled descriptors for deprecated types not in reflection

## Example CLI

```bash
go build -o grpc-cli ./example

./grpc-cli -addr localhost:9090 -insecure -list
./grpc-cli -addr localhost:9090 -insecure -method "cosmos.bank.v1beta1.Query.TotalSupply" -request "{}"
```

## License

MIT License - see [LICENSE](LICENSE) for details.
