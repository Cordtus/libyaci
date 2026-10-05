// Example demonstrates how to use libyaci to call Cosmos SDK gRPC methods
// without any precompiled protobuf stubs.
//
// Run with: go run ./examples/cli -addr localhost:9090 -insecure
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Cordtus/libyaci"
)

func main() {
	addr := flag.String("addr", "localhost:9090", "gRPC server address")
	insecure := flag.Bool("insecure", false, "skip TLS verification")
	listSvcs := flag.Bool("list", false, "list all available services")
	jsonOut := flag.Bool("json", false, "emit machine-readable JSON for -list")
	method := flag.String("method", "", "method to call (e.g., cosmos.bank.v1beta1.Query.Balance)")
	request := flag.String("request", "", "JSON request body")
	protoDir := flag.String("protodir", "", "directory of .proto files used as fallback for deprecated types")
	flag.Parse()

	ctx := context.Background()

	// Build client options
	var opts []libyaci.Option
	if *insecure {
		opts = append(opts, libyaci.WithInsecure())
	}
	if *protoDir != "" {
		opts = append(opts, libyaci.WithProtoDir(*protoDir))
	}

	// Connect and fetch all proto descriptors
	fmt.Fprintf(os.Stderr, "Connecting to %s...\n", *addr)
	client, err := libyaci.Dial(ctx, *addr, opts...)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	fmt.Fprintf(os.Stderr, "Connected successfully\n\n")

	// List services mode
	if *listSvcs {
		services := client.Catalog().Services()
		if *jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(services); err != nil {
				log.Fatalf("Failed to encode services: %v", err)
			}
			return
		}
		fmt.Println("Available services:")
		for _, svc := range services {
			fmt.Printf("  %s\n", svc.Name)
			for _, m := range svc.Methods {
				fmt.Printf("    - %s (%s -> %s)\n", m.Name, m.Input, m.Output)
			}
		}
		return
	}

	// If no method specified, show some examples
	if *method == "" {
		showExamples(client)
		return
	}

	// Call the specified method
	fmt.Fprintf(os.Stderr, "Calling %s...\n", *method)

	handle, err := client.Method(*method)
	if err != nil {
		log.Fatalf("Method not found: %v", err)
	}
	info := handle.Info()
	fmt.Fprintf(os.Stderr, "  Input:  %s\n", info.Input)
	fmt.Fprintf(os.Stderr, "  Output: %s\n\n", info.Output)

	req := handle.NewRequest()
	if *request != "" {
		if err := req.LoadJSON([]byte(*request)); err != nil {
			log.Fatalf("Invalid request: %v", err)
		}
	}
	resp, err := handle.Call(ctx, req)
	if err != nil {
		log.Fatalf("Call failed: %v", err)
	}
	respJSON, err := resp.JSON()
	if err != nil {
		log.Fatalf("JSON marshal failed: %v", err)
	}

	// Pretty print the response
	var prettyJSON map[string]interface{}
	if err := json.Unmarshal(respJSON, &prettyJSON); err != nil {
		fmt.Println(string(respJSON))
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(prettyJSON)
	}
}

func showExamples(client *libyaci.Client) {
	fmt.Println("libyaci - Dynamic gRPC Client")
	fmt.Println("=============================")
	fmt.Println()
	fmt.Println("This client uses gRPC server reflection to call advertised methods without")
	fmt.Println("requiring precompiled protobuf stubs.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println()

	// Try to find some common Cosmos methods
	examples := []struct {
		method  string
		request string
		desc    string
	}{
		{
			method:  "cosmos.base.tendermint.v1beta1.Service.GetLatestBlock",
			request: "{}",
			desc:    "Get the latest block",
		},
		{
			method:  "cosmos.base.tendermint.v1beta1.Service.GetNodeInfo",
			request: "{}",
			desc:    "Get node information",
		},
		{
			method:  "cosmos.bank.v1beta1.Query.TotalSupply",
			request: "{}",
			desc:    "Get total token supply",
		},
		{
			method:  "cosmos.staking.v1beta1.Query.Pool",
			request: "{}",
			desc:    "Get staking pool info",
		},
	}

	services := client.ListServices()
	serviceSet := make(map[string]bool)
	for _, s := range services {
		serviceSet[s] = true
	}

	for _, ex := range examples {
		parts := strings.Split(ex.method, ".")
		serviceName := strings.Join(parts[:len(parts)-1], ".")
		if serviceSet[serviceName] {
			fmt.Printf("  # %s\n", ex.desc)
			fmt.Printf("  -method '%s' -request '%s'\n\n", ex.method, ex.request)
		}
	}

	fmt.Println("Use -list to see all available services and methods")
}
