package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil/bech32"
)

const privKeyOneHex = "0000000000000000000000000000000000000000000000000000000000000001"

func TestPrivateKeySignerSignAndVerify(t *testing.T) {
	signer, err := NewPrivateKeySigner(privKeyOneHex, Secp256k1)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner failed: %v", err)
	}
	if len(signer.PublicKey()) != 33 {
		t.Fatalf("public key length = %d, want 33", len(signer.PublicKey()))
	}

	signDoc := []byte("sign-doc-bytes")
	sig, err := signer.Sign(signDoc)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("signature length = %d, want 64", len(sig))
	}

	// Deterministic (RFC 6979): signing again yields the same bytes.
	again, err := signer.Sign(signDoc)
	if err != nil {
		t.Fatalf("Sign (second) failed: %v", err)
	}
	if string(sig) != string(again) {
		t.Fatal("signature is not deterministic")
	}

	digest := sha256.Sum256(signDoc)
	var r, s btcec.ModNScalar
	if r.SetByteSlice(sig[:32]) || s.SetByteSlice(sig[32:]) {
		t.Fatal("signature scalar overflow")
	}
	parsed := ecdsa.NewSignature(&r, &s)
	pub, err := btcec.ParsePubKey(signer.PublicKey())
	if err != nil {
		t.Fatalf("ParsePubKey failed: %v", err)
	}
	if !parsed.Verify(digest[:], pub) {
		t.Fatal("signature did not verify against the public key")
	}
}

func TestPrivateKeySignerAddress(t *testing.T) {
	signer, err := NewPrivateKeySigner(privKeyOneHex, Secp256k1)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner failed: %v", err)
	}
	address, err := signer.Address("cosmos")
	if err != nil {
		t.Fatalf("Address failed: %v", err)
	}
	hrp, data, err := bech32.Decode(address)
	if err != nil {
		t.Fatalf("bech32.Decode failed: %v", err)
	}
	if hrp != "cosmos" {
		t.Fatalf("hrp = %q, want cosmos", hrp)
	}
	payload, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		t.Fatalf("ConvertBits failed: %v", err)
	}
	if len(payload) != 20 {
		t.Fatalf("address payload length = %d, want 20", len(payload))
	}

	osmo, err := signer.Address("osmo")
	if err != nil {
		t.Fatalf("Address(osmo) failed: %v", err)
	}
	if osmo == address || !strings.HasPrefix(osmo, "osmo1") {
		t.Fatalf("unexpected osmo address %q", osmo)
	}
}

func TestPrivateKeySignerPublicKeyVector(t *testing.T) {
	signer, err := NewPrivateKeySigner(privKeyOneHex, Secp256k1)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner failed: %v", err)
	}
	// secp256k1 generator point for private key 1.
	const want = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	if got := hex.EncodeToString(signer.PublicKey()); got != want {
		t.Fatalf("public key = %s, want %s", got, want)
	}
}

func TestPrivateKeySignerRedactsSecret(t *testing.T) {
	signer, err := NewPrivateKeySigner(privKeyOneHex, Secp256k1)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner failed: %v", err)
	}
	for _, rendered := range []string{fmt.Sprintf("%v", signer), fmt.Sprintf("%+v", signer), fmt.Sprintf("%#v", signer)} {
		if strings.Contains(rendered, privKeyOneHex) {
			t.Fatalf("rendered signer leaks the private key: %s", rendered)
		}
		if !strings.Contains(rendered, "pub:") {
			t.Fatalf("rendered signer should include the public key: %s", rendered)
		}
	}
}

func TestMnemonicRejectsMalformedHDPath(t *testing.T) {
	const mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	for _, path := range []string{
		"m/44/118'/0'/0/0",    // purpose not hardened
		"m/44'/118/0'/0/0",    // coin type not hardened
		"m/44'/118'/0'/0",     // missing index
		"m/44'/118'/0'/0/0/0", // extra segment
		"44'/118'/0'/0/0",     // missing m
	} {
		if _, err := NewMnemonicSigner(mnemonic, path, Secp256k1); err == nil {
			t.Fatalf("HD path %q should be rejected", path)
		}
	}
}

func TestEthereumAddressVector(t *testing.T) {
	signer, err := NewPrivateKeySigner(privKeyOneHex, EthSecp256k1)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner failed: %v", err)
	}
	// Well-known Ethereum private-key-1 address.
	const want = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf"
	if got := signer.EthereumAddress(); got != want {
		t.Fatalf("EthereumAddress = %q, want %q", got, want)
	}
}

func TestEthSecp256k1AddressDiffersFromSecp256k1(t *testing.T) {
	secp, err := NewPrivateKeySigner(privKeyOneHex, Secp256k1)
	if err != nil {
		t.Fatalf("secp signer: %v", err)
	}
	eth, err := NewPrivateKeySigner(privKeyOneHex, EthSecp256k1)
	if err != nil {
		t.Fatalf("eth signer: %v", err)
	}
	secpAddr, _ := secp.Address("cosmos")
	ethAddr, _ := eth.Address("cosmos")
	if secpAddr == ethAddr {
		t.Fatal("secp256k1 and ethsecp256k1 addresses should differ")
	}
	if eth.PubKeyTypeURL() != ethSecp256k1PubKeyURL {
		t.Fatalf("ethsecp256k1 pubkey type URL = %q", eth.PubKeyTypeURL())
	}
	if secp.PubKeyTypeURL() != secp256k1PubKeyTypeURL {
		t.Fatalf("secp256k1 pubkey type URL = %q", secp.PubKeyTypeURL())
	}
}

func TestMnemonicSignerDeterminism(t *testing.T) {
	const mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	first, err := NewMnemonicSigner(mnemonic, DefaultHDPath, Secp256k1)
	if err != nil {
		t.Fatalf("NewMnemonicSigner failed: %v", err)
	}
	second, err := NewMnemonicSigner(mnemonic, DefaultHDPath, Secp256k1)
	if err != nil {
		t.Fatalf("NewMnemonicSigner (second) failed: %v", err)
	}
	if string(first.PublicKey()) != string(second.PublicKey()) {
		t.Fatal("mnemonic derivation is not deterministic")
	}

	// Empty path must fall back to the default Cosmos path.
	defaulted, err := NewMnemonicSigner(mnemonic, "", Secp256k1)
	if err != nil {
		t.Fatalf("NewMnemonicSigner (default path) failed: %v", err)
	}
	if string(defaulted.PublicKey()) != string(first.PublicKey()) {
		t.Fatal("empty HD path should use the default path")
	}

	other, err := NewMnemonicSigner(mnemonic, "m/44'/118'/0'/0/1", Secp256k1)
	if err != nil {
		t.Fatalf("NewMnemonicSigner (other index) failed: %v", err)
	}
	if string(other.PublicKey()) == string(first.PublicKey()) {
		t.Fatal("different HD index should derive a different key")
	}

	if _, err := NewMnemonicSigner("not a valid mnemonic", "", Secp256k1); err == nil {
		t.Fatal("invalid mnemonic should be rejected")
	}
}

func TestGenerateMnemonic(t *testing.T) {
	mnemonic, err := GenerateMnemonic(256)
	if err != nil {
		t.Fatalf("GenerateMnemonic failed: %v", err)
	}
	if words := len(strings.Fields(mnemonic)); words != 24 {
		t.Fatalf("mnemonic word count = %d, want 24", words)
	}
	if _, err := GenerateMnemonic(100); err == nil {
		t.Fatal("invalid strength should be rejected")
	}
}

func TestNewPrivateKeySignerRejectsBadInput(t *testing.T) {
	if _, err := NewPrivateKeySigner("abcd", Secp256k1); err == nil {
		t.Fatal("short key should be rejected")
	}
	if _, err := NewPrivateKeySigner(strings.Repeat("0", 64), Secp256k1); err == nil {
		t.Fatal("zero key should be rejected")
	}
	// Curve order N is out of range.
	if _, err := NewPrivateKeySigner("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", Secp256k1); err == nil {
		t.Fatal("key equal to the curve order should be rejected")
	}
	if _, err := NewPrivateKeySigner(privKeyOneHex, KeyAlgorithm("nope")); err == nil {
		t.Fatal("unknown algorithm should be rejected")
	}
}
