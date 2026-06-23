# Repository Guidelines

## Project Structure & Module Organization

`libyaci` is a Go module for dynamic gRPC queries against reflection-enabled Cosmos SDK chains. Core package files live at the repository root: `client.go` handles dialing and invocation, `reflection.go` fetches descriptors, `catalog.go` indexes reflected capabilities, `method.go`, `request.go`, and `response.go` expose descriptor-backed calls, `fallback.go` and `protodir.go` provide secondary proto fallback support, and `cosmos.go` contains Cosmos convenience methods. The `alpnfix/` subpackage contains the grpc-go ALPN workaround. Root-level `*_test.go` files hold package tests and `mock_test.go` provides the bufconn reflection test server. Runnable samples are under `examples/`, including `examples/cli` and `examples/explorer`.

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

## Commit & Pull Request Guidelines

Recent history uses Conventional Commit-style subjects: `feat:`, `fix:`, `docs:`, `chore:`, and breaking changes with `!` or `BREAKING CHANGE`. Release automation reads these prefixes to decide version bumps, so keep subjects accurate and imperative. Pull requests should describe behavioral changes, list verification commands run, mention any live endpoint requirements, and call out public API or dependency changes.
