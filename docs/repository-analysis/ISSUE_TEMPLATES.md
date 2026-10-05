# GitHub Issue Templates

> **Historical.** Automated scan from 2026-01-27; not maintained. Verify against
> the current code before filing or acting on these issues. See `AGENTS.md`.

This document contains ready-to-copy issue templates for quick creation in the GitHub UI.

---

## Issue #1: Extension Field Support

**Title**: Support Protocol Buffer Extension Fields

**Labels**: `enhancement`, `feature`, `protobuf`

**Description**:
```markdown
## Problem
Currently, libyaci does not support Protocol Buffer extension fields. The `FindExtensionByName` and `FindExtensionByNumber` methods in the Resolver always return `protoregistry.NotFound`.

## Current Code
Located in `client.go:508-515`:
```go
// FindExtensionByName is not implemented (returns NotFound).
func (r *Resolver) FindExtensionByName(_ protoreflect.FullName) (protoreflect.ExtensionType, error) {
    return nil, protoregistry.NotFound
}

// FindExtensionByNumber is not implemented (returns NotFound).
func (r *Resolver) FindExtensionByNumber(_ protoreflect.FullName, _ protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
    return nil, protoregistry.NotFound
}
```

## Impact
This limitation prevents full protobuf compatibility for chains that use proto extensions in their messages.

## Proposed Solution
Implement both methods to:
1. Query server reflection for extension descriptors
2. Register and cache discovered extensions
3. Return proper ExtensionType instances

## Acceptance Criteria
- [ ] Implement `FindExtensionByName` to resolve extensions via server reflection
- [ ] Implement `FindExtensionByNumber` to resolve extensions by field number
- [ ] Add unit tests for extension resolution
- [ ] Add integration tests with real extension fields
- [ ] Document extension support in README
- [ ] Update examples if needed

## References
- [Protocol Buffers Extensions](https://protobuf.dev/programming-guides/extension_declarations/)
- [gRPC Reflection Protocol](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md)
```

---

## Issue #2: Streaming RPC Support

**Title**: Add Support for gRPC Streaming (Client, Server, and Bidirectional)

**Labels**: `enhancement`, `feature`, `grpc`

**Description**:
```markdown
## Problem
libyaci currently only supports unary (request-response) gRPC methods. Many blockchain services offer streaming endpoints for real-time data, but these cannot be accessed with the current implementation.

## Use Cases
- Real-time block subscriptions
- Event streaming (e.g., transaction events, validator events)
- Transaction mempool monitoring
- Continuous data feeds
- Interactive protocols

## Proposed Solution
Add three new methods to the Client:
1. `InvokeClientStream` - Client sends multiple messages, gets one response
2. `InvokeServerStream` - Client sends one message, receives multiple responses
3. `InvokeBidiStream` - Both client and server can send multiple messages

### Proposed API
```go
// Server streaming example
stream, err := client.InvokeServerStream(ctx, "chain.Service.Subscribe", request)
for {
    resp, err := stream.Recv()
    if err == io.EOF {
        break
    }
    // Process response
}

// Client streaming example
stream, err := client.InvokeClientStream(ctx, "chain.Service.Upload")
for _, msg := range messages {
    stream.Send(msg)
}
resp, err := stream.CloseAndRecv()

// Bidirectional streaming example
stream, err := client.InvokeBidiStream(ctx, "chain.Service.Exchange")
go func() {
    for {
        resp, err := stream.Recv()
        // Process responses
    }
}()
for _, msg := range messages {
    stream.Send(msg)
}
stream.CloseSend()
```

## Technical Considerations
- Dynamic message construction for streaming
- Proper stream lifecycle management
- Context cancellation handling
- Error propagation
- Thread safety for bidirectional streams

## Acceptance Criteria
- [ ] Implement `InvokeClientStream` method
- [ ] Implement `InvokeServerStream` method
- [ ] Implement `InvokeBidiStream` method
- [ ] Add comprehensive tests for each streaming type
- [ ] Add examples for common use cases
- [ ] Update documentation with streaming guide
- [ ] Ensure thread-safe operation

## Priority
High - Many blockchain applications require real-time data streaming.
```

---

## Issue #3: Transaction Broadcasting Support

**Title**: Add Convenience Methods for Cosmos SDK Transaction Broadcasting

**Labels**: `enhancement`, `feature`, `cosmos`

**Description**:
```markdown
## Problem
While it's possible to broadcast transactions using `client.Invoke("cosmos.tx.v1beta1.Service.BroadcastTx", ...)`, this requires users to manually construct the request JSON with the correct broadcast mode and handle the response parsing.

## Proposed Solution
Add high-level convenience methods for common transaction operations:

```go
// Broadcast a signed transaction
resp, err := client.BroadcastTx(txBytes, libyaci.BroadcastModeSYNC)

// Simulate a transaction before broadcasting
simResp, err := client.SimulateTx(txBytes)

// Broadcast and wait for commit (BLOCK mode)
resp, err := client.BroadcastTxCommit(txBytes)

// Check transaction status
tx, err := client.GetTxByHash(hash)
```

### Broadcast Modes
- `BroadcastModeASYNC` - Return immediately after submit
- `BroadcastModeSYNC` - Wait for CheckTx
- `BroadcastModeBLOCK` - Wait for commit (deprecated but still used)

## API Design
```go
type BroadcastMode string

const (
    BroadcastModeASYNC BroadcastMode = "BROADCAST_MODE_ASYNC"
    BroadcastModeSYNC  BroadcastMode = "BROADCAST_MODE_SYNC"
    BroadcastModeBLOCK BroadcastMode = "BROADCAST_MODE_BLOCK"
)

type TxResponse struct {
    Code      uint32
    TxHash    string
    RawLog    string
    GasWanted int64
    GasUsed   int64
    Height    int64
    // ... other fields
}

func (c *Client) BroadcastTx(txBytes []byte, mode BroadcastMode) (*TxResponse, error)
func (c *Client) SimulateTx(txBytes []byte) (*SimulateResponse, error)
```

## Acceptance Criteria
- [ ] Define `BroadcastMode` constants
- [ ] Implement `BroadcastTx` method
- [ ] Implement `SimulateTx` method
- [ ] Add convenience method `BroadcastTxCommit` for SYNC mode
- [ ] Add structured response types
- [ ] Add examples for each broadcast mode
- [ ] Add tests with mock responses
- [ ] Document transaction broadcasting workflow
- [ ] Add error handling guide

## References
- [Cosmos SDK Tx Service](https://docs.cosmos.network/main/build/modules/tx)
- Existing method: `methodGetTx` in `cosmos.go`
```

---

## Issue #4: Better Error Messages with Context

**Title**: Improve Error Messages with Actionable Context and Suggestions

**Labels**: `enhancement`, `dx`, `error-handling`

**Description**:
```markdown
## Problem
Current error messages sometimes lack context and actionable suggestions, making it harder for users to debug issues.

### Examples of Current Issues
1. Generic "failed to fetch descriptor" errors without method name
2. TypeNotFoundError without clear guidance on how to fix
3. ALPN errors without pointing to documentation
4. Connection errors without retry suggestions

## Proposed Improvements

### 1. Include Method Name in Errors
**Before**: `failed to invoke: context deadline exceeded`
**After**: `failed to invoke cosmos.bank.v1beta1.Query.Balance: context deadline exceeded (consider increasing timeout with WithDialTimeout)`

### 2. Enhanced TypeNotFoundError
**Before**: `type tendermint.liquidity.v1beta1.MsgSwap not found`
**After**: 
```
type tendermint.liquidity.v1beta1.MsgSwap not found

This is likely a deprecated module no longer available via server reflection.

Suggested fixes:
1. Use WithProtoDir() to provide local proto files:
   client, err := libyaci.Dial(ctx, addr, libyaci.WithProtoDir("./protos"))

2. Download proto files for this module:
   tendermint/liquidity/v1beta1/tx.proto

3. See documentation: https://github.com/Cordtus/libyaci#local-proto-directory
```

### 3. ALPN Connection Errors
**Before**: `transport: authentication handshake failed: credentials: cannot check peer: missing selected ALPN property`
**After**:
```
transport: authentication handshake failed: missing ALPN property

This server does not properly support ALPN negotiation (required by grpc-go v1.67+).

Suggested fixes:
1. Import the alpnfix package (recommended):
   import _ "github.com/Cordtus/libyaci/alpnfix"

2. Set environment variable:
   GRPC_ENFORCE_ALPN_ENABLED=false

See: https://github.com/Cordtus/libyaci#alpn-enforcement
```

### 4. Connection Errors with Retry Guidance
**Before**: `connection refused`
**After**: `failed to connect to localhost:9090: connection refused (make sure the gRPC server is running and reflection is enabled)`

## Implementation Plan
1. Create custom error types for common scenarios
2. Add helper methods to format errors with context
3. Include relevant documentation links
4. Add "did you mean?" suggestions for typos in method names

## Acceptance Criteria
- [ ] Review all error message sites in codebase
- [ ] Create custom error types (TypeNotFoundError, ConnectionError, etc.)
- [ ] Add method name to all RPC errors
- [ ] Add suggestions to all error types
- [ ] Include documentation links in errors
- [ ] Update error tests to verify new messages
- [ ] Document error handling best practices in README

## Benefits
- Faster debugging for users
- Reduced support burden
- Better developer experience
- Self-documenting code
```

---

## Issue #5: Pagination Helper Utilities

**Title**: Add Helper Methods to Simplify Cosmos SDK Pagination

**Labels**: `enhancement`, `cosmos`, `dx`

**Description**:
```markdown
## Problem
Many Cosmos SDK queries return paginated results. Current implementation requires users to manually:
1. Track pagination keys
2. Make multiple requests
3. Concatenate results
4. Handle errors in pagination loops

This is tedious and error-prone for common use cases like "fetch all validators" or "fetch all balances".

## Proposed Solution

### Auto-Pagination Methods
Add convenience methods that automatically paginate through all results:

```go
// Instead of manual pagination:
allValidators := []Validator{}
paginationKey := ""
for {
    resp, err := client.GetValidators("BOND_STATUS_BONDED", paginationKey)
    // ... handle pagination manually
}

// Use auto-pagination:
allValidators, err := client.GetAllValidatorsPaginated()
```

### Iterator Pattern
For large datasets where loading everything into memory is impractical:

```go
iter := client.NewPaginatedIterator(
    "cosmos.bank.v1beta1.Query.AllBalances",
    []byte(`{"address":"cosmos1..."}`),
)

for iter.Next() {
    result := iter.Value()
    // Process one page at a time
}

if err := iter.Error(); err != nil {
    // Handle error
}
```

### Generic Pagination Helper
```go
type PaginationConfig struct {
    PageSize   uint64
    MaxPages   int  // 0 = unlimited
    CountTotal bool
}

func (c *Client) InvokeWithPagination(
    method string,
    baseRequest map[string]interface{},
    config PaginationConfig,
) (*PaginatedResult, error)
```

## Acceptance Criteria
- [ ] Implement `InvokeWithPagination` generic helper
- [ ] Add iterator pattern implementation
- [ ] Update existing Cosmos methods to use pagination helpers
- [ ] Add `GetAllValidatorsPaginated()` example method
- [ ] Add examples for both patterns in documentation
- [ ] Add tests with various page sizes and limits
- [ ] Handle edge cases (empty results, single page, errors mid-pagination)
- [ ] Document best practices (when to use auto vs iterator)

## References
- Cosmos SDK uses `cosmos.base.query.v1beta1.PageRequest` and `PageResponse`
- Common pattern across all Cosmos SDK modules
```

---

## Issue #6: Integration Tests Against Real Chains

**Title**: Add Comprehensive Integration Test Suite

**Labels**: `testing`, `quality`

**Description**:
```markdown
## Problem
Current test suite uses mocks, which don't catch:
- Real-world gRPC behavior
- Actual proto descriptor complexities
- Network timing issues
- Edge cases in Cosmos SDK responses
- Fallback registry behavior with real deprecated types

## Proposed Solution
Add integration tests that run against real blockchain nodes:

### Test Targets
1. **Local testnet** (primary) - Using Docker Compose
2. **Public testnet** (secondary) - As fallback
3. **Mock server** (tertiary) - For CI when nodes unavailable

### Test Coverage
- Basic connectivity and reflection
- All Cosmos SDK convenience methods
- Pagination scenarios
- Transaction decoding with real transactions
- Fallback registry with deprecated types
- Error scenarios (invalid requests, missing methods)
- Concurrent access patterns
- Connection recovery

### Test Structure
```go
func TestIntegration_CosmosHub(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    addr := os.Getenv("TEST_CHAIN_ADDR")
    if addr == "" {
        t.Skip("TEST_CHAIN_ADDR not set")
    }
    
    client, err := libyaci.Dial(context.Background(), addr)
    require.NoError(t, err)
    defer client.Close()
    
    // Run tests...
}
```

### Docker Compose Setup
```yaml
version: '3'
services:
  chain:
    image: cosmoshub/gaia:latest
    ports:
      - "9090:9090"
    command: start --grpc.enable=true
```

## Acceptance Criteria
- [ ] Create Docker Compose setup for local test chain
- [ ] Implement integration test suite structure
- [ ] Add tests for all major features
- [ ] Test all Cosmos SDK convenience methods
- [ ] Add tests for error scenarios
- [ ] Add concurrent access tests
- [ ] Add CI job for integration tests (with Docker)
- [ ] Make integration tests skippable with `-short`
- [ ] Document how to run integration tests locally
- [ ] Add troubleshooting guide for test failures

## CI Configuration
```yaml
integration-tests:
  runs-on: ubuntu-latest
  services:
    chain:
      image: cosmoshub/gaia:latest
      ports:
        - 9090:9090
  steps:
    - name: Run integration tests
      run: go test -v -tags=integration ./...
```

## Benefits
- Catch real-world issues before release
- Validate behavior against actual chains
- Increase confidence in releases
- Document expected behavior
```

---

## Quick Reference: All Issues

| # | Title | Priority | Labels |
|---|-------|----------|--------|
| 1 | Extension Field Support | HIGH | enhancement, feature, protobuf |
| 2 | Streaming RPC Support | HIGH | enhancement, feature, grpc |
| 3 | Transaction Broadcasting | HIGH | enhancement, feature, cosmos |
| 4 | Better Error Messages | HIGH | enhancement, dx, error-handling |
| 5 | Pagination Helper | HIGH | enhancement, cosmos, dx |
| 6 | Integration Tests | HIGH | testing, quality |
| 7 | Connection Pooling | MEDIUM | enhancement, performance |
| 8 | Rate Limiting | MEDIUM | enhancement, reliability |
| 9 | Interceptor Support | MEDIUM | enhancement, extensibility |
| 10 | Reconnection Logic | MEDIUM | enhancement, reliability |
| 11 | Caching Layer | MEDIUM | enhancement, performance |
| 12 | Request Timeout Config | MEDIUM | enhancement, configuration |
| 13 | Metrics & Observability | MEDIUM | enhancement, observability |
| 14 | Advanced Usage Guide | MEDIUM | documentation |
| 15 | Test Coverage >80% | MEDIUM | testing, quality |
| 16 | TLS Certificate Options | MEDIUM | enhancement, security |
| 17 | Batch Request Support | LOW | enhancement, performance |
| 18 | Health Check Endpoint | LOW | enhancement, reliability |
| 19 | Proto File Export | LOW | enhancement, tooling |
| 20 | Architecture Docs | LOW | documentation |
| 21 | Migration Guide | LOW | documentation |
| 22 | Reduce Magic Numbers | LOW | code-quality, refactoring |
| 23 | CLI Enhancements | LOW | enhancement, tooling, dx |
| 24 | Go Module Docs | LOW | documentation, dx |
| 25 | Fuzzing Tests | LOW | testing, quality, security |

---

## Bulk Creation Script (Requires GitHub CLI)

If you have the GitHub CLI installed and authenticated, you can use this script:

```bash
#!/bin/bash

# Issue #1
gh issue create \
  --title "Support Protocol Buffer Extension Fields" \
  --label "enhancement,feature,protobuf" \
  --body "$(cat << 'EOF'
## Problem
Currently, libyaci does not support Protocol Buffer extension fields...
[Copy full description from above]
EOF
)"

# Repeat for each issue...
```

Or use the GitHub API directly with curl:

```bash
curl -X POST \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/Cordtus/libyaci/issues \
  -d '{
    "title": "Support Protocol Buffer Extension Fields",
    "body": "ISSUE DESCRIPTION",
    "labels": ["enhancement", "feature", "protobuf"]
  }'
```
