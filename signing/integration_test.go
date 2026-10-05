package signing

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Cordtus/libyaci"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestIntegrationSigning exercises the signing path against live chains.
//
// Endpoints come from LIBYACI_SIGNING_ENDPOINTS, a comma-separated list where
// each entry is host:port with optional pipe-separated flags:
//
//	host:443
//	host:9090|insecure
//	host:443|ethsecp
//	host:443|prefix=osmo
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
	address  string
	insecure bool
	algo     KeyAlgorithm
	prefix   string
}

func parseEndpointSpec(raw string) endpointSpec {
	parts := strings.Split(raw, "|")
	spec := endpointSpec{address: strings.TrimSpace(parts[0]), algo: Secp256k1}
	for _, flag := range parts[1:] {
		flag = strings.TrimSpace(flag)
		switch {
		case flag == "insecure":
			spec.insecure = true
		case flag == "ethsecp":
			spec.algo = EthSecp256k1
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
