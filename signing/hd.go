package signing

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/go-bip39"
)

// DefaultHDPath is the standard Cosmos SDK BIP44 path.
const DefaultHDPath = "m/44'/118'/0'/0/0"

var curveN = btcec.S256().N

func deriveFromMnemonic(mnemonic, hdPath string) ([]byte, error) {
	mnemonic = strings.TrimSpace(mnemonic)
	if mnemonic == "" {
		return nil, errors.New("mnemonic is empty")
	}
	if hdPath == "" {
		hdPath = DefaultHDPath
	}
	if err := validateHDPath(hdPath); err != nil {
		return nil, err
	}
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return nil, fmt.Errorf("validate mnemonic: %w", err)
	}
	return derivePath(seed, hdPath)
}

// validateHDPath enforces the BIP44 shape so a typo fails loudly instead of
// silently deriving a different key: m / purpose' / coin_type' / account' /
// change / index, with the first three segments hardened and the last two not.
func validateHDPath(path string) error {
	parts := splitHDPath(path)
	if len(parts) != 6 {
		return fmt.Errorf("HD path must have 6 segments (m/44'/coin'/account'/change/index), got %d", len(parts))
	}
	if parts[0] != "m" {
		return errors.New("HD path must start with m")
	}
	if _, hardened, err := parsePathSegment(parts[1]); err != nil || !hardened {
		return errors.New("HD path purpose segment must be hardened, for example 44'")
	}
	if _, hardened, err := parsePathSegment(parts[2]); err != nil || !hardened {
		return errors.New("HD path coin type segment must be hardened, for example 118'")
	}
	if _, hardened, err := parsePathSegment(parts[3]); err != nil || !hardened {
		return errors.New("HD path account segment must be hardened, for example 0'")
	}
	if _, hardened, err := parsePathSegment(parts[4]); err != nil || hardened {
		return errors.New("HD path change segment must be non-hardened, for example 0")
	}
	if _, hardened, err := parsePathSegment(parts[5]); err != nil || hardened {
		return errors.New("HD path address index segment must be non-hardened, for example 0")
	}
	return nil
}

func derivePath(seed []byte, path string) ([]byte, error) {
	key, chainCode := masterKey(seed)
	segments := splitHDPath(path)
	if len(segments) == 0 || segments[0] != "m" {
		return nil, fmt.Errorf("HD path must start with m: %s", path)
	}
	for _, segment := range segments[1:] {
		index, hardened, err := parsePathSegment(segment)
		if err != nil {
			return nil, err
		}
		if hardened {
			index += 0x80000000
		}
		key, chainCode, err = deriveChild(key, chainCode, index, hardened)
		if err != nil {
			return nil, err
		}
	}
	return key, nil
}

func masterKey(seed []byte) (key, chainCode []byte) {
	mac := hmac.New(sha512.New, []byte("Bitcoin seed"))
	mac.Write(seed)
	sum := mac.Sum(nil)
	return sum[:32], sum[32:]
}

func parsePathSegment(segment string) (uint32, bool, error) {
	hardened := strings.HasSuffix(segment, "'")
	segment = strings.TrimSuffix(segment, "'")
	value, err := strconv.ParseUint(segment, 10, 31)
	if err != nil {
		return 0, false, fmt.Errorf("invalid HD path segment %q: %w", segment, err)
	}
	return uint32(value), hardened, nil
}

// deriveChild implements BIP32 CKDpriv in-package to stay dependency-light and
// match the proven reference. Per BIP32 an invalid child should advance to the
// next index; aborting is a deliberate, practically-unreachable deviation.
// ponytail: hand-rolled BIP32; swap for btcutil/hdkeychain if the module graph matters.
func deriveChild(parentKey, parentChainCode []byte, index uint32, hardened bool) (key, chainCode []byte, err error) {
	data := make([]byte, 0, 37)
	if hardened {
		data = append(data, 0)
		data = append(data, parentKey...)
	} else {
		_, pub := btcec.PrivKeyFromBytes(parentKey)
		data = append(data, pub.SerializeCompressed()...)
	}
	var indexBytes [4]byte
	binary.BigEndian.PutUint32(indexBytes[:], index)
	data = append(data, indexBytes[:]...)

	mac := hmac.New(sha512.New, parentChainCode)
	mac.Write(data)
	sum := mac.Sum(nil)
	il, childChain := sum[:32], sum[32:]

	ilInt := new(big.Int).SetBytes(il)
	if ilInt.Sign() == 0 || ilInt.Cmp(curveN) >= 0 {
		return nil, nil, errors.New("derived invalid secp256k1 key")
	}
	parentInt := new(big.Int).SetBytes(parentKey)
	childInt := new(big.Int).Add(ilInt, parentInt)
	childInt.Mod(childInt, curveN)
	if childInt.Sign() == 0 {
		return nil, nil, errors.New("derived zero secp256k1 key")
	}
	childKey := childInt.Bytes()
	if len(childKey) < privKeySize {
		padded := make([]byte, privKeySize)
		copy(padded[privKeySize-len(childKey):], childKey)
		childKey = padded
	}
	return childKey, childChain, nil
}

func splitHDPath(path string) []string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "(") && strings.HasSuffix(path, ")") {
		path = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(path, "("), ")"))
	}
	raw := strings.Split(path, "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		parts = append(parts, strings.TrimSpace(part))
	}
	return parts
}
