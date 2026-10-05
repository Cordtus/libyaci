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
  `PubKeyTypeURL`, and `Algorithm`, so hardware, KMS, remote, or multisig
  signers can be added without touching the tx builder. The built-in
  `PrivateKeySigner` is the in-process implementation.
- **Direct mode only.** Signatures are deterministic (RFC 6979) 64-byte `r||s`
  over `SHA-256(signDocBytes)`, placed in `SIGN_MODE_DIRECT` `ModeInfo`.
  `SIGN_MODE_AMINO_JSON` is not implemented.
- **Key algorithms.** `Secp256k1` (address = `RIPEMD160(SHA256(pubkey))`) and
  `EthSecp256k1` (Keccak-256 address). The public-key Any type URL defaults to
  `/cosmos.crypto.secp256k1.PubKey` or
  `/ethermint.crypto.v1.ethsecp256k1.PubKey` and is overridable with
  `WithPubKeyTypeURL` for chains that use a different ethsecp256k1 URL.
- **Key sources.** Raw 32-byte key (64-char hex or base64) and BIP39 mnemonic
  with BIP44 derivation (default `m/44'/118'/0'/0/0`). BIP32 child derivation is
  implemented in-package (HMAC-SHA512), so no external BIP32 dependency is
  needed. The HD path is validated to the BIP44 shape (6 segments, first three
  hardened) so a typo fails instead of silently deriving a different key.
- **Secret safety.** `PrivateKeySigner` implements `String`/`GoString` to redact
  the private key, so `%v`/`%+v`/`%#v` logging cannot leak it. `WithPubKeyTypeURL`
  is rejected for `Secp256k1` signers (it only makes sense with `EthSecp256k1`).

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

## Not implemented

`SIGN_MODE_AMINO_JSON`, legacy amino multisig, fee estimation heuristics, and
automatic sequence retry on `ErrWrongSequence`.
