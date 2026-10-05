// Package signing adds Cosmos transaction signing and broadcasting on top of
// the reflection-first libyaci client.
//
// Transaction scaffolding (cosmos.tx.v1beta1.*, cosmos.crypto.secp256k1.PubKey,
// cosmos.base.v1beta1.Coin) is resolved from server reflection, so no generated
// Cosmos stubs are required. Only the cryptographic primitives (secp256k1,
// bech32, BIP39) are compiled in.
//
// The package signs SIGN_MODE_DIRECT transactions only.
package signing

//lint:file-ignore SA1019 RIPEMD-160 is required for Cosmos secp256k1 addresses

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/cosmos/go-bip39"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/sha3"
)

// KeyAlgorithm identifies the address derivation used by a key.
type KeyAlgorithm string

const (
	// Secp256k1 is the standard Cosmos SDK key: address = RIPEMD160(SHA256(pubkey)).
	Secp256k1 KeyAlgorithm = "secp256k1"
	// EthSecp256k1 derives a Keccak-256 (Ethereum) address, used by Ethermint,
	// Injective, Sei, and other EVM-compatible Cosmos chains.
	EthSecp256k1 KeyAlgorithm = "ethsecp256k1"
)

const (
	secp256k1PubKeyTypeURL = "/cosmos.crypto.secp256k1.PubKey"
	ethSecp256k1PubKeyURL  = "/ethermint.crypto.v1.ethsecp256k1.PubKey"
	privKeySize            = 32
)

// Signer produces Cosmos SIGN_MODE_DIRECT signatures.
type Signer interface {
	// PublicKey returns the 33-byte compressed secp256k1 public key.
	PublicKey() []byte
	// Address returns the bech32 address for the given human-readable prefix.
	Address(prefix string) (string, error)
	// Sign signs the sign-doc bytes and returns a 64-byte secp256k1 signature
	// (r || s), as expected by the Cosmos SDK.
	Sign(signDoc []byte) ([]byte, error)
	// PubKeyTypeURL is the protobuf Any type URL placed in the signer info.
	PubKeyTypeURL() string
}

// PrivateKeySigner is an in-process secp256k1 Signer.
type PrivateKeySigner struct {
	priv          []byte
	pub           []byte
	algo          KeyAlgorithm
	pubKeyTypeURL string
}

// PrivateKeyOption configures a PrivateKeySigner.
type PrivateKeyOption func(*PrivateKeySigner)

// WithPubKeyTypeURL overrides the protobuf Any type URL used for the public key
// in the signer info. Use this for chains with a non-default ethsecp256k1 type
// URL (for example Injective).
func WithPubKeyTypeURL(url string) PrivateKeyOption {
	return func(s *PrivateKeySigner) {
		if url != "" {
			s.pubKeyTypeURL = url
		}
	}
}

// NewPrivateKeySigner builds a signer from a hex (64 chars, optional 0x prefix)
// or base64-encoded secp256k1 private key.
func NewPrivateKeySigner(secret string, algo KeyAlgorithm, opts ...PrivateKeyOption) (*PrivateKeySigner, error) {
	raw, err := decodePrivateKey(secret)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeySignerFromBytes(raw, algo, opts...)
}

// NewPrivateKeySignerFromBytes builds a signer from raw 32-byte private key
// material.
func NewPrivateKeySignerFromBytes(priv []byte, algo KeyAlgorithm, opts ...PrivateKeyOption) (*PrivateKeySigner, error) {
	if len(priv) != privKeySize {
		return nil, fmt.Errorf("private key must be %d bytes, got %d", privKeySize, len(priv))
	}
	if new(big.Int).SetBytes(priv).Sign() == 0 || new(big.Int).SetBytes(priv).Cmp(btcec.S256().N) >= 0 {
		return nil, errors.New("private key is outside the secp256k1 range")
	}
	normalized, err := normalizeAlgorithm(algo)
	if err != nil {
		return nil, err
	}
	_, pub := btcec.PrivKeyFromBytes(priv)
	s := &PrivateKeySigner{
		priv:          append([]byte(nil), priv...),
		pub:           pub.SerializeCompressed(),
		algo:          normalized,
		pubKeyTypeURL: defaultPubKeyTypeURL(normalized),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.pubKeyTypeURL != defaultPubKeyTypeURL(s.algo) && s.algo != EthSecp256k1 {
		return nil, errors.New("WithPubKeyTypeURL is only valid for EthSecp256k1 signers")
	}
	return s, nil
}

// NewMnemonicSigner derives a signer from a BIP39 mnemonic and BIP44 HD path.
// If hdPath is empty, m/44'/118'/0'/0/0 is used.
func NewMnemonicSigner(mnemonic, hdPath string, algo KeyAlgorithm, opts ...PrivateKeyOption) (*PrivateKeySigner, error) {
	priv, err := deriveFromMnemonic(mnemonic, hdPath)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeySignerFromBytes(priv, algo, opts...)
}

// GenerateMnemonic returns a new BIP39 mnemonic. strengthBits must be one of
// 128, 160, 192, 224, or 256; 256 is recommended.
func GenerateMnemonic(strengthBits int) (string, error) {
	switch strengthBits {
	case 128, 160, 192, 224, 256:
	default:
		return "", fmt.Errorf("mnemonic strength must be 128, 160, 192, 224, or 256 bits, got %d", strengthBits)
	}
	entropy, err := bip39.NewEntropy(strengthBits)
	if err != nil {
		return "", fmt.Errorf("generate entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("generate mnemonic: %w", err)
	}
	return mnemonic, nil
}

// PublicKey returns a copy of the compressed public key.
func (s *PrivateKeySigner) PublicKey() []byte {
	return append([]byte(nil), s.pub...)
}

// Algorithm returns the configured key algorithm.
func (s *PrivateKeySigner) Algorithm() KeyAlgorithm {
	return s.algo
}

// String redacts the private key so a signer is safe to log.
func (s *PrivateKeySigner) String() string {
	return fmt.Sprintf("signing.PrivateKeySigner{algo:%s, pub:%x}", s.algo, s.pub)
}

// GoString redacts the private key for %#v formatting.
func (s *PrivateKeySigner) GoString() string {
	return s.String()
}

// PubKeyTypeURL returns the Any type URL used for the public key.
func (s *PrivateKeySigner) PubKeyTypeURL() string {
	return s.pubKeyTypeURL
}

// Address returns the bech32 address for the given prefix.
func (s *PrivateKeySigner) Address(prefix string) (string, error) {
	return AddressFromPublicKey(s.pub, prefix, s.algo)
}

// EthereumAddress returns the EIP-55-agnostic lowercase 0x address derived from
// the uncompressed public key (Keccak-256). Useful for EVM-compatible Cosmos
// chains and for cross-checking an ethsecp256k1 key.
func (s *PrivateKeySigner) EthereumAddress() string {
	pub, err := btcec.ParsePubKey(s.pub)
	if err != nil {
		return ""
	}
	return "0x" + hex.EncodeToString(ethAddressBytes(pub.SerializeUncompressed()[1:]))
}

// Sign signs the sign-doc bytes with deterministic ECDSA (RFC 6979) over
// SHA-256 and returns a 64-byte r||s signature.
func (s *PrivateKeySigner) Sign(signDoc []byte) ([]byte, error) {
	if len(signDoc) == 0 {
		return nil, errors.New("sign doc is empty")
	}
	key, _ := btcec.PrivKeyFromBytes(s.priv)
	digest := sha256.Sum256(signDoc)
	sig := ecdsa.SignCompact(key, digest[:], false)
	// SignCompact returns a 65-byte [recovery||r||s]; Cosmos wants r||s.
	return sig[1:], nil
}

// AddressFromPublicKey derives a bech32 address from a compressed secp256k1
// public key.
func AddressFromPublicKey(publicKey []byte, prefix string, algo KeyAlgorithm) (string, error) {
	if prefix == "" {
		return "", errors.New("bech32 prefix is required")
	}
	normalized, err := normalizeAlgorithm(algo)
	if err != nil {
		return "", err
	}
	pub, err := btcec.ParsePubKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("parse public key: %w", err)
	}
	var payload []byte
	switch normalized {
	case Secp256k1:
		payload = secp256k1AddressBytes(pub.SerializeCompressed())
	case EthSecp256k1:
		payload = ethAddressBytes(pub.SerializeUncompressed()[1:])
	}
	converted, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("convert address bits: %w", err)
	}
	address, err := bech32.Encode(prefix, converted)
	if err != nil {
		return "", fmt.Errorf("encode bech32 address: %w", err)
	}
	return address, nil
}

func decodePrivateKey(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil, errors.New("private key is empty")
	}
	trimmed = strings.TrimPrefix(trimmed, "0x")
	if len(trimmed) == privKeySize*2 {
		raw, err := hex.DecodeString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("decode hex private key: %w", err)
		}
		return raw, nil
	}
	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode private key as 64-char hex or base64: %w", err)
	}
	return raw, nil
}

func normalizeAlgorithm(algo KeyAlgorithm) (KeyAlgorithm, error) {
	switch KeyAlgorithm(strings.ToLower(strings.TrimSpace(string(algo)))) {
	case "", Secp256k1:
		return Secp256k1, nil
	case EthSecp256k1:
		return EthSecp256k1, nil
	default:
		return "", fmt.Errorf("unsupported key algorithm %q", algo)
	}
}

func defaultPubKeyTypeURL(algo KeyAlgorithm) string {
	if algo == EthSecp256k1 {
		return ethSecp256k1PubKeyURL
	}
	return secp256k1PubKeyTypeURL
}

func secp256k1AddressBytes(compressedPublicKey []byte) []byte {
	sum := sha256.Sum256(compressedPublicKey)
	ripemd := ripemd160.New()
	ripemd.Write(sum[:])
	return ripemd.Sum(nil)
}

func ethAddressBytes(uncompressedPublicKey []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(uncompressedPublicKey)
	sum := hasher.Sum(nil)
	return sum[len(sum)-20:]
}
