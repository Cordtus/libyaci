// Demo: Multi-Chain Explorer
//
// This demo showcases the benefits of libyaci over traditional gRPC client approaches.
// With traditional methods, supporting multiple chains would require:
//   - Importing proto packages for each chain's custom modules
//   - Managing version conflicts between different SDK versions
//   - Writing separate client code for each chain's unique features
//   - Recompiling whenever a chain updates its protos
//
// With libyaci, we connect to any reflection-enabled Cosmos chain and query
// advertised modules without generated chain-specific dependencies.
//
// Usage:
//
//	go run . -addr grpc.osmosis.zone:9090
//	go run . -addr cosmos-grpc.polkachu.com:14990 -insecure
//	go run . -addr grpc.celestia.nodestake.top:443
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Cordtus/libyaci"
)

func main() {
	addr := flag.String("addr", "", "gRPC endpoint (e.g., grpc.osmosis.zone:9090)")
	insecure := flag.Bool("insecure", false, "skip TLS verification")
	timeout := flag.Duration("timeout", 30*time.Second, "connection timeout")
	flag.Parse()

	if *addr == "" {
		fmt.Println("Usage: explorer -addr <grpc-endpoint> [-insecure]")
		fmt.Println("\nExamples:")
		fmt.Println("  explorer -addr grpc.osmosis.zone:9090")
		fmt.Println("  explorer -addr cosmos-grpc.polkachu.com:14990")
		fmt.Println("  explorer -addr grpc.celestia.nodestake.top:443")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Build options
	var opts []libyaci.Option
	if *insecure {
		opts = append(opts, libyaci.WithInsecure())
	}
	opts = append(opts, libyaci.WithMaxRecvMsgSize(20*1024*1024)) // 20MB for large responses

	fmt.Printf("Connecting to %s...\n", *addr)
	start := time.Now()

	client, err := libyaci.Dial(ctx, *addr, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	fmt.Printf("Connected in %v\n\n", time.Since(start).Round(time.Millisecond))

	// Run the demo
	runDemo(client)
}

func runDemo(client *libyaci.Client) {
	printHeader("LIBYACI DEMO: Multi-Chain Explorer")
	fmt.Println("This demo shows how libyaci can query ANY Cosmos chain without")
	fmt.Println("pre-compiled protobuf stubs or chain-specific dependencies.")
	fmt.Println()

	// 1. Node Info
	printSection("1. Node Information")
	showNodeInfo(client)

	// 2. Service Discovery
	printSection("2. Available Services (via gRPC Reflection)")
	showServices(client)

	// 3. Chain Parameters
	printSection("3. Chain Status")
	showChainStatus(client)

	// 4. Token Supply
	printSection("4. Token Economics")
	showTokenSupply(client)

	// 5. Staking Info
	printSection("5. Staking Overview")
	showStakingInfo(client)

	// 6. Governance
	printSection("6. Governance")
	showGovernance(client)

	// 7. IBC Channels (if available)
	printSection("7. IBC Connectivity")
	showIBCInfo(client)

	// 8. Custom Module Detection
	printSection("8. Chain-Specific Modules")
	showCustomModules(client)

	printHeader("Demo Complete")
	fmt.Println("All queries executed dynamically - no compiled protos needed!")
}

func showNodeInfo(client *libyaci.Client) {
	resp, err := client.Invoke("cosmos.base.tendermint.v1beta1.Service.GetNodeInfo", nil)
	if err != nil {
		fmt.Printf("  Could not fetch node info: %v\n", err)
		return
	}

	var result struct {
		DefaultNodeInfo struct {
			Network string `json:"network"`
			Version string `json:"version"`
			Moniker string `json:"moniker"`
		} `json:"defaultNodeInfo"`
		ApplicationVersion struct {
			Name             string `json:"name"`
			AppName          string `json:"appName"`
			Version          string `json:"version"`
			GoVersion        string `json:"goVersion"`
			CosmosSdkVersion string `json:"cosmosSdkVersion"`
		} `json:"applicationVersion"`
	}
	json.Unmarshal(resp, &result)

	appName := result.ApplicationVersion.Name
	if result.ApplicationVersion.AppName != "" {
		appName = result.ApplicationVersion.AppName
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Chain ID:\t%s\n", result.DefaultNodeInfo.Network)
	fmt.Fprintf(w, "  Node Moniker:\t%s\n", result.DefaultNodeInfo.Moniker)
	fmt.Fprintf(w, "  App Name:\t%s\n", appName)
	fmt.Fprintf(w, "  App Version:\t%s\n", result.ApplicationVersion.Version)
	fmt.Fprintf(w, "  SDK Version:\t%s\n", result.ApplicationVersion.CosmosSdkVersion)
	fmt.Fprintf(w, "  Go Version:\t%s\n", result.ApplicationVersion.GoVersion)
	w.Flush()
}

func showServices(client *libyaci.Client) {
	services := client.ListServices()

	// Group by module
	modules := make(map[string][]string)
	for _, svc := range services {
		parts := strings.Split(svc, ".")
		if len(parts) >= 2 {
			module := parts[0]
			if parts[0] == "cosmos" || parts[0] == "ibc" {
				if len(parts) >= 3 {
					module = parts[0] + "." + parts[1]
				}
			}
			modules[module] = append(modules[module], svc)
		}
	}

	// Sort and display
	var keys []string
	for k := range modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Printf("  Found %d services across %d modules:\n\n", len(services), len(modules))

	for _, module := range keys {
		svcs := modules[module]
		fmt.Printf("  %s (%d services)\n", module, len(svcs))
	}
}

func showChainStatus(client *libyaci.Client) {
	resp, err := client.Invoke("cosmos.base.tendermint.v1beta1.Service.GetLatestBlock", nil)
	if err != nil {
		fmt.Printf("  Could not fetch latest block: %v\n", err)
		return
	}

	var result struct {
		Block struct {
			Header struct {
				Height  string `json:"height"`
				Time    string `json:"time"`
				ChainID string `json:"chain_id"`
			} `json:"header"`
		} `json:"block"`
	}
	json.Unmarshal(resp, &result)

	blockTime, _ := time.Parse(time.RFC3339Nano, result.Block.Header.Time)
	age := time.Since(blockTime).Round(time.Second)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Latest Block:\t%s\n", result.Block.Header.Height)
	fmt.Fprintf(w, "  Block Time:\t%s\n", blockTime.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(w, "  Block Age:\t%s ago\n", age)
	w.Flush()
}

func showTokenSupply(client *libyaci.Client) {
	resp, err := client.Invoke("cosmos.bank.v1beta1.Query.TotalSupply", []byte(`{"pagination":{"limit":"10"}}`))
	if err != nil {
		fmt.Printf("  Could not fetch supply: %v\n", err)
		return
	}

	var result struct {
		Supply []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"supply"`
	}
	json.Unmarshal(resp, &result)

	if len(result.Supply) == 0 {
		fmt.Println("  No supply data available")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  DENOM\tSUPPLY\n")
	fmt.Fprintf(w, "  -----\t------\n")

	for i, coin := range result.Supply {
		if i >= 5 {
			fmt.Fprintf(w, "  ... and %d more tokens\t\n", len(result.Supply)-5)
			break
		}
		// Format large numbers
		amount := formatAmount(coin.Amount)
		denom := truncate(coin.Denom, 30)
		fmt.Fprintf(w, "  %s\t%s\n", denom, amount)
	}
	w.Flush()
}

func showStakingInfo(client *libyaci.Client) {
	// Get staking pool
	poolResp, err := client.Invoke("cosmos.staking.v1beta1.Query.Pool", nil)
	if err != nil {
		fmt.Printf("  Could not fetch staking pool: %v\n", err)
		return
	}

	var poolResult struct {
		Pool struct {
			BondedTokens    string `json:"bondedTokens"`
			NotBondedTokens string `json:"notBondedTokens"`
		} `json:"pool"`
	}
	json.Unmarshal(poolResp, &poolResult)

	// Get validators count
	valsResp, err := client.Invoke("cosmos.staking.v1beta1.Query.Validators",
		[]byte(`{"status":"BOND_STATUS_BONDED","pagination":{"countTotal":true,"limit":"1"}}`))

	var valsResult struct {
		Pagination struct {
			Total string `json:"total"`
		} `json:"pagination"`
	}
	if err == nil {
		json.Unmarshal(valsResp, &valsResult)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Bonded Tokens:\t%s\n", formatAmount(poolResult.Pool.BondedTokens))
	fmt.Fprintf(w, "  Unbonded Tokens:\t%s\n", formatAmount(poolResult.Pool.NotBondedTokens))
	if valsResult.Pagination.Total != "" {
		fmt.Fprintf(w, "  Active Validators:\t%s\n", valsResult.Pagination.Total)
	}
	w.Flush()
}

func showGovernance(client *libyaci.Client) {
	// Try v1 first, fall back to v1beta1
	resp, err := client.Invoke("cosmos.gov.v1.Query.Proposals",
		[]byte(`{"proposal_status":"PROPOSAL_STATUS_VOTING_PERIOD","pagination":{"limit":"5"}}`))

	if err != nil {
		// Try v1beta1
		resp, err = client.Invoke("cosmos.gov.v1beta1.Query.Proposals",
			[]byte(`{"proposal_status":"2","pagination":{"limit":"5"}}`))
	}

	if err != nil {
		fmt.Printf("  Could not fetch proposals: %v\n", err)
		return
	}

	var result struct {
		Proposals []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Status  string `json:"status"`
			Content struct {
				Title string `json:"title"`
			} `json:"content"`
			Messages []struct {
				Type string `json:"@type"`
			} `json:"messages"`
		} `json:"proposals"`
	}
	json.Unmarshal(resp, &result)

	if len(result.Proposals) == 0 {
		fmt.Println("  No active proposals in voting period")
		return
	}

	fmt.Printf("  Found %d proposal(s) in voting period:\n\n", len(result.Proposals))
	for _, prop := range result.Proposals {
		title := prop.Title
		if title == "" && prop.Content.Title != "" {
			title = prop.Content.Title
		}
		if title == "" && len(prop.Messages) > 0 {
			title = prop.Messages[0].Type
		}
		fmt.Printf("  #%s: %s\n", prop.ID, truncate(title, 60))
	}
}

func showIBCInfo(client *libyaci.Client) {
	resp, err := client.Invoke("ibc.core.channel.v1.Query.Channels",
		[]byte(`{"pagination":{"limit":"100","countTotal":true}}`))

	if err != nil {
		fmt.Println("  IBC not available or no channels configured")
		return
	}

	var result struct {
		Channels []struct {
			State        string `json:"state"`
			ChannelID    string `json:"channelId"`
			PortID       string `json:"portId"`
			Counterparty struct {
				ChannelID string `json:"channelId"`
				PortID    string `json:"portId"`
			} `json:"counterparty"`
		} `json:"channels"`
		Pagination struct {
			Total string `json:"total"`
		} `json:"pagination"`
	}
	json.Unmarshal(resp, &result)

	// Count by state
	openCount := 0
	for _, ch := range result.Channels {
		if ch.State == "STATE_OPEN" {
			openCount++
		}
	}

	total := result.Pagination.Total
	if total == "" {
		total = fmt.Sprintf("%d", len(result.Channels))
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  Total Channels:\t%s\n", total)
	fmt.Fprintf(w, "  Open Channels:\t%d\n", openCount)
	w.Flush()

	// Show a few example channels
	if len(result.Channels) > 0 {
		fmt.Println("\n  Recent channels:")
		shown := 0
		for _, ch := range result.Channels {
			if ch.State == "STATE_OPEN" && shown < 3 {
				fmt.Printf("    %s (%s) <-> %s\n", ch.ChannelID, ch.PortID, ch.Counterparty.ChannelID)
				shown++
			}
		}
	}
}

func showCustomModules(client *libyaci.Client) {
	services := client.ListServices()

	// Known standard modules
	standard := map[string]bool{
		"cosmos": true, "ibc": true, "grpc": true, "tendermint": true,
	}

	var custom []string
	for _, svc := range services {
		parts := strings.Split(svc, ".")
		if len(parts) > 0 && !standard[parts[0]] {
			// Check if we haven't already added this prefix
			prefix := parts[0]
			found := false
			for _, c := range custom {
				if strings.HasPrefix(c, prefix) {
					found = true
					break
				}
			}
			if !found {
				custom = append(custom, svc)
			}
		}
	}

	if len(custom) == 0 {
		fmt.Println("  No chain-specific custom modules detected")
		fmt.Println("  (This is a standard Cosmos SDK chain)")
		return
	}

	fmt.Printf("  Detected %d chain-specific module(s):\n\n", len(custom))

	// Group by top-level namespace
	namespaces := make(map[string]int)
	for _, svc := range services {
		parts := strings.Split(svc, ".")
		if len(parts) > 0 && !standard[parts[0]] {
			namespaces[parts[0]]++
		}
	}

	for ns, count := range namespaces {
		fmt.Printf("  - %s (%d services)\n", ns, count)

		// Show example queries for known chains
		switch ns {
		case "osmosis":
			fmt.Println("    Example: Pool liquidity, swap routes, incentives")
		case "celestia":
			fmt.Println("    Example: Blob submissions, data availability")
		case "injective":
			fmt.Println("    Example: Derivatives, exchange, oracle")
		case "akash":
			fmt.Println("    Example: Deployments, providers, marketplace")
		case "stride":
			fmt.Println("    Example: Liquid staking, host zones")
		case "neutron":
			fmt.Println("    Example: Interchain queries, contracts")
		}
	}

	fmt.Println("\n  With libyaci, you can query these custom modules")
	fmt.Println("  without importing their proto definitions!")
}

// Utility functions

func printHeader(title string) {
	line := strings.Repeat("=", len(title)+4)
	fmt.Printf("\n%s\n  %s\n%s\n\n", line, title, line)
}

func printSection(title string) {
	fmt.Printf("\n%s\n%s\n", title, strings.Repeat("-", len(title)))
}

func formatAmount(s string) string {
	if s == "" {
		return "0"
	}
	// For very large numbers, use scientific notation or abbreviations
	if len(s) > 15 {
		return s[:len(s)-15] + "." + s[len(s)-15:len(s)-13] + "Q" // Quadrillion
	}
	if len(s) > 12 {
		return s[:len(s)-12] + "." + s[len(s)-12:len(s)-10] + "T" // Trillion
	}
	if len(s) > 9 {
		return s[:len(s)-9] + "." + s[len(s)-9:len(s)-7] + "B" // Billion
	}
	if len(s) > 6 {
		return s[:len(s)-6] + "." + s[len(s)-6:len(s)-4] + "M" // Million
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
