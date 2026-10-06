# Repository Guidelines

## Project Structure & Module Organization

`libyaci` is a Go module for dynamic gRPC queries against reflection-enabled Cosmos SDK chains. Core package files live at the repository root: `client.go` handles dialing and invocation, `reflection.go` fetches descriptors, `catalog.go` indexes reflected capabilities, `method.go`, `request.go`, and `response.go` expose descriptor-backed calls, `fallback.go` and `protodir.go` provide secondary proto fallback support, and `cosmos.go` contains Cosmos convenience methods. The `signing/` subpackage adds transaction signing and broadcasting. The `alpnfix/` subpackage contains the grpc-go ALPN workaround. Root-level `*_test.go` files hold package tests and `mock_test.go` provides the bufconn reflection test server. Runnable samples are under `examples/`, including `examples/cli` and `examples/explorer`.

## Documentation

- [`README.md`](README.md) is the human-facing contract: usage, options, and the Cosmos helper reference.
- [`dev-docs/`](dev-docs/README.md) holds project-level knowledge that spans files or operators: architecture, the deprecated-proto runbook, and the work log. Read the relevant doc before planning changes in its area and update it once the work is verified.
- [`docs/repository-analysis/`](docs/repository-analysis/README.md) is a historical automated scan (2026-01-27), not maintained; verify any claim against the code.
- Source-local contracts stay in Go comments; do not duplicate them here or in `dev-docs/`.

## Build, Test, and Development Commands

- `go test ./...`: run all unit tests.
- `go test -race -coverprofile=coverage.out ./...`: match CI race and coverage testing.
- `go build ./...`: compile the library and all packages.
- `go vet ./...`: run standard Go static checks.
- `staticcheck ./...`: run the same external linter used by CI.
- `go build -o grpc-cli ./examples/cli`: build the example CLI.
- `cd examples/explorer && go build -o explorer .`: build the explorer demo.

Use Go 1.24 or newer. Run `go mod tidy` after dependency changes and confirm `go.mod` and `go.sum` contain only intentional edits.

## Coding Style & Naming Conventions

Format Go code with `gofmt`; keep imports grouped by the standard formatter. Prefer small functions, package-private helpers for internal behavior, and exported identifiers only for public API. Public functions, options, and response types should keep the existing concise Go naming style, such as `Dial`, `InvokeRaw`, `WithProtoDir`, and `BlockResponse`. Include comments for exported symbols and for non-obvious descriptor or protobuf recovery logic.

## Testing Guidelines

Use Go's standard `testing` package. Name tests `TestXxx` and keep table-driven cases where input/output behavior varies. Prefer the existing bufconn mock reflection server for deterministic unit tests. Integration tests that require a live chain endpoint should be skipped unless `LIBYACI_TEST_ENDPOINT` is set, for example:

```bash
LIBYACI_TEST_ENDPOINT=cosmos-grpc.publicnode.com:443 go test -v ./...
```

## Architecture

Core components:

- **Client** (`client.go`): entry point. `Dial` connects, fetches descriptors via reflection, and builds the registry. Exposes `Invoke*`, `InvokeRaw`, `Method`, `NewMessage`, and accessors (`Catalog`, `Resolver`, `ProtoDir`, `SkippedFiles`). `Dial` uses `grpc.NewClient` (lazy connection); `WithDialTimeout` bounds descriptor fetching, not the TCP dial.
- **Resolver** (`client.go`): thread-safe type resolution implementing `protoregistry.MessageTypeResolver` and `ExtensionTypeResolver`. Caches reflected descriptors, fetches missing `Any` types on demand (coordinated by an `inProgress` singleflight map), and falls back to local protos. `FindExtensionByName`/`FindExtensionByNumber` walk extension declarations explicitly because `protoregistry.Files` does not resolve them.
- **Reflection** (`reflection.go`): `fetchDescriptorSnapshot` lists services and recursively fetches file descriptors; `buildFileDescriptorSetReport` topologically sorts them and registers best-effort, returning the names of files that could not be registered (surfaced via `Client.SkippedFiles()`). `applyDescriptorPatches` rewrites `cosmos.base.abci.v1beta1.TxResponse.raw_log` from string to bytes (see cosmos-sdk#22414), so JSON output for that field is base64.
- **Catalog** (`catalog.go`): immutable snapshot of advertised services, methods, and messages, plus `ChainInfo`. Only services returned by reflection's `ListServices` are callable; dependency-only services are excluded.
- **Method/Request/Response** (`method.go`, `request.go`, `response.go`): descriptor-backed dynamic request/response builders. `Request.Set`/`SetPath`/`SetMap` accept Go values and resolve proto or JSON field names.
- **Pagination** (`pagination.go`): `Method.EachPage` follows `pagination.key`/`next_key` for standard Cosmos queries and aborts if the server repeats a key.
- **Streaming** (`stream.go`): server-, client-, and bidirectional-streaming RPCs via `Method.ServerStream`/`ClientStream`/`BidiStream` and `Client` equivalents.
- **Fallback** (`fallback.go`, `protodir.go`): `FallbackRegistry` plus `ProtoDir`, which compiles local `.proto` files with `protocompile` on first miss. A configured `WithProtoDir` is attached to a clone so it never mutates the shared/global registry.
- **Cosmos** (`cosmos.go`): convenience helpers over reflected Cosmos SDK query methods.
- **Signing** (`signing/`): `Signer` interface plus an in-process secp256k1/ethsecp256k1 signer (raw key or BIP39 mnemonic). `BuildAndSign` assembles `SIGN_MODE_DIRECT` transactions using reflection-resolved tx scaffolding, then `Broadcast`/`Simulate` submit them. See [`dev-docs/architecture/signing.md`](dev-docs/architecture/signing.md).
- **ALPN fix** (`alpnfix/`): import `_ "github.com/Cordtus/libyaci/alpnfix"` before gRPC imports to disable ALPN enforcement.

Data flow: `Dial` → reflection snapshot → `buildFileDescriptorSetReport` → dynamic request → gRPC invoke → protojson marshal; unknown `Any` types are fetched on demand; deprecated types resolve through `WithProtoDir`.

Method names use `package.Service.Method`, e.g. `cosmos.bank.v1beta1.Query.Balance`.

Key patterns: dynamicpb for runtime messages; conditional retry (only `Unavailable`, `DeadlineExceeded`, `ResourceExhausted`, `Aborted`); per-call timeout via `WithDefaultTimeout`/`InvokeWithTimeout`; unary and streaming RPCs (server/client/bidi via `stream.go`). Core dependencies are intentionally minimal: `google.golang.org/grpc`, `google.golang.org/protobuf`, `github.com/bufbuild/protocompile`; the optional `signing/` subpackage adds `btcec/v2`, `btcutil/bech32`, `cosmos/go-bip39`, and `golang.org/x/crypto`. No Cosmos SDK dependency in either case.

## Local Proto Fallback

Server reflection only exposes what the node currently serves. Historical transactions may reference message types from removed modules (for example `tendermint.liquidity.v1beta1`), so decoding them requires local definitions:

```go
client, err := libyaci.Dial(ctx, addr, libyaci.WithProtoDir("./protos"))
```

Proto files mirror their package path (for example `protos/tendermint/liquidity/v1beta1/tx.proto`). Imports within the directory and `google/protobuf/*` are resolved automatically. The directory is validated eagerly by `Dial`; compilation stays lazy. Resolution order: primary reflected registry → on-demand reflection → local proto directory → `TypeNotFoundError` with a file-layout hint. UTF-8 recovery also works for local-proto types.

## Repository Analysis Docs

`docs/repository-analysis/` is a historical automated scan from 2026-01-27. It is not current; verify any claim there against the code before acting on it.

## gRPC Method Reference

Primary paths used by the Cosmos helpers:

```
# Tendermint
cosmos.base.tendermint.v1beta1.Service.{GetLatestBlock,GetBlockByHeight,GetNodeInfo,GetSyncing,GetLatestValidatorSet,GetValidatorSetByHeight,GetBlockResults,GetLatestBlockResults}
# Transactions
cosmos.tx.v1beta1.Service.{GetTx,GetTxsEvent,GetBlockWithTxs}
# Auth/Authz
cosmos.auth.v1beta1.Query.{Account,Accounts,AccountInfo,ModuleAccounts,ModuleAccountByName,Bech32Prefix,Params}
cosmos.authz.v1beta1.Query.{Grants,GranterGrants,GranteeGrants}
# Bank
cosmos.bank.v1beta1.Query.{Balance,AllBalances,SpendableBalances,TotalSupply,SupplyOf,DenomMetadata,DenomsMetadata,DenomOwners,Params}
# Staking
cosmos.staking.v1beta1.Query.{Validators,Validator,Pool,Params,Delegation,DelegatorDelegations,DelegatorUnbondingDelegations,DelegatorValidators,ValidatorDelegations,UnbondingDelegation,Redelegations}
# Distribution
cosmos.distribution.v1beta1.Query.{CommunityPool,DelegationRewards,DelegationTotalRewards,DelegatorWithdrawAddress,ValidatorCommission,ValidatorOutstandingRewards,Params}
# Gov (v1)
cosmos.gov.v1.Query.{Proposals,Proposal,Votes,Deposits,TallyResult,Params}
# Mint/Slashing/Evidence/Feegrant/Upgrade
cosmos.mint.v1beta1.Query.{Inflation,AnnualProvisions,Params}
cosmos.slashing.v1beta1.Query.{SigningInfo,SigningInfos,Params}
cosmos.evidence.v1beta1.Query.{Evidence,AllEvidence}
cosmos.feegrant.v1beta1.Query.{Allowance,Allowances,AllowancesByGranter}
cosmos.upgrade.v1beta1.Query.{CurrentPlan,AppliedPlan,ModuleVersions}
# IBC
ibc.core.client.v1.Query.{ClientStates,ClientState}
ibc.core.connection.v1.Query.{Connections,Connection,ClientConnections}
ibc.core.channel.v1.Query.{Channels,Channel,ConnectionChannels}
ibc.applications.transfer.v1.Query.{Denom,Denoms,DenomHash,EscrowAddress,TotalEscrowForDenom,Params}
```

## Commit & Pull Request Guidelines

Recent history uses Conventional Commit-style subjects: `feat:`, `fix:`, `docs:`, `chore:`, and breaking changes with `!` or `BREAKING CHANGE`. Release automation reads these prefixes to decide version bumps, so keep subjects accurate and imperative. Pull requests should describe behavioral changes, list verification commands run, mention any live endpoint requirements, and call out public API or dependency changes.
