# Architecture overview

libyaci is a dynamic gRPC client that uses server reflection to invoke methods
without generated protobuf stubs. It targets Cosmos SDK chains but works with any
reflection-enabled gRPC server. The design goal is that current-chain capability
comes from reflection, while removed/historical message types come from an
optional local proto directory.

## Components and ownership

| Area | Files | Responsibility |
|---|---|---|
| Client / connection | `client.go` | `Dial`, invocation entry points, context lifecycle, accessors |
| Resolver | `client.go` | Reflected descriptor cache, on-demand `Any` fetch, fallback, extensions |
| Reflection | `reflection.go` | Service listing, recursive descriptor fetch, registry build |
| Capability catalog | `catalog.go` | Advertised services/methods/messages and chain metadata |
| Dynamic messages | `method.go`, `request.go`, `response.go` | Descriptor-backed request/response builders |
| Pagination | `pagination.go` | `Method.EachPage` over Cosmos `pagination.key`/`next_key` |
| Fallback | `fallback.go`, `protodir.go` | Pre-registered descriptors and lazy local `.proto` compilation |
| Cosmos helpers | `cosmos.go` | Convenience wrappers over reflected Cosmos query methods |
| Signing | `signing/` | `SIGN_MODE_DIRECT` transaction building, signing, broadcasting |
| ALPN workaround | `alpnfix/` | Disables grpc-go ALPN enforcement for older nodes |

## Connection and reflection flow

1. `Dial` parses options and validates `WithProtoDir` eagerly (path must exist
   and be a directory).
2. It creates a client-lifetime context independent of the dial context, then
   dials with `grpc.NewClient` (the connection is established lazily).
3. `fetchDescriptorSnapshot` lists advertised services and recursively fetches
   their file descriptors and dependencies. Reflection v1 is tried first, then
   v1alpha; the working version is cached per connection.
4. `buildFileDescriptorSetReport` topologically sorts the descriptors and
   registers them best-effort. Files that cannot be registered (unresolvable
   dependency or option) are skipped and returned; `Dial` surfaces them via
   `Client.SkippedFiles()` instead of failing the whole client.
5. The catalog is built from the registered files but only marks services
   actually advertised by `ListServices` as callable.

## Type resolution and fallback

`Resolver` implements `protoregistry.MessageTypeResolver` and
`ExtensionTypeResolver`. Resolution order for an unknown `Any` type:

1. primary reflected registry (cache hit, no network);
2. on-demand reflection fetch, coordinated by an `inProgress` singleflight map;
3. the fallback registry (programmatically registered descriptors and/or the
   local proto directory);
4. `TypeNotFoundError` with a file-layout hint.

`WithProtoDir` is attached to a **clone** of the fallback registry, so a
per-client proto directory never mutates the shared `GlobalFallback()` or a
caller-owned `FallbackRegistry`.

Extensions are not resolvable through `protoregistry.Files.FindDescriptorByName`,
so `FindExtensionByName` / `FindExtensionByNumber` walk file and message
extension declarations explicitly (primary registry first, then fallback).

## Retry policy

Retries apply only to transient conditions:
`Unavailable`, `DeadlineExceeded`, `ResourceExhausted`, and `Aborted`.
Application errors (`InvalidArgument`, `NotFound`, `PermissionDenied`, local
parse errors) return immediately. Backoff is linear (2s, 4s, 6s…). Block-height
probing helpers pass a zero retry budget so an expected "height not available"
response is not retried. A timeout set with `WithDefaultTimeout` (or
`InvokeWithTimeout`) bounds the whole invocation, including retries and backoff.

## Thread safety

`Client` and `Resolver` are safe for concurrent use. The resolver guards its
registry with an `RWMutex`; concurrent lookups of the same symbol coordinate on
a per-symbol completion channel. Catalog maps are immutable after construction
and read under an `RWMutex`. Cache statistics use atomics. The reflection
version cache is a package-level map guarded by a mutex and cleared on
`Client.Close`.

## Compatibility and known limitations

- Go 1.24+, `grpc-go` v1.77, `protobuf` v1.36.
- Reflection v1 and v1alpha are both supported.
- TLS verifies against system roots by default; `WithTLSConfig` supplies a custom CA, client certificate, or `InsecureSkipVerify` for self-signed nodes.
- Some nodes fail ALPN negotiation; see the `alpnfix` package and the README.
- Only unary RPCs are invocable. Streaming methods are advertised by the catalog
  but calling one returns a clear error.
- `applyDescriptorPatches` rewrites `cosmos.base.abci.v1beta1.TxResponse.raw_log`
  from string to bytes (cosmos-sdk#22414), so JSON output for that field is
  base64. Dynamic UTF-8 recovery patches other string fields on demand, for both
  reflected and local-proto types.
- `Dial` uses a lazy connection; `WithDialTimeout` bounds descriptor fetching,
  not the TCP dial.

## Related

- Human contract: [`../README.md`](../README.md)
- Agent project map: [`../AGENTS.md`](../AGENTS.md)
- Signing design: [`signing.md`](signing.md)
- Fallback runbook: [`operations/deprecated-protos.md`](../operations/deprecated-protos.md)
