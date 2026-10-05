# Runbook: deprecated / historical proto types

## When you need this

Server reflection only exposes what the node currently serves. Historical
transactions and state can reference message types from modules that have since
been removed (for example `tendermint.liquidity.v1beta1`). Reflection cannot
describe them, so decoding fails with a `TypeNotFoundError`. Supply the missing
definitions from local `.proto` files.

## Usage

Library:

```go
client, err := libyaci.Dial(ctx, addr, libyaci.WithProtoDir("./protos"))
```

CLI:

```bash
./grpc-cli -addr localhost:9090 -insecure -protodir ./protos -method "cosmos.tx.v1beta1.Service.GetTx" -request '{"hash":"..."}'
```

## Directory layout

Mirror the proto package path. Imports within the directory and
`google/protobuf/*` are resolved automatically.

```
protos/
  tendermint/liquidity/v1beta1/tx.proto
  cosmos/base/v1beta1/coin.proto
```

The directory is validated when `Dial` runs (it must exist and be a directory).
Compilation stays lazy: files are compiled on the first type-resolution miss.
`client.ProtoDir()` returns the configured directory.

## Resolution order

1. primary reflected registry;
2. on-demand reflection for the missing symbol;
3. the local proto directory;
4. `TypeNotFoundError` whose hint names the expected file path.

## UTF-8 recovery

Some historical fields are declared `string` but carry binary data. When
protojson fails on invalid UTF-8, the library retries with a temporary registry
that patches the offending field to `bytes`. This works for both reflected and
local-proto types; the original resolver is never modified.

## Troubleshooting

| Symptom | Cause | Action |
|---|---|---|
| `Dial` fails with `proto directory ... no such file or directory` | `WithProtoDir` path missing | Fix the path; validation is eager |
| `proto path ... is not a directory` | Path is a file | Point at a directory |
| `TypeNotFoundError` naming a package path | Type not in reflection or the directory | Add the `.proto` under the matching path |
| A decode still fails for a dependency type | A transitive import is missing | Add the imported `.proto` too; imports are resolved within the directory |
| `client.SkippedFiles()` is non-empty | Reflected files could not be registered | Those methods will not be callable; inspect the names for unresolvable options/deps |

## Obtaining proto files

Cosmos SDK chains publish protos in their source repository under `proto/`, and
standard module protos are in the Cosmos SDK repository or the Buf Schema
Registry (`buf.build/cosmos/cosmos-sdk`).
