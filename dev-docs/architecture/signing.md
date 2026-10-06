# Transaction signing

The `signing/` subpackage builds, signs, and broadcasts Cosmos transactions on
top of the reflection-first client. It deliberately does not import the Cosmos
SDK or generated stubs: the transaction scaffolding is resolved from server
reflection.

## Design

- **Scaffolding from reflection.** `cosmos.tx.v1beta1.TxBody`, `AuthInfo`,
  `SignerInfo`, `ModeInfo`, `Fee`, `SignDoc`, `TxRaw`,
  `cosmos.crypto.secp256k1.PubKey`, `cosmos.base.v1beta1.Coin`, and the
  `SignMode` enum are fetched through a `Resolver` (in practice
  `client.Resolver()`). Messages are assembled as protobuf-JSON and decoded with
  `protojson`, then serialized with deterministic `proto.Marshal`.
- **Signer interface.** `Signer` exposes `PublicKey`, `Address(prefix)`, `Sign`,
  and `PubKeyTypeURL`, so hardware, KMS, remote, or multisig signers can be
  added without touching the tx builder. The built-in `PrivateKeySigner` is the
  in-process implementation.
- **Direct mode only.** Signatures are deterministic (RFC 6979) 64-byte `r||s`
  over `SHA-256(signDocBytes)`, placed in `SIGN_MODE_DIRECT` `ModeInfo`.
  `SIGN_MODE_AMINO_JSON` is not implemented.
- **Key algorithms.** `Secp256k1` (address = `RIPEMD160(SHA256(pubkey))`) and
  `EthSecp256k1` (Keccak-256 address). The public-key Any type URL defaults to
  `/cosmos.crypto.secp256k1.PubKey` or
  `/ethermint.crypto.v1.ethsecp256k1.PubKey` and is overridable with
  `WithPubKeyTypeURL` for chains that use a different ethsecp256k1 URL (for
  example Injective's `/injective.crypto.v1beta1.ethsecp256k1.PubKey`). Use
  `ResolvePubKeyTypeURL(resolver, algo)` to pick the URL the chain actually
  reflects instead of hard-coding it.
- **Bech32 prefix.** Addresses must use the chain's configured prefix
  (`cosmos`, `osmo`, `testcore`, `sei`, `inj`, …). `client.GetBech32Prefix()`
  discovers it when `cosmos.auth.v1beta1.Query.Bech32Prefix` is advertised;
  otherwise the caller supplies it. A wrong prefix fails the node's address
  decoding, not our construction.
- **Fee estimation.** `EstimateGas` builds and signs a transaction with a
  provisional gas limit and returns the simulated gas used. `EstimateFee`
  applies a gas adjustment (default 1.3) and computes `ceil(gasLimit × price)`
  with exact rational arithmetic. Gas prices are caller-supplied because nodes
  do not reliably expose minimum gas prices over gRPC. Simulation accepts any
  valid signature, so fee estimation does not require the account's private key;
  it does require the signer-info public key to derive to an existing account
  (fee-payer lookup), and a few chains also enforce funds during simulation.
- **Key sources.** Raw 32-byte key (64-char hex or base64) and BIP39 mnemonic
  with BIP44 derivation (default `m/44'/118'/0'/0/0`). BIP32 child derivation is
  implemented in-package (HMAC-SHA512), so no external BIP32 dependency is
  needed. The HD path is validated to the BIP44 shape (6 segments, first three
  hardened) so a typo fails instead of silently deriving a different key.
- **Secret safety.** `PrivateKeySigner` implements `String`/`GoString` to redact
  the private key, so `%v`/`%+v`/`%#v` logging cannot leak it. `WithPubKeyTypeURL`
  is rejected for `Secp256k1` signers (it only makes sense with `EthSecp256k1`).

## Test identity

`TestMnemonic` (`about` ×11 + `abuse`) and `NewTestSigner(algo)` are the
canonical test/dry-run key. It controls no funds and must never hold real
assets. Known values at `m/44'/118'/0'/0/0`:

- public key `022fb148970ff67750208b4f12248b5338995877e6d774eaf393a728c54faa5b19`
- Ethereum address `0x497c499b8d09d421c15d61bb99c33d0c859779bd`
- secp256k1 addresses: `cosmos1wcf9yalcx4fdwm5hw8arswcvrfrds4xxxtaz44`,
  `terra1wcf9yalcx4fdwm5hw8arswcvrfrds4xxq08zh4`
- ethsecp256k1 address: `genesis1f97ynxudp82zrs2avxaenseapjzew7dag0wkqm`

The integration probe's `estimate` mode uses it as the unrelated signing key.

## Dependencies

The subpackage adds `github.com/btcsuite/btcd/btcec/v2` (secp256k1 + ECDSA),
`github.com/btcsuite/btcd/btcutil/bech32` (addresses),
`github.com/cosmos/go-bip39` (mnemonics), and `golang.org/x/crypto`
(RIPEMD-160, Keccak-256). The root package does not import `signing`, so
read-only users do not compile these.

## Boundaries and security

- Private keys and mnemonics are handled in process. Callers must not log them.
  For production custody, implement the `Signer` interface with an external
  signer.
- `Broadcast` treats a non-zero CheckTx `code` as an error but still returns the
  parsed response. `BroadcastModeSync` only confirms CheckTx acceptance, not
  inclusion; use `BroadcastModeBlock` or a follow-up `GetTx` to wait for a
  block.
- `Simulate` returns `gasInfo.gasUsed`; callers still choose the gas limit.

## Live verification

`signing/integration_test.go` (gated by `LIBYACI_SIGNING_ENDPOINTS`) builds,
signs, and simulates a self-send against live testnets. Verified against:

| Chain | chain-id | algo | outcome |
|---|---|---|---|
| Cosmos Hub theta | theta-testnet-001 | secp256k1 | descriptor fetch slow; endpoint-dependent |
| Celestia | mocha-5 | secp256k1 | decoded; fee-payer account absent (expected) |
| Akash | sandbox-2 | secp256k1 | decoded; fee-payer account absent |
| Coreum | coreum-testnet-1 | secp256k1 | decoded; prefix `testcore` |
| Axelar | axelar-testnet-lisbon-3 | secp256k1 | decoded; fee-payer account absent |
| Noble | grand-1 | secp256k1 | decoded; fee-payer account absent |
| Seda | seda-1-testnet | secp256k1 | decoded; fee-payer account absent |
| Sei | atlantic-2 | secp256k1 | decoded; reached fee check (insufficient funds) |
| Injective | injective-888 | ethsecp256k1 | decoded; reached fee check (insufficient funds) |
| **GenesisL1** | genesis_29-2 | ethsecp256k1 | signing decoded (3/3 nodes); `EstimateFee` live (1102 `el1`) |
| **Terra2** | phoenix-1 | secp256k1 | signing decoded (2/5 nodes); `EstimateFee` live (964 `uluna`) |

`EstimateFee` was exercised live on GenesisL1 and Terra2 by presenting an
existing account's public key and signing the sign-doc with an unrelated key —
confirming that simulation does not require the matching private key. GenesisL1
accounts use `/ethermint.crypto.v1.ethsecp256k1.PubKey`, so callers must use
`EthSecp256k1` (or the account's reflected key type) for it.

"Reached the fee check" means the ante handler verified the signature and then
failed on funds, which is the expected outcome for an unfunded test key. All
outcomes are ante errors, never decode/signature errors. Self-signed TLS
endpoints are reachable with `libyaci.WithTLSConfig(&tls.Config{InsecureSkipVerify: true})`
(the probe's `tlsinsecure` flag); verified against `celestia-testnet-grpc.itrocket.net:443`.

## Not implemented

`SIGN_MODE_AMINO_JSON`, legacy amino multisig, automatic gas-price discovery,
and automatic sequence retry on `ErrWrongSequence`.
