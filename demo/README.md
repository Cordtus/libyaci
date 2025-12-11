# Multi-Chain Explorer Demo

Demonstrates querying Cosmos SDK chains via gRPC reflection.

## Build

```bash
go build -o explorer .
```

## Usage

```bash
./explorer -addr <grpc-endpoint> [-insecure] [-timeout 30s]
```

## Examples

```bash
./explorer -addr grpc.osmosis.zone:9090
./explorer -addr localhost:9090 -insecure
```

## Output

The demo queries and displays:

1. Node information (chain ID, version, SDK version)
2. Available gRPC services
3. Latest block height and time
4. Token supply
5. Staking pool status
6. Active governance proposals
7. IBC channel count
8. Chain-specific modules (if any)
