package signing

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Cordtus/libyaci"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Resolver resolves protobuf message and extension types. libyaci's
// *Resolver and *dynamicpb.Types both satisfy it.
type Resolver interface {
	protoregistry.MessageTypeResolver
	protoregistry.ExtensionTypeResolver
}

// PubKeyTypeURLCandidates returns the known public-key Any type URLs for an
// algorithm, in preference order.
func PubKeyTypeURLCandidates(algo KeyAlgorithm) []string {
	switch algo {
	case EthSecp256k1:
		return []string{
			ethSecp256k1PubKeyURL,
			"/injective.crypto.v1beta1.ethsecp256k1.PubKey",
			"/cosmos.crypto.ethsecp256k1.PubKey",
		}
	default:
		return []string{secp256k1PubKeyTypeURL}
	}
}

// ResolvePubKeyTypeURL returns the first public-key Any type URL for algo that
// the resolver can resolve from reflection. Use it to choose the URL passed to
// WithPubKeyTypeURL, since ethsecp256k1 chains differ (for example Injective).
func ResolvePubKeyTypeURL(resolver Resolver, algo KeyAlgorithm) (string, error) {
	if resolver == nil {
		return "", errors.New("resolver is required")
	}
	for _, url := range PubKeyTypeURLCandidates(algo) {
		name := protoreflect.FullName(strings.TrimPrefix(url, "/"))
		if _, err := resolver.FindMessageByName(name); err == nil {
			return url, nil
		}
	}
	return "", fmt.Errorf("no %s public-key type was found via reflection", algo)
}

// Msg is a protobuf-JSON message object that includes an "@type" field, for
// example {"@type":"/cosmos.bank.v1beta1.MsgSend","fromAddress":"...",...}.
type Msg = json.RawMessage

// Coin is a Cosmos SDK coin.
type Coin struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

// TxOptions controls transaction construction.
type TxOptions struct {
	Memo          string
	TimeoutHeight uint64
	Fee           []Coin
	GasLimit      uint64
	FeePayer      string
	FeeGranter    string
	AccountNumber uint64
	Sequence      uint64
	// AddressPrefix, when set, populates SignedTx.SignerAddress.
	AddressPrefix string
}

// SignedTx is a fully signed, broadcast-ready transaction.
type SignedTx struct {
	BodyBytes     []byte
	AuthInfoBytes []byte
	SignDocBytes  []byte
	Signature     []byte // 64-byte secp256k1 r||s
	TxBytes       []byte
	TxHash        string // uppercase hex of SHA-256(TxBytes)
	SignerAddress string // empty unless TxOptions.AddressPrefix was set
}

const (
	txBodyType     = "cosmos.tx.v1beta1.TxBody"
	authInfoType   = "cosmos.tx.v1beta1.AuthInfo"
	signDocType    = "cosmos.tx.v1beta1.SignDoc"
	txRawType      = "cosmos.tx.v1beta1.TxRaw"
	broadcastTx    = "cosmos.tx.v1beta1.Service.BroadcastTx"
	simulateTx     = "cosmos.tx.v1beta1.Service.Simulate"
	signModeDirect = "SIGN_MODE_DIRECT"
)

// BuildAndSign assembles, signs (SIGN_MODE_DIRECT), and serializes a
// transaction. Message types and the transaction scaffolding are resolved
// through the provided Resolver (typically client.Resolver()).
func BuildAndSign(resolver Resolver, signer Signer, chainID string, msgs []Msg, opts TxOptions) (*SignedTx, error) {
	if resolver == nil {
		return nil, errors.New("resolver is required")
	}
	if signer == nil {
		return nil, errors.New("signer is required")
	}
	if strings.TrimSpace(chainID) == "" {
		return nil, errors.New("chainID is required")
	}
	if len(msgs) == 0 {
		return nil, errors.New("at least one message is required")
	}
	for i, raw := range msgs {
		if !json.Valid(raw) {
			return nil, fmt.Errorf("message %d is not valid JSON", i+1)
		}
	}
	for i, coin := range opts.Fee {
		if coin.Denom == "" || coin.Amount == "" {
			return nil, fmt.Errorf("fee coin %d requires both denom and amount", i+1)
		}
	}

	bodyBytes, err := marshalDynamic(resolver, txBodyType, txBodyDoc(msgs, opts))
	if err != nil {
		return nil, err
	}
	authInfoBytes, err := marshalDynamic(resolver, authInfoType, authInfoDoc(signer, opts))
	if err != nil {
		return nil, err
	}
	signDocBytes, err := marshalDynamic(resolver, signDocType, map[string]any{
		"body_bytes":      base64.StdEncoding.EncodeToString(bodyBytes),
		"auth_info_bytes": base64.StdEncoding.EncodeToString(authInfoBytes),
		"chain_id":        chainID,
		"account_number":  strconv.FormatUint(opts.AccountNumber, 10),
	})
	if err != nil {
		return nil, err
	}

	signature, err := signer.Sign(signDocBytes)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	txBytes, err := marshalDynamic(resolver, txRawType, map[string]any{
		"body_bytes":      base64.StdEncoding.EncodeToString(bodyBytes),
		"auth_info_bytes": base64.StdEncoding.EncodeToString(authInfoBytes),
		"signatures":      []string{base64.StdEncoding.EncodeToString(signature)},
	})
	if err != nil {
		return nil, err
	}

	out := &SignedTx{
		BodyBytes:     bodyBytes,
		AuthInfoBytes: authInfoBytes,
		SignDocBytes:  signDocBytes,
		Signature:     signature,
		TxBytes:       txBytes,
		TxHash:        strings.ToUpper(hex.EncodeToString(sha256Sum(txBytes))),
	}
	if opts.AddressPrefix != "" {
		address, err := signer.Address(opts.AddressPrefix)
		if err != nil {
			return nil, err
		}
		out.SignerAddress = address
	}
	return out, nil
}

func txBodyDoc(msgs []Msg, opts TxOptions) map[string]any {
	doc := map[string]any{"messages": msgs}
	if opts.Memo != "" {
		doc["memo"] = opts.Memo
	}
	if opts.TimeoutHeight != 0 {
		doc["timeout_height"] = strconv.FormatUint(opts.TimeoutHeight, 10)
	}
	return doc
}

func authInfoDoc(signer Signer, opts TxOptions) map[string]any {
	coins := make([]map[string]string, 0, len(opts.Fee))
	for _, coin := range opts.Fee {
		if coin.Denom == "" || coin.Amount == "" {
			continue
		}
		coins = append(coins, map[string]string{"denom": coin.Denom, "amount": coin.Amount})
	}
	fee := map[string]any{
		"amount":    coins,
		"gas_limit": strconv.FormatUint(opts.GasLimit, 10),
	}
	if opts.FeePayer != "" {
		fee["payer"] = opts.FeePayer
	}
	if opts.FeeGranter != "" {
		fee["granter"] = opts.FeeGranter
	}
	return map[string]any{
		"signer_infos": []any{
			map[string]any{
				"public_key": map[string]any{
					"@type": signer.PubKeyTypeURL(),
					"key":   base64.StdEncoding.EncodeToString(signer.PublicKey()),
				},
				"mode_info": map[string]any{
					"single": map[string]any{"mode": signModeDirect},
				},
				"sequence": strconv.FormatUint(opts.Sequence, 10),
			},
		},
		"fee": fee,
	}
}

func marshalDynamic(resolver Resolver, fullName string, doc map[string]any) ([]byte, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", fullName, err)
	}
	msgType, err := resolver.FindMessageByName(protoreflect.FullName(fullName))
	if err != nil {
		return nil, fmt.Errorf("resolve %s from reflection: %w", fullName, err)
	}
	msg := dynamicpb.NewMessage(msgType.Descriptor())
	if err := (protojson.UnmarshalOptions{Resolver: resolver}).Unmarshal(raw, msg); err != nil {
		return nil, fmt.Errorf("build %s: %w", fullName, err)
	}
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", fullName, err)
	}
	return encoded, nil
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

// BroadcastMode selects how a transaction is submitted.
type BroadcastMode string

const (
	BroadcastModeSync  BroadcastMode = "BROADCAST_MODE_SYNC"
	BroadcastModeAsync BroadcastMode = "BROADCAST_MODE_ASYNC"
	BroadcastModeBlock BroadcastMode = "BROADCAST_MODE_BLOCK"
)

// BroadcastResponse summarizes a BroadcastTx response. Code 0 means CheckTx
// accepted the transaction; a non-zero code means it was rejected.
type BroadcastResponse struct {
	Code      uint32
	Codespace string
	TxHash    string
	RawLog    string
	Raw       json.RawMessage
}

// Broadcast submits signed transaction bytes via
// cosmos.tx.v1beta1.Service.BroadcastTx. It returns an error if the node
// rejects the transaction (non-zero CheckTx code).
func Broadcast(ctx context.Context, client *libyaci.Client, txBytes []byte, mode BroadcastMode) (*BroadcastResponse, error) {
	if client == nil {
		return nil, errors.New("client is required")
	}
	if len(txBytes) == 0 {
		return nil, errors.New("txBytes is empty")
	}
	if mode == "" {
		mode = BroadcastModeSync
	}
	method, err := client.Method(broadcastTx)
	if err != nil {
		return nil, err
	}
	req := method.NewRequest()
	if err := req.Set("tx_bytes", txBytes); err != nil {
		return nil, fmt.Errorf("set tx_bytes: %w", err)
	}
	if err := req.Set("mode", string(mode)); err != nil {
		return nil, fmt.Errorf("set mode: %w", err)
	}
	resp, err := method.Call(ctx, req)
	if err != nil {
		return nil, err
	}
	data, err := resp.JSON()
	if err != nil {
		return nil, err
	}
	return parseBroadcastResponse(data)
}

func parseBroadcastResponse(data []byte) (*BroadcastResponse, error) {
	result := &BroadcastResponse{Raw: data}
	fields := unwrapTxResponse(data)
	result.Code = uint32(uintField(fields, "code"))
	result.Codespace = stringField(fields, "codespace")
	result.TxHash = stringField(fields, "txhash", "txHash", "hash")
	result.RawLog = stringField(fields, "rawLog", "raw_log", "log")
	if result.Code == 0 {
		return result, nil
	}
	details := []string{fmt.Sprintf("code %d", result.Code)}
	if result.Codespace != "" {
		details = append(details, "codespace "+result.Codespace)
	}
	if result.TxHash != "" {
		details = append(details, "tx "+result.TxHash)
	}
	if log := humanReadableLog(result.RawLog); log != "" {
		details = append(details, log)
	}
	return result, fmt.Errorf("broadcast rejected: %s", strings.Join(details, "; "))
}

// humanReadableLog decodes the base64 form that raw_log takes after the
// descriptor patch (string -> bytes), falling back to the raw value.
func humanReadableLog(raw string) string {
	if raw == "" {
		return ""
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && utf8.Valid(decoded) {
		return string(decoded)
	}
	return raw
}

// Simulate estimates gas for signed transaction bytes via
// cosmos.tx.v1beta1.Service.Simulate.
func Simulate(ctx context.Context, client *libyaci.Client, txBytes []byte) (uint64, error) {
	if client == nil {
		return 0, errors.New("client is required")
	}
	if len(txBytes) == 0 {
		return 0, errors.New("txBytes is empty")
	}
	method, err := client.Method(simulateTx)
	if err != nil {
		return 0, err
	}
	req := method.NewRequest()
	if err := req.Set("tx_bytes", txBytes); err != nil {
		return 0, fmt.Errorf("set tx_bytes: %w", err)
	}
	resp, err := method.Call(ctx, req)
	if err != nil {
		return 0, err
	}
	data, err := resp.JSON()
	if err != nil {
		return 0, err
	}
	return parseSimulateGas(data)
}

func parseSimulateGas(data []byte) (uint64, error) {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return 0, fmt.Errorf("parse simulate response: %w", err)
	}
	gasInfo, _ := decoded["gasInfo"].(map[string]any)
	if gasInfo == nil {
		gasInfo, _ = decoded["gas_info"].(map[string]any)
	}
	if gasInfo != nil {
		if gas, ok := toUint(firstOf(gasInfo, "gasUsed", "gas_used")); ok {
			return gas, nil
		}
	}
	return 0, errors.New("simulate response did not contain gasInfo.gasUsed")
}

const (
	defaultGasAdjustment = 1.3
	// provisionalGasLimit is used when estimating gas with no caller-supplied
	// limit. It must stay below typical per-tx/block gas limits.
	provisionalGasLimit = 1_000_000
)

// EstimateGas builds and signs the transaction with a provisional gas limit,
// simulates it, and returns the gas used. Simulation requires the signer's
// account (and fee payer) to exist on chain; some chains also require it to be
// funded. The returned value is the raw gas used, before any adjustment.
func EstimateGas(ctx context.Context, client *libyaci.Client, signer Signer, chainID string, msgs []Msg, opts TxOptions) (uint64, error) {
	provisional := opts
	if provisional.GasLimit == 0 {
		provisional.GasLimit = provisionalGasLimit
	}
	tx, err := BuildAndSign(client.Resolver(), signer, chainID, msgs, provisional)
	if err != nil {
		return 0, err
	}
	return Simulate(ctx, client, tx.TxBytes)
}

// EstimateFee simulates the transaction, applies a gas adjustment to the gas
// used, and computes the fee from per-gas-unit prices. The returned gas limit
// and fee can be passed straight into BuildAndSign.
//
// gasPrices are price-per-gas-unit amounts, for example
// {Denom: "uatom", Amount: "0.025"}. A non-positive gasAdjustment defaults to
// 1.3. Simulation requires the signer's account to exist on chain; some chains
// also require it to be funded.
func EstimateFee(ctx context.Context, client *libyaci.Client, signer Signer, chainID string, msgs []Msg, opts TxOptions, gasPrices []Coin, gasAdjustment float64) (uint64, []Coin, error) {
	gasUsed, err := EstimateGas(ctx, client, signer, chainID, msgs, opts)
	if err != nil {
		return 0, nil, err
	}
	gasLimit := applyGasAdjustment(gasUsed, gasAdjustment)
	fee, err := FeeFromGasPrices(gasLimit, gasPrices)
	if err != nil {
		return 0, nil, err
	}
	return gasLimit, fee, nil
}

// applyGasAdjustment scales gasUsed by the adjustment, rounding up, with a
// floor of the raw gas used.
func applyGasAdjustment(gasUsed uint64, adjustment float64) uint64 {
	if adjustment <= 0 {
		adjustment = defaultGasAdjustment
	}
	limit := uint64(math.Ceil(float64(gasUsed) * adjustment))
	if limit < gasUsed {
		return gasUsed
	}
	return limit
}

// FeeFromGasPrices computes a fee as ceil(gasLimit * price) for each price,
// using exact rational arithmetic (no floating point).
func FeeFromGasPrices(gasLimit uint64, gasPrices []Coin) ([]Coin, error) {
	if len(gasPrices) == 0 {
		return nil, nil
	}
	fee := make([]Coin, 0, len(gasPrices))
	for i, price := range gasPrices {
		if price.Denom == "" || price.Amount == "" {
			return nil, fmt.Errorf("gas price %d requires both denom and amount", i+1)
		}
		rate, ok := new(big.Rat).SetString(strings.TrimSpace(price.Amount))
		if !ok || rate.Sign() < 0 {
			return nil, fmt.Errorf("invalid gas price %q for %s", price.Amount, price.Denom)
		}
		total := new(big.Rat).Mul(rate, new(big.Rat).SetUint64(gasLimit))
		fee = append(fee, Coin{Denom: price.Denom, Amount: ceilRat(total)})
	}
	return fee, nil
}

func ceilRat(value *big.Rat) string {
	quotient := new(big.Int).Quo(value.Num(), value.Denom())
	remainder := new(big.Int).Rem(value.Num(), value.Denom())
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient.String()
}

// FetchAccountNumberSequence looks up an account's number and sequence, trying
// AccountInfo first and falling back to the Account query.
func FetchAccountNumberSequence(ctx context.Context, client *libyaci.Client, address string) (uint64, uint64, error) {
	if client == nil {
		return 0, 0, errors.New("client is required")
	}
	if strings.TrimSpace(address) == "" {
		return 0, 0, errors.New("address is required")
	}
	if client.SupportsMethod("cosmos.auth.v1beta1.Query.AccountInfo") {
		info, err := client.GetAccountInfo(address)
		if err == nil {
			number, numberErr := strconv.ParseUint(strings.TrimSpace(info.Info.AccountNumber), 10, 64)
			sequence, seqErr := strconv.ParseUint(strings.TrimSpace(info.Info.Sequence), 10, 64)
			if numberErr == nil && seqErr == nil {
				return number, sequence, nil
			}
		}
	}

	resp, err := client.GetAccount(address)
	if err != nil {
		return 0, 0, fmt.Errorf("query account: %w", err)
	}
	return parseAccountSequence(resp.Account)
}

func parseAccountSequence(raw json.RawMessage) (uint64, uint64, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return 0, 0, fmt.Errorf("parse account: %w", err)
	}
	// Module/vesting accounts nest a baseAccount.
	if base, ok := obj["baseAccount"].(map[string]any); ok {
		if number, sequence, ok := accountNumbers(base); ok {
			return number, sequence, nil
		}
	}
	if number, sequence, ok := accountNumbers(obj); ok {
		return number, sequence, nil
	}
	return 0, 0, errors.New("account number and sequence were not found in the account response")
}

func accountNumbers(obj map[string]any) (uint64, uint64, bool) {
	number, numberOK := toUint(firstOf(obj, "accountNumber", "account_number"))
	sequence, seqOK := toUint(firstOf(obj, "sequence"))
	if !numberOK || !seqOK {
		return 0, 0, false
	}
	return number, sequence, true
}

func firstOf(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func unwrapTxResponse(data []byte) map[string]any {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil
	}
	if raw, ok := decoded["txResponse"].(map[string]any); ok {
		return raw
	}
	if raw, ok := decoded["tx_response"].(map[string]any); ok {
		return raw
	}
	return decoded
}

func uintField(values map[string]any, keys ...string) uint64 {
	for _, key := range keys {
		if value, ok := toUint(values[key]); ok {
			return value
		}
	}
	return 0
}

func stringField(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := values[key].(string); ok {
			return strings.TrimSpace(raw)
		}
	}
	return ""
}

func toUint(raw any) (uint64, bool) {
	switch typed := raw.(type) {
	case float64:
		if typed >= 0 {
			return uint64(typed), true
		}
	case string:
		if parsed, err := strconv.ParseUint(strings.TrimSpace(typed), 10, 64); err == nil {
			return parsed, true
		}
	case json.Number:
		if parsed, err := strconv.ParseUint(typed.String(), 10, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

// WaitOptions controls transaction confirmation polling.
type WaitOptions struct {
	PollInterval time.Duration // default 2s
	Timeout      time.Duration // default 60s
}

// TxResult summarizes an included transaction.
type TxResult struct {
	Height int64
	TxHash string
	Code   uint32
	RawLog string
	Raw    json.RawMessage
}

// WaitForTx polls GetTx until the transaction is found (included) or the
// timeout elapses.
func WaitForTx(ctx context.Context, client *libyaci.Client, hash string, opts WaitOptions) (*TxResult, error) {
	if client == nil {
		return nil, errors.New("client is required")
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, errors.New("tx hash is required")
	}
	interval := opts.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	deadline := time.Now().Add(timeout)

	var lastErr error
	for {
		tx, err := client.GetTx(hash)
		if err == nil {
			result, perr := parseTxResult(tx.TxResponse)
			if perr == nil {
				return result, nil
			}
			lastErr = perr
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr == nil {
				lastErr = errors.New("timeout")
			}
			return nil, fmt.Errorf("tx %s not confirmed within %s: %w", hash, timeout, lastErr)
		}
		if err := sleepCtx(ctx, interval); err != nil {
			return nil, err
		}
	}
}

// BroadcastAndConfirm broadcasts with the given mode and, when accepted, waits
// for the transaction to be included.
func BroadcastAndConfirm(ctx context.Context, client *libyaci.Client, txBytes []byte, mode BroadcastMode, opts WaitOptions) (*BroadcastResponse, *TxResult, error) {
	resp, err := Broadcast(ctx, client, txBytes, mode)
	if err != nil {
		return resp, nil, err
	}
	result, err := WaitForTx(ctx, client, resp.TxHash, opts)
	return resp, result, err
}

// SignAndBroadcast fetches the account number/sequence when they are unset,
// builds and signs the transaction, and broadcasts it. If the node rejects it
// for a sequence mismatch, it refreshes the sequence and retries up to `retries`
// times.
func SignAndBroadcast(ctx context.Context, client *libyaci.Client, signer Signer, chainID, addressPrefix string, msgs []Msg, opts TxOptions, mode BroadcastMode, retries uint) (*SignedTx, *BroadcastResponse, error) {
	if client == nil {
		return nil, nil, errors.New("client is required")
	}
	if signer == nil {
		return nil, nil, errors.New("signer is required")
	}
	address, err := signer.Address(addressPrefix)
	if err != nil {
		return nil, nil, err
	}
	if opts.AccountNumber == 0 && opts.Sequence == 0 {
		number, sequence, err := FetchAccountNumberSequence(ctx, client, address)
		if err != nil {
			return nil, nil, err
		}
		opts.AccountNumber, opts.Sequence = number, sequence
	}

	var tx *SignedTx
	var resp *BroadcastResponse
	for attempt := uint(0); ; attempt++ {
		tx, err = BuildAndSign(client.Resolver(), signer, chainID, msgs, opts)
		if err != nil {
			return nil, nil, err
		}
		resp, err = Broadcast(ctx, client, tx.TxBytes, mode)
		if err == nil {
			return tx, resp, nil
		}
		if attempt >= retries || !isSequenceError(err) {
			return tx, resp, err
		}
		_, sequence, fetchErr := FetchAccountNumberSequence(ctx, client, address)
		if fetchErr != nil {
			return tx, resp, err
		}
		opts.Sequence = sequence
	}
}

func isSequenceError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, needle := range []string{
		"account sequence mismatch", "incorrect account sequence",
		"wrong sequence", "sequence mismatch", "invalid sequence",
	} {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseTxResult(raw json.RawMessage) (*TxResult, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("parse tx response: %w", err)
	}
	fields := obj
	if nested, ok := obj["txResponse"].(map[string]any); ok {
		fields = nested
	}
	return &TxResult{
		Height: int64(uintField(fields, "height")),
		TxHash: stringField(fields, "txhash", "txHash", "hash"),
		Code:   uint32(uintField(fields, "code")),
		RawLog: humanReadableLog(stringField(fields, "rawLog", "raw_log", "log")),
		Raw:    raw,
	}, nil
}

// EstimateFeeForAccount estimates gas and fee for an account without its
// private key. Simulation accepts any valid signature, so this presents the
// account's public key in the signer info and signs with a dummy key. The
// account must exist on chain.
//
// pubKeyTypeURL is the account's public-key Any type URL, for example
// "/cosmos.crypto.secp256k1.PubKey" or
// "/ethermint.crypto.v1.ethsecp256k1.PubKey" (from the Account query).
func EstimateFeeForAccount(ctx context.Context, client *libyaci.Client, publicKey []byte, pubKeyTypeURL, addressPrefix, chainID string, msgs []Msg, opts TxOptions, gasPrices []Coin, gasAdjustment float64) (uint64, []Coin, error) {
	if len(publicKey) == 0 {
		return 0, nil, errors.New("public key is required")
	}
	if pubKeyTypeURL == "" {
		return 0, nil, errors.New("pubKeyTypeURL is required")
	}
	inner, err := NewTestSigner(Secp256k1)
	if err != nil {
		return 0, nil, err
	}
	signer := dummySigner{pub: publicKey, inner: inner, url: pubKeyTypeURL}
	return EstimateFee(ctx, client, signer, chainID, msgs, opts, gasPrices, gasAdjustment)
}

// dummySigner presents a caller-supplied public key in the signer info and
// signs with an unrelated key. Only valid for simulation, where the signature
// is not checked against the account.
type dummySigner struct {
	pub   []byte
	inner *PrivateKeySigner
	url   string
}

func (d dummySigner) PublicKey() []byte     { return d.pub }
func (d dummySigner) PubKeyTypeURL() string { return d.url }
func (d dummySigner) Sign(doc []byte) ([]byte, error) {
	return d.inner.Sign(doc)
}
func (d dummySigner) Address(prefix string) (string, error) {
	return AddressFromPublicKey(d.pub, prefix, algoFromTypeURL(d.url))
}

func algoFromTypeURL(url string) KeyAlgorithm {
	if strings.Contains(strings.ToLower(url), "ethsecp256k1") {
		return EthSecp256k1
	}
	return Secp256k1
}

// FetchMinGasPrice queries cosmos.base.node.v1beta1.Service.Config for the
// node's minimum gas price. Many nodes do not expose it, so callers should fall
// back to a configured price when this errors.
func FetchMinGasPrice(ctx context.Context, client *libyaci.Client) ([]Coin, error) {
	if client == nil {
		return nil, errors.New("client is required")
	}
	resp, err := client.Invoke("cosmos.base.node.v1beta1.Service.Config", []byte("{}"))
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(resp, &obj); err != nil {
		return nil, fmt.Errorf("parse node config: %w", err)
	}
	raw := stringField(obj, "minimumGasPrice", "minimum_gas_price")
	if raw == "" {
		return nil, errors.New("node does not expose a minimum gas price")
	}
	return parseDecCoins(raw)
}

// parseDecCoins parses a comma-separated DecCoin list such as
// "0.025uatom,0.001stake" into price coins.
func parseDecCoins(raw string) ([]Coin, error) {
	var coins []Coin
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := 0
		for idx < len(part) && ((part[idx] >= '0' && part[idx] <= '9') || part[idx] == '.') {
			idx++
		}
		if idx == 0 || idx == len(part) {
			return nil, fmt.Errorf("invalid gas price %q", part)
		}
		coins = append(coins, Coin{Denom: part[idx:], Amount: part[:idx]})
	}
	return coins, nil
}
