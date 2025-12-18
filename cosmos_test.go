package libyaci

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestParseEarliestFromError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected int64
	}{
		{
			name:     "lowest height pattern",
			errStr:   "height 1 is not available, lowest height is 5200791",
			expected: 5200791,
		},
		{
			name:     "base height pattern",
			errStr:   "block at height 1 not found; base height: 12345678",
			expected: 12345678,
		},
		{
			name:     "earliest available block height pattern",
			errStr:   "earliest available block height is 1000000",
			expected: 1000000,
		},
		{
			name:     "earliest available pattern",
			errStr:   "error: earliest available: 9876543",
			expected: 9876543,
		},
		{
			name:     "min height pattern",
			errStr:   "requested height below pruning limit; min height: 42424242",
			expected: 42424242,
		},
		{
			name:     "no match returns zero",
			errStr:   "some random error message without height info",
			expected: 0,
		},
		{
			name:     "empty string",
			errStr:   "",
			expected: 0,
		},
		{
			name:     "pattern without number",
			errStr:   "lowest height is not a number",
			expected: 0,
		},
		{
			name:     "multiple patterns picks first",
			errStr:   "lowest height is 100, base height: 200",
			expected: 100,
		},
		{
			name:     "height 1 (genesis)",
			errStr:   "lowest height is 1",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEarliestFromError(tt.errStr)
			if got != tt.expected {
				t.Errorf("parseEarliestFromError(%q) = %d, want %d", tt.errStr, got, tt.expected)
			}
		})
	}
}

func TestCosmosMethodConstants(t *testing.T) {
	// Verify method constants are correctly formatted (package.Service.Method)
	methods := []struct {
		name   string
		method string
	}{
		{"GetLatestBlock", methodGetLatestBlock},
		{"GetBlockByHeight", methodGetBlockByHeight},
		{"GetNodeInfo", methodGetNodeInfo},
		{"GetTxsEvent", methodGetTxsEvent},
		{"Balance", methodBalance},
		{"AllBalances", methodAllBalances},
		{"Validators", methodValidators},
		{"StakingParams", methodStakingParams},
		{"DelegatorDelegations", methodDelegatorDelegations},
		{"ModuleAccounts", methodModuleAccounts},
	}

	for _, m := range methods {
		t.Run(m.name, func(t *testing.T) {
			service, method, err := parseMethodFullName(m.method)
			if err != nil {
				t.Errorf("parseMethodFullName(%s) returned error: %v", m.method, err)
				return
			}
			if service == "" {
				t.Errorf("parseMethodFullName(%s) returned empty service", m.method)
			}
			if method == "" {
				t.Errorf("parseMethodFullName(%s) returned empty method", m.method)
			}
		})
	}
}

func TestBlockResponseParsing(t *testing.T) {
	// Test JSON unmarshaling of BlockResponse
	jsonData := []byte(`{
		"block": {
			"header": {
				"height": "12345678",
				"time": "2025-01-15T10:30:00Z",
				"chainId": "cosmoshub-4"
			},
			"data": {
				"txs": ["tx1", "tx2"]
			}
		},
		"blockId": {
			"hash": "ABC123"
		}
	}`)

	var block BlockResponse
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("Failed to unmarshal BlockResponse: %v", err)
	}

	if block.Block.Header.Height != "12345678" {
		t.Errorf("Height = %q, want %q", block.Block.Header.Height, "12345678")
	}
	if block.Block.Header.ChainID != "cosmoshub-4" {
		t.Errorf("ChainID = %q, want %q", block.Block.Header.ChainID, "cosmoshub-4")
	}
	if len(block.Block.Data.Txs) != 2 {
		t.Errorf("Txs count = %d, want 2", len(block.Block.Data.Txs))
	}
}

func TestValidatorsResponseParsing(t *testing.T) {
	jsonData := []byte(`{
		"validators": [
			{
				"operatorAddress": "cosmosvaloper1abc",
				"description": {"moniker": "Validator One"},
				"status": "BOND_STATUS_BONDED",
				"tokens": "1000000000"
			},
			{
				"operatorAddress": "cosmosvaloper1xyz",
				"description": {"moniker": "Validator Two"},
				"status": "BOND_STATUS_BONDED",
				"tokens": "500000000"
			}
		],
		"pagination": {"nextKey": "", "total": "2"}
	}`)

	var resp ValidatorsResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		t.Fatalf("Failed to unmarshal ValidatorsResponse: %v", err)
	}

	if len(resp.Validators) != 2 {
		t.Errorf("Validators count = %d, want 2", len(resp.Validators))
	}
	if resp.Validators[0].OperatorAddress != "cosmosvaloper1abc" {
		t.Errorf("First validator address = %q, want %q", resp.Validators[0].OperatorAddress, "cosmosvaloper1abc")
	}
	if resp.Validators[0].Description.Moniker != "Validator One" {
		t.Errorf("First validator moniker = %q, want %q", resp.Validators[0].Description.Moniker, "Validator One")
	}
}

func TestModuleAccountsResponseParsing(t *testing.T) {
	jsonData := []byte(`{
		"accounts": [
			{
				"@type": "/cosmos.auth.v1beta1.ModuleAccount",
				"baseAccount": {"address": "cosmos1abc123"},
				"name": "distribution",
				"permissions": ["basic"]
			},
			{
				"@type": "/cosmos.auth.v1beta1.ModuleAccount",
				"baseAccount": {"address": "cosmos1xyz789"},
				"name": "staking",
				"permissions": ["burner", "minter"]
			}
		]
	}`)

	var resp ModuleAccountsResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		t.Fatalf("Failed to unmarshal ModuleAccountsResponse: %v", err)
	}

	if len(resp.Accounts) != 2 {
		t.Errorf("Accounts count = %d, want 2", len(resp.Accounts))
	}
	if resp.Accounts[0].Name != "distribution" {
		t.Errorf("First account name = %q, want %q", resp.Accounts[0].Name, "distribution")
	}
	if resp.Accounts[0].BaseAccount.Address != "cosmos1abc123" {
		t.Errorf("First account address = %q, want %q", resp.Accounts[0].BaseAccount.Address, "cosmos1abc123")
	}
}

func TestStakingParamsResponseParsing(t *testing.T) {
	jsonData := []byte(`{
		"params": {
			"bondDenom": "uatom",
			"unbondingTime": "1814400s",
			"maxValidators": 180,
			"maxEntries": 7,
			"historicalEntries": 10000,
			"minCommissionRate": "0.050000000000000000"
		}
	}`)

	var resp StakingParamsResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		t.Fatalf("Failed to unmarshal StakingParamsResponse: %v", err)
	}

	if resp.Params.BondDenom != "uatom" {
		t.Errorf("BondDenom = %q, want %q", resp.Params.BondDenom, "uatom")
	}
	if resp.Params.MaxValidators != 180 {
		t.Errorf("MaxValidators = %d, want 180", resp.Params.MaxValidators)
	}
}

func TestBalanceResponseParsing(t *testing.T) {
	jsonData := []byte(`{
		"balance": {
			"denom": "uatom",
			"amount": "1234567890"
		}
	}`)

	var resp BalanceResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		t.Fatalf("Failed to unmarshal BalanceResponse: %v", err)
	}

	if resp.Balance.Denom != "uatom" {
		t.Errorf("Denom = %q, want %q", resp.Balance.Denom, "uatom")
	}
	if resp.Balance.Amount != "1234567890" {
		t.Errorf("Amount = %q, want %q", resp.Balance.Amount, "1234567890")
	}
}

func TestDelegationsResponseParsing(t *testing.T) {
	jsonData := []byte(`{
		"delegationResponses": [
			{
				"delegation": {
					"delegatorAddress": "cosmos1abc",
					"validatorAddress": "cosmosvaloper1xyz",
					"shares": "1000000.000000000000000000"
				},
				"balance": {
					"denom": "uatom",
					"amount": "1000000"
				}
			}
		],
		"pagination": {"nextKey": "", "total": "1"}
	}`)

	var resp DelegationsResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		t.Fatalf("Failed to unmarshal DelegationsResponse: %v", err)
	}

	if len(resp.DelegationResponses) != 1 {
		t.Errorf("DelegationResponses count = %d, want 1", len(resp.DelegationResponses))
	}
	if resp.DelegationResponses[0].Delegation.DelegatorAddress != "cosmos1abc" {
		t.Errorf("DelegatorAddress = %q, want %q", resp.DelegationResponses[0].Delegation.DelegatorAddress, "cosmos1abc")
	}
	if resp.DelegationResponses[0].Balance.Amount != "1000000" {
		t.Errorf("Balance amount = %q, want %q", resp.DelegationResponses[0].Balance.Amount, "1000000")
	}
}

// Integration tests - skip if no live server available
// To run: LIBYACI_TEST_ENDPOINT=cosmos-grpc.publicnode.com:443 go test -v ./...

func getTestEndpoint() string {
	if ep := os.Getenv("LIBYACI_TEST_ENDPOINT"); ep != "" {
		return ep
	}
	return ""
}

func TestIntegrationGetLatestBlock(t *testing.T) {
	endpoint := getTestEndpoint()
	if endpoint == "" {
		t.Skip("LIBYACI_TEST_ENDPOINT not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := Dial(ctx, endpoint, WithDialTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	block, err := client.GetLatestBlock()
	if err != nil {
		t.Fatalf("GetLatestBlock failed: %v", err)
	}

	if block.Block.Header.Height == "" {
		t.Error("Expected non-empty block height")
	}

	t.Logf("Latest block height: %s", block.Block.Header.Height)
}

func TestIntegrationGetChainID(t *testing.T) {
	endpoint := getTestEndpoint()
	if endpoint == "" {
		t.Skip("LIBYACI_TEST_ENDPOINT not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := Dial(ctx, endpoint, WithDialTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	chainID, err := client.GetChainID()
	if err != nil {
		t.Fatalf("GetChainID failed: %v", err)
	}

	if chainID == "" {
		t.Error("Expected non-empty chain ID")
	}

	t.Logf("Chain ID: %s", chainID)
}

func TestIntegrationGetAllValidators(t *testing.T) {
	endpoint := getTestEndpoint()
	if endpoint == "" {
		t.Skip("LIBYACI_TEST_ENDPOINT not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := Dial(ctx, endpoint, WithDialTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	validators, err := client.GetAllValidators()
	if err != nil {
		t.Fatalf("GetAllValidators failed: %v", err)
	}

	if len(validators) == 0 {
		t.Error("Expected at least one validator")
	}

	t.Logf("Found %d validators", len(validators))
}

func TestIntegrationGetModuleAccounts(t *testing.T) {
	endpoint := getTestEndpoint()
	if endpoint == "" {
		t.Skip("LIBYACI_TEST_ENDPOINT not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := Dial(ctx, endpoint, WithDialTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	modules, err := client.GetModuleAccounts()
	if err != nil {
		t.Fatalf("GetModuleAccounts failed: %v", err)
	}

	if len(modules) == 0 {
		t.Error("Expected at least one module account")
	}

	t.Logf("Found %d module accounts", len(modules))
	for addr, name := range modules {
		t.Logf("  %s: %s", name, addr)
	}
}

func TestIntegrationGetBondDenom(t *testing.T) {
	endpoint := getTestEndpoint()
	if endpoint == "" {
		t.Skip("LIBYACI_TEST_ENDPOINT not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := Dial(ctx, endpoint, WithDialTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer client.Close()

	denom, err := client.GetBondDenom()
	if err != nil {
		t.Fatalf("GetBondDenom failed: %v", err)
	}

	if denom == "" {
		t.Error("Expected non-empty bond denom")
	}

	t.Logf("Bond denom: %s", denom)
}
