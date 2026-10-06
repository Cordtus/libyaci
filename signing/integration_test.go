package signing

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Cordtus/libyaci"
	"github.com/btcsuite/btcd/btcutil/bech32"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestIntegrationSigning exercises the signing path against live chains.
//
// Endpoints come from LIBYACI_SIGNING_ENDPOINTS, a comma-separated list where
// each entry is host:port with optional pipe-separated flags:
//
//	host:443
//	host:9090|insecure
//	host:443|tlsinsecure
//	host:443|ethsecp
//	host:443|prefix=osmo
//	host:443|prefix=genesis|estimate
//
// The optional "estimate" flag performs a live fee estimation against an
// existing account's public key (signed with an unrelated key, which simulation
// accepts), proving the gas/fee path end to end.
//
// It is skipped unless the variable is set. The test verifies that reflection
// exposes the tx scaffolding, that a transaction can be built and signed for
// the chain's key/prefix, and that the node can decode the resulting bytes.
// It does not broadcast (no funded accounts), so it never changes state.
func TestIntegrationSigning(t *testing.T) {
	raw := os.Getenv("LIBYACI_SIGNING_ENDPOINTS")
	if raw == "" {
		t.Skip("LIBYACI_SIGNING_ENDPOINTS not set")
	}

	passed := 0
	failed := 0
	for _, spec := range strings.Split(raw, ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if probeEndpoint(t, spec) {
			passed++
		} else {
			failed++
		}
	}
	t.Logf("%d endpoint(s) passed, %d failed", passed, failed)
	if passed == 0 {
		t.Fatal("no endpoint passed the signing probe")
	}
}

type endpointSpec struct {
	address       string
	insecure      bool
	tlsSkipVerify bool
	algo          KeyAlgorithm
	prefix        string
	estimate      bool
}

func parseEndpointSpec(raw string) endpointSpec {
	parts := strings.Split(raw, "|")
	spec := endpointSpec{address: strings.TrimSpace(parts[0]), algo: Secp256k1}
	for _, flag := range parts[1:] {
		flag = strings.TrimSpace(flag)
		switch {
		case flag == "insecure":
			spec.insecure = true
		case flag == "tlsinsecure":
			spec.tlsSkipVerify = true
		case flag == "ethsecp":
			spec.algo = EthSecp256k1
		case flag == "estimate":
			spec.estimate = true
		case strings.HasPrefix(flag, "prefix="):
			spec.prefix = strings.TrimPrefix(flag, "prefix=")
		}
	}
	return spec
}

func probeEndpoint(t *testing.T, raw string) bool {
	t.Helper()
	spec := parseEndpointSpec(raw)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	opts := []libyaci.Option{
		libyaci.WithDialTimeout(30 * time.Second),
		libyaci.WithDefaultTimeout(30 * time.Second),
	}
	if spec.insecure {
		opts = append(opts, libyaci.WithInsecure())
	}
	if spec.tlsSkipVerify {
		opts = append(opts, libyaci.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	}
	client, err := libyaci.Dial(ctx, spec.address, opts...)
	if err != nil {
		t.Logf("dial failed: %v", err)
		return false
	}
	defer client.Close()

	// 1. Reflection must expose the tx scaffolding.
	for _, name := range []string{
		txBodyType, authInfoType, signDocType, txRawType,
		"cosmos.crypto.secp256k1.PubKey", "cosmos.base.v1beta1.Coin",
		"cosmos.tx.signing.v1beta1.SignMode",
	} {
		if _, err := client.Resolver().FindMessageByName(protoreflect.FullName(name)); err != nil {
			// SignMode is an enum, not a message; tolerate that.
			if name == "cosmos.tx.signing.v1beta1.SignMode" {
				continue
			}
			t.Logf("reflection missing %s: %v", name, err)
			return false
		}
	}

	prefix := spec.prefix
	if prefix == "" {
		if p, err := client.GetBech32Prefix(); err == nil {
			prefix = p
		}
	}
	if prefix == "" {
		prefix = "cosmos"
	}

	chainID := ""
	if id, err := client.GetChainID(); err == nil {
		chainID = id
	}
	if chainID == "" {
		t.Log("could not determine chain ID")
		return false
	}

	if spec.estimate {
		return probeEstimate(t, ctx, client, spec, prefix, chainID)
	}

	// 2. Pick a public-key type URL the chain actually has.
	pubKeyURL, err := ResolvePubKeyTypeURL(client.Resolver(), spec.algo)
	if err != nil {
		t.Logf("no pubkey type URL for %s: %v", spec.algo, err)
		return false
	}
	signer, err := NewPrivateKeySigner(privKeyOneHex, spec.algo, WithPubKeyTypeURL(pubKeyURL))
	if err != nil {
		t.Logf("signer: %v", err)
		return false
	}
	address, err := signer.Address(prefix)
	if err != nil {
		t.Logf("address: %v", err)
		return false
	}

	// 3. Account lookup against a module account (proves auth query + parsing).
	if modules, err := client.GetModuleAccounts(); err == nil && len(modules) > 0 {
		for moduleAddr := range modules {
			if _, _, err := FetchAccountNumberSequence(ctx, client, moduleAddr); err != nil {
				t.Logf("account lookup for %s failed: %v", moduleAddr, err)
			} else {
				break
			}
		}
	} else if err != nil {
		t.Logf("module accounts unavailable: %v", err)
	}

	// 4. Build and sign a self-send.
	denom := "stake"
	if bond, err := client.GetBondDenom(); err == nil && bond != "" {
		denom = bond
	}
	msg := Msg(fmt.Sprintf(
		`{"@type":"/cosmos.bank.v1beta1.MsgSend","fromAddress":%q,"toAddress":%q,"amount":[{"denom":%q,"amount":"1"}]}`,
		address, address, denom,
	))
	tx, err := BuildAndSign(client.Resolver(), signer, chainID, []Msg{msg}, TxOptions{
		Fee:           []Coin{{Denom: denom, Amount: "1"}},
		GasLimit:      200000,
		AccountNumber: 0,
		Sequence:      0,
		AddressPrefix: prefix,
	})
	if err != nil {
		t.Logf("build/sign failed: %v", err)
		return false
	}
	if len(tx.Signature) != 64 || len(tx.TxBytes) == 0 {
		t.Log("build/sign produced empty output")
		return false
	}

	// 5. Ask the node to decode/simulate the tx. A logical ante error
	// (insufficient funds, account not found) still proves the node decoded
	// and accepted the tx structure; a decode/signature error does not.
	gas, simErr := Simulate(ctx, client, tx.TxBytes)
	switch {
	case simErr == nil:
		t.Logf("PASS %s chainID=%s prefix=%s gas=%d", spec.address, chainID, prefix, gas)
		return true
	case isDecodeOrSignatureError(simErr):
		t.Logf("simulate rejected the encoding/signature: %v", simErr)
		return false
	default:
		t.Logf("PASS %s chainID=%s prefix=%s algo=%s (simulate: %v)", spec.address, chainID, prefix, spec.algo, simErr)
		return true
	}
}

func isDecodeOrSignatureError(err error) bool {
	message := strings.ToLower(err.Error())
	for _, needle := range []string{
		"unable to decode", "unmarshal", "cannot unmarshal", "unknown field",
		"no concrete type registered", "signature verification failed",
		"failed to verify", "invalid signature", "unauthorized",
		"invalid type", "failed to parse",
	} {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}

// estimateSigner presents an existing account's public key in the signer info
// but signs with an unrelated key. Simulation accepts this, which is how fee
// estimation can be done without the account's private key.
type estimateSigner struct {
	pub   []byte
	inner *PrivateKeySigner
	url   string
}

func (s estimateSigner) PublicKey() []byte     { return s.pub }
func (s estimateSigner) PubKeyTypeURL() string { return s.url }
func (s estimateSigner) Sign(doc []byte) ([]byte, error) {
	return s.inner.Sign(doc)
}
func (s estimateSigner) Address(prefix string) (string, error) {
	return AddressFromPublicKey(s.pub, prefix, Secp256k1)
}

// probeEstimate discovers an existing account with an exposed public key and
// estimates gas/fee for a self-send signed by an unrelated key.
func probeEstimate(t *testing.T, ctx context.Context, client *libyaci.Client, spec endpointSpec, prefix, chainID string) bool {
	account, pubKey, typeURL, number, sequence, err := discoverAccount(ctx, client, prefix)
	if err != nil {
		t.Logf("estimate: %v", err)
		return false
	}
	inner, err := NewTestSigner(Secp256k1)
	if err != nil {
		t.Logf("estimate: inner signer: %v", err)
		return false
	}
	signer := estimateSigner{pub: pubKey, inner: inner, url: typeURL}

	denom := "stake"
	if bond, err := client.GetBondDenom(); err == nil && bond != "" {
		denom = bond
	}
	msg := Msg(fmt.Sprintf(
		`{"@type":"/cosmos.bank.v1beta1.MsgSend","fromAddress":%q,"toAddress":%q,"amount":[{"denom":%q,"amount":"1"}]}`,
		account, account, denom,
	))
	gasLimit, fee, err := EstimateFee(ctx, client, signer, chainID, []Msg{msg}, TxOptions{
		Fee:           []Coin{{Denom: denom, Amount: "1"}},
		GasLimit:      300000,
		AccountNumber: number,
		Sequence:      sequence,
		AddressPrefix: prefix,
	}, []Coin{{Denom: denom, Amount: "0.01"}}, 1.3)
	if err != nil {
		t.Logf("estimate failed: %v", err)
		return false
	}
	t.Logf("PASS estimate %s chainID=%s account=%s pubkeyType=%s gasLimit=%d fee=%v", spec.address, chainID, account, typeURL, gasLimit, fee)
	return true
}

func discoverAccount(ctx context.Context, client *libyaci.Client, prefix string) (address string, pubKey []byte, typeURL string, number, sequence uint64, err error) {
	validators, err := client.GetAllValidators()
	if err != nil {
		return "", nil, "", 0, 0, fmt.Errorf("validators: %w", err)
	}
	if len(validators) == 0 {
		return "", nil, "", 0, 0, errors.New("no validators")
	}
	var valoper string
	for addr := range validators {
		valoper = addr
		break
	}
	_, payload, err := bech32.Decode(valoper)
	if err != nil {
		return "", nil, "", 0, 0, fmt.Errorf("decode valoper: %w", err)
	}
	raw, err := bech32.ConvertBits(payload, 5, 8, false)
	if err != nil {
		return "", nil, "", 0, 0, fmt.Errorf("convert valoper: %w", err)
	}
	converted, err := bech32.ConvertBits(raw, 8, 5, true)
	if err != nil {
		return "", nil, "", 0, 0, fmt.Errorf("convert account: %w", err)
	}
	address, err = bech32.Encode(prefix, converted)
	if err != nil {
		return "", nil, "", 0, 0, fmt.Errorf("encode account: %w", err)
	}

	info, err := client.GetAccountInfo(address)
	if err != nil {
		return "", nil, "", 0, 0, fmt.Errorf("account info for %s: %w", address, err)
	}
	number, _ = strconv.ParseUint(strings.TrimSpace(info.Info.AccountNumber), 10, 64)
	sequence, _ = strconv.ParseUint(strings.TrimSpace(info.Info.Sequence), 10, 64)
	pubAny, ok := info.Info.PubKey.(map[string]any)
	if !ok {
		return "", nil, "", 0, 0, fmt.Errorf("account %s has no exposed public key", address)
	}
	keyB64, _ := pubAny["key"].(string)
	pubKey, err = base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(pubKey) != 33 {
		return "", nil, "", 0, 0, fmt.Errorf("unusable public key for %s", address)
	}
	typeURL, _ = pubAny["@type"].(string)
	if typeURL == "" {
		typeURL = secp256k1PubKeyTypeURL
	}
	return address, pubKey, typeURL, number, sequence, nil
}
