# Outstanding Features, TODOs, and Improvements

> **Historical.** Automated scan from 2026-01-27; not maintained. Several items
> are already implemented (for example pagination helpers) and some referenced
> options have been removed. Verify against the current code. See `AGENTS.md`.

This document catalogs all identified improvements, missing features, and enhancements discovered during a comprehensive repository scan on 2026-01-27.

## Summary
- **Total Issues Identified**: 20+
- **Categories**: Features (8), Enhancements (7), Documentation (3), Testing (2), Performance (2)
- **Priority Levels**: High (6), Medium (10), Low (6)

---

## 🚀 Feature Requests

### 1. **Extension Field Support** ⚠️ HIGH PRIORITY
**Category**: Feature  
**Labels**: `enhancement`, `feature`, `protobuf`  
**Description**: Currently, extension fields are not supported. The `FindExtensionByName` and `FindExtensionByNumber` methods always return `protoregistry.NotFound`.

**Details**:
- Located in `client.go:508-515`
- Would enable full protobuf extension support
- Useful for chains using proto extensions

**Acceptance Criteria**:
- Implement `FindExtensionByName` to resolve extensions via reflection
- Implement `FindExtensionByNumber` to resolve extensions by field number
- Add tests for extension resolution
- Document extension support in README

---

### 2. **Streaming RPC Support**
**Category**: Feature  
**Labels**: `enhancement`, `feature`, `grpc`  
**Priority**: HIGH

**Description**: Add support for streaming gRPC methods (client streaming, server streaming, and bidirectional streaming).

**Details**:
- Current implementation only supports unary RPCs
- Many blockchain services offer streaming endpoints (e.g., event subscriptions, block streaming)
- Would require new methods: `InvokeClientStream`, `InvokeServerStream`, `InvokeBidiStream`

**Example Use Cases**:
- Real-time block subscriptions
- Event streaming
- Transaction mempool monitoring

**Acceptance Criteria**:
- Implement client streaming support
- Implement server streaming support
- Implement bidirectional streaming support
- Add examples for each type
- Update documentation

---

### 3. **Transaction Broadcasting Support**
**Category**: Feature  
**Labels**: `enhancement`, `feature`, `cosmos`  
**Priority**: HIGH

**Description**: Add convenience methods for broadcasting transactions to Cosmos SDK chains.

**Details**:
- Currently, users must manually call `cosmos.tx.v1beta1.Service.BroadcastTx`
- Should support SYNC, ASYNC, and BLOCK broadcast modes
- Include transaction simulation support
- Helper methods for common transaction types

**Proposed API**:
```go
resp, err := client.BroadcastTx(txBytes, BroadcastMode_SYNC)
simResp, err := client.SimulateTx(txBytes)
```

**Acceptance Criteria**:
- Add `BroadcastTx` method with mode parameter
- Add `SimulateTx` method
- Add examples in documentation
- Add integration tests

---

### 4. **Connection Pooling**
**Category**: Feature  
**Labels**: `enhancement`, `performance`  
**Priority**: MEDIUM

**Description**: Implement connection pooling for high-throughput applications.

**Details**:
- Current implementation uses a single connection
- High-volume applications may benefit from connection pooling
- Should be optional (backward compatible)

**Proposed API**:
```go
client, err := libyaci.DialPool(ctx, addr,
    libyaci.WithPoolSize(5),
    libyaci.WithInsecure(),
)
```

**Acceptance Criteria**:
- Implement connection pool manager
- Add `DialPool` function
- Add `WithPoolSize` option
- Load balance requests across pool
- Add benchmarks comparing pooled vs single connection

---

### 5. **Rate Limiting Support**
**Category**: Feature  
**Labels**: `enhancement`, `reliability`  
**Priority**: MEDIUM

**Description**: Add built-in rate limiting to prevent overwhelming servers.

**Details**:
- Useful for public endpoints with rate limits
- Should support both requests/second and requests/minute
- Optional feature (disabled by default)

**Proposed API**:
```go
client, err := libyaci.Dial(ctx, addr,
    libyaci.WithRateLimit(10, time.Second),
)
```

**Acceptance Criteria**:
- Implement token bucket or similar algorithm
- Add `WithRateLimit` option
- Add tests for rate limiting behavior
- Document usage and best practices

---

### 6. **Interceptor Support**
**Category**: Feature  
**Labels**: `enhancement`, `extensibility`  
**Priority**: MEDIUM

**Description**: Allow users to register custom interceptors for logging, metrics, or request modification.

**Details**:
- gRPC already supports interceptors
- Should expose this capability to libyaci users
- Useful for observability, debugging, and custom authentication

**Proposed API**:
```go
client, err := libyaci.Dial(ctx, addr,
    libyaci.WithUnaryInterceptor(myLoggingInterceptor),
)
```

**Acceptance Criteria**:
- Add `WithUnaryInterceptor` option
- Add `WithStreamInterceptor` option (for future streaming support)
- Provide example interceptors (logging, metrics)
- Document interceptor usage

---

### 7. **Batch Request Support**
**Category**: Feature  
**Labels**: `enhancement`, `performance`  
**Priority**: LOW

**Description**: Add ability to batch multiple independent requests into a single operation.

**Details**:
- Useful for fetching multiple pieces of data efficiently
- Could use goroutines internally for parallel requests
- Return results in same order as requests

**Proposed API**:
```go
results, err := client.InvokeBatch([]BatchRequest{
    {Method: "cosmos.bank.v1beta1.Query.Balance", Request: []byte(`{...}`)},
    {Method: "cosmos.staking.v1beta1.Query.Pool", Request: []byte(`{}`)},
})
```

**Acceptance Criteria**:
- Implement `InvokeBatch` method
- Handle partial failures gracefully
- Add tests for batch operations
- Document batch API and limitations

---

### 8. **Health Check Endpoint**
**Category**: Feature  
**Labels**: `enhancement`, `reliability`  
**Priority**: LOW

**Description**: Add a convenience method to check if the server is healthy and responsive.

**Details**:
- Useful for connection pool health checks
- Should check both connection and reflection availability
- Return detailed status information

**Proposed API**:
```go
health, err := client.HealthCheck()
// Returns: connected, reflection_available, latency_ms
```

**Acceptance Criteria**:
- Implement `HealthCheck` method
- Test connection state
- Test reflection API availability
- Measure and return latency
- Add documentation

---

## ⚡ Enhancements

### 9. **Better Error Messages**
**Category**: Enhancement  
**Labels**: `enhancement`, `dx`, `error-handling`  
**Priority**: HIGH

**Description**: Improve error messages with more context and actionable suggestions.

**Current Issues**:
- Some errors don't provide enough context
- No suggestions for common mistakes
- Stack traces could be more helpful

**Specific Improvements**:
- Include method name in all error messages
- Suggest WithProtoDir when TypeNotFoundError occurs
- Add hints for common ALPN errors
- Provide examples in error messages

**Acceptance Criteria**:
- Review all error messages in codebase
- Add context and suggestions
- Create custom error types where appropriate
- Update tests to verify error messages

---

### 10. **Pagination Helper**
**Category**: Enhancement  
**Labels**: `enhancement`, `cosmos`, `dx`  
**Priority**: HIGH

**Description**: Add helper methods to simplify pagination through large result sets.

**Details**:
- Many Cosmos SDK queries return paginated results
- Current pagination requires manual key management
- Common pattern that should be simplified

**Proposed API**:
```go
// Auto-paginate and collect all results
allValidators, err := client.GetAllValidatorsPaginated()

// Or use iterator pattern
iter := client.NewPaginatedIterator("cosmos.bank.v1beta1.Query.AllBalances", request)
for iter.Next() {
    // Process result
}
```

**Acceptance Criteria**:
- Add pagination helper utilities
- Support auto-collection of all pages
- Support iterator pattern for large datasets
- Add examples in documentation
- Test with various Cosmos SDK queries

---

### 11. **Reconnection Logic**
**Category**: Enhancement  
**Labels**: `enhancement`, `reliability`  
**Priority**: MEDIUM

**Description**: Add automatic reconnection with exponential backoff when connection is lost.

**Details**:
- Current implementation requires manual reconnection
- Network issues or server restarts require client restart
- Should be configurable (opt-in)

**Proposed API**:
```go
client, err := libyaci.Dial(ctx, addr,
    libyaci.WithAutoReconnect(true),
    libyaci.WithReconnectBackoff(1*time.Second, 30*time.Second),
)
```

**Acceptance Criteria**:
- Implement connection monitoring
- Add exponential backoff algorithm
- Add `WithAutoReconnect` option
- Add tests for reconnection scenarios
- Document reconnection behavior

---

### 12. **Caching Layer**
**Category**: Enhancement  
**Labels**: `enhancement`, `performance`  
**Priority**: MEDIUM

**Description**: Add optional caching for frequently accessed data.

**Details**:
- Some queries return static/slow-changing data (chain params, etc.)
- Cache could significantly reduce load on servers
- Should be optional with configurable TTL

**Proposed API**:
```go
client, err := libyaci.Dial(ctx, addr,
    libyaci.WithCache(true),
    libyaci.WithCacheTTL(5*time.Minute),
)
```

**Acceptance Criteria**:
- Implement in-memory cache with TTL
- Add `WithCache` and `WithCacheTTL` options
- Allow cache bypass for specific requests
- Add metrics for cache hits/misses
- Document caching behavior and limitations

---

### 13. **Request Timeout Configuration**
**Category**: Enhancement  
**Labels**: `enhancement`, `configuration`  
**Priority**: MEDIUM

**Description**: Add per-request timeout configuration.

**Details**:
- Current implementation relies on context deadlines
- Would be convenient to set default timeouts
- Should support per-method timeout overrides

**Proposed API**:
```go
client, err := libyaci.Dial(ctx, addr,
    libyaci.WithDefaultTimeout(30*time.Second),
)

// Or per-request
resp, err := client.InvokeWithTimeout(method, request, 10*time.Second)
```

**Acceptance Criteria**:
- Add `WithDefaultTimeout` option
- Add `InvokeWithTimeout` method
- Document timeout behavior
- Add tests for timeout scenarios

---

### 14. **Metrics and Observability**
**Category**: Enhancement  
**Labels**: `enhancement`, `observability`  
**Priority**: MEDIUM

**Description**: Add built-in metrics collection using Prometheus or similar.

**Details**:
- Request counts, latencies, error rates
- Cache hit/miss rates (if caching is implemented)
- Connection pool utilization (if pooling is implemented)
- Resolver statistics (already partially implemented)

**Proposed API**:
```go
client, err := libyaci.Dial(ctx, addr,
    libyaci.WithMetrics(true),
    libyaci.WithMetricsRegistry(promRegistry),
)
```

**Acceptance Criteria**:
- Define metrics to collect
- Implement metrics collection
- Add Prometheus exporter
- Provide example of metrics usage
- Document available metrics

---

### 15. **Proto File Export**
**Category**: Enhancement  
**Labels**: `enhancement`, `tooling`  
**Priority**: LOW

**Description**: Add ability to export fetched proto descriptors to .proto files.

**Details**:
- Useful for debugging and documentation
- Can help users understand available services
- Could generate entire proto file tree from reflection

**Proposed API**:
```go
err := client.ExportProtos("/path/to/output/dir")
```

**Acceptance Criteria**:
- Implement proto descriptor to .proto file conversion
- Handle dependencies correctly
- Preserve package structure
- Add example in documentation

---

## 📚 Documentation

### 16. **Advanced Usage Guide**
**Category**: Documentation  
**Labels**: `documentation`  
**Priority**: MEDIUM

**Description**: Create comprehensive guide for advanced usage patterns.

**Suggested Topics**:
- Custom fallback registries
- Working with deprecated modules
- Performance optimization tips
- Error handling best practices
- Integration patterns
- Multi-chain applications

**Acceptance Criteria**:
- Create `docs/advanced-usage.md`
- Cover all suggested topics
- Include code examples
- Link from main README

---

### 17. **Architecture Documentation**
**Category**: Documentation  
**Labels**: `documentation`  
**Priority**: LOW

**Description**: Document internal architecture and design decisions.

**Suggested Content**:
- How reflection protocol works
- Type resolution flow diagram
- Fallback registry mechanism
- UTF-8 patching approach
- Thread safety guarantees

**Acceptance Criteria**:
- Create `docs/architecture.md`
- Include diagrams
- Explain key design decisions
- Document thread safety model

---

### 18. **Migration Guide from Other Clients**
**Category**: Documentation  
**Labels**: `documentation`  
**Priority**: LOW

**Description**: Create migration guides from common gRPC clients.

**Target Audiences**:
- Users migrating from cosmjs/stargate
- Users migrating from cosmos-sdk-go
- Users migrating from grpcurl

**Acceptance Criteria**:
- Create `docs/migration.md`
- Include comparison tables
- Provide migration examples
- Document common pitfalls

---

## 🧪 Testing

### 19. **Integration Tests**
**Category**: Testing  
**Labels**: `testing`, `quality`  
**Priority**: HIGH

**Description**: Add comprehensive integration tests against real chains.

**Details**:
- Current tests use mocks
- Should test against real Cosmos SDK chains (testnet or local)
- Cover all Cosmos SDK convenience methods
- Test error scenarios

**Acceptance Criteria**:
- Set up local test chain or use public testnet
- Add integration test suite
- Test all major features
- Add CI job for integration tests
- Document how to run integration tests

---

### 20. **Fuzzing Tests**
**Category**: Testing  
**Labels**: `testing`, `quality`, `security`  
**Priority**: MEDIUM

**Description**: Add fuzzing tests for input validation and parsing.

**Target Areas**:
- Method name parsing
- JSON request/response parsing
- Proto descriptor processing
- UTF-8 error recovery

**Acceptance Criteria**:
- Implement Go fuzzing tests
- Target critical parsing functions
- Add to CI pipeline
- Document fuzzing coverage

---

## 🔧 Technical Debt & Code Quality

### 21. **Reduce Magic Numbers**
**Category**: Code Quality  
**Labels**: `code-quality`, `refactoring`  
**Priority**: LOW

**Description**: Extract magic numbers into named constants.

**Examples**:
- Retry delays (`2*attempt * time.Second`)
- Buffer sizes
- Timeout values

**Acceptance Criteria**:
- Identify all magic numbers
- Create named constants
- Update code to use constants
- Document constant meanings

---

### 22. **Improve Test Coverage**
**Category**: Testing  
**Labels**: `testing`, `quality`  
**Priority**: MEDIUM

**Description**: Increase test coverage to >80%.

**Current Status**: Unknown (no coverage report in CI)

**Focus Areas**:
- Error paths
- Edge cases
- Concurrent access scenarios
- Fallback registry
- ProtoDir loading

**Acceptance Criteria**:
- Measure current coverage
- Add tests to reach 80%+
- Add coverage reporting to CI
- Add coverage badge to README

---

## 🎯 Developer Experience

### 23. **CLI Tool Enhancements**
**Category**: Enhancement  
**Labels**: `enhancement`, `tooling`, `dx`  
**Priority**: LOW

**Description**: Enhance the example CLI tool with more features.

**Suggested Enhancements**:
- Interactive mode (select service/method from menu)
- Save/load request templates
- Output formatting options (JSON, YAML, table)
- Request history
- Authentication support

**Acceptance Criteria**:
- Implement suggested features
- Update CLI documentation
- Add examples for each feature

---

### 24. **Go Module Documentation**
**Category**: Documentation  
**Labels**: `documentation`, `dx`  
**Priority**: LOW

**Description**: Improve pkg.go.dev documentation with more examples.

**Details**:
- Add examples for all major functions
- Include runnable examples
- Document common patterns
- Add links to guides

**Acceptance Criteria**:
- Add example functions to all major APIs
- Ensure examples compile and run
- Add package-level examples
- Verify on pkg.go.dev

---

## 🔐 Security

### 25. **TLS Certificate Verification Options**
**Category**: Enhancement  
**Labels**: `enhancement`, `security`  
**Priority**: MEDIUM

**Description**: Add more TLS configuration options.

**Suggested Options**:
- Custom CA certificates
- Client certificate authentication
- TLS version constraints
- Cipher suite selection

**Proposed API**:
```go
client, err := libyaci.Dial(ctx, addr,
    libyaci.WithTLSConfig(tlsConfig),
)
```

**Acceptance Criteria**:
- Add `WithTLSConfig` option
- Support custom CA certificates
- Support client certificates
- Document TLS configuration
- Add security best practices guide

---

## 📊 Summary by Priority

### High Priority (6 issues)
1. Extension Field Support
2. Streaming RPC Support
3. Transaction Broadcasting Support
4. Better Error Messages
5. Pagination Helper
6. Integration Tests

### Medium Priority (10 issues)
7. Connection Pooling
8. Rate Limiting Support
9. Interceptor Support
10. Reconnection Logic
11. Caching Layer
12. Request Timeout Configuration
13. Metrics and Observability
14. Advanced Usage Guide
15. Improve Test Coverage
16. TLS Certificate Verification Options

### Low Priority (9 issues)
17. Batch Request Support
18. Health Check Endpoint
19. Proto File Export
20. Architecture Documentation
21. Migration Guide
22. Reduce Magic Numbers
23. CLI Tool Enhancements
24. Go Module Documentation
25. Fuzzing Tests

---

## 🏷️ Suggested Labels

Create these labels in the GitHub repository:
- `enhancement` - New feature or improvement
- `feature` - New feature request
- `documentation` - Documentation improvements
- `testing` - Test-related issues
- `performance` - Performance improvements
- `reliability` - Reliability/stability improvements
- `dx` - Developer experience improvements
- `security` - Security-related issues
- `code-quality` - Code quality/refactoring
- `extensibility` - Extensibility features
- `observability` - Logging/metrics/monitoring
- `configuration` - Configuration options
- `tooling` - CLI and tooling improvements
- `cosmos` - Cosmos SDK specific features
- `grpc` - gRPC protocol features
- `protobuf` - Protocol Buffers features
- `error-handling` - Error handling improvements
- `quality` - Overall quality improvements
- `refactoring` - Code refactoring

---

## 📝 Notes

- This document was generated from an automated scan on 2026-01-27
- No existing TODO/FIXME comments were found in the codebase
- All issues are based on analysis of code structure, missing features, and potential improvements
- Priority levels are suggestions and can be adjusted based on project goals
- Some issues may have dependencies on others (e.g., streaming support needed before stream interceptors)

---

## ⚠️ Important: Cannot Create Issues Programmatically

**Note**: I do not have permission to create GitHub issues programmatically. The repository owner will need to manually create these issues using the GitHub web interface or GitHub CLI.

### To create these issues:

**Option 1: GitHub Web Interface**
1. Go to https://github.com/Cordtus/libyaci/issues/new
2. Copy the title and description from each issue above
3. Add appropriate labels
4. Submit the issue

**Option 2: GitHub CLI (if installed)**
```bash
gh issue create --title "TITLE" --body "DESCRIPTION" --label "LABELS"
```

**Option 3: Bulk Creation Script**
A script could be created to bulk-create these issues using the GitHub API with a personal access token.
