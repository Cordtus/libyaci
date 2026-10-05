# Dev docs index

Project-level knowledge for libyaci that has no home beside the code. Read the
relevant document before planning work in its area; update it once the change is
verified. Source-local contracts stay in Go comments; this tree holds what spans
files or operators.

The human-facing contract is [`README.md`](../README.md); the agent project map
is [`AGENTS.md`](../AGENTS.md). Neither is duplicated here — this index points at
them.

## Architecture

- `architecture/overview.md` — how reflection, type resolution, fallback, retry,
  and threading fit together, plus compatibility and known limitations.
- `architecture/signing.md` — transaction signing/broadcasting design, key
  algorithms, dependencies, and security boundaries.

## Operations

- `operations/deprecated-protos.md` — runbook for supplying local `.proto`
  files so historical/deprecated types decode, and how to troubleshoot it.

## Logs

- `logs/2026-10.md` — ongoing-work log, newest entry last.

## Historical material

- `../docs/repository-analysis/` — automated scan from 2026-01-27, not
  maintained; verify claims against the code before acting on them.
