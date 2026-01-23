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
	method := flag.String("method", "", "method to call (e.g., cosmos.bank.v1beta1.Query.Balance)")
	request := flag.String("request", "", "JSON request body")
	flag.Parse()

	ctx := context.Background()

	// Build client options
	var opts []libyaci.Option
	if *insecure {
		opts = append(opts, libyaci.WithInsecure())
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
		services := client.ListServices()
		fmt.Println("Available services:")
		for _, svc := range services {
			fmt.Printf("  %s\n", svc)
			methods, _ := client.ListMethods(svc)
			for _, m := range methods {
				fmt.Printf("    - %s\n", m)
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

	input, output, err := client.DescribeMethod(*method)
	if err != nil {
		log.Fatalf("Method not found: %v", err)
	}
	fmt.Fprintf(os.Stderr, "  Input:  %s\n", input)
	fmt.Fprintf(os.Stderr, "  Output: %s\n\n", output)

	resp, err := client.Invoke(*method, []byte(*request))
	if err != nil {
		log.Fatalf("Call failed: %v", err)
	}

	// Pretty print the response
	var prettyJSON map[string]interface{}
	if err := json.Unmarshal(resp, &prettyJSON); err != nil {
		fmt.Println(string(resp))
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
	fmt.Println("This client uses gRPC server reflection to call any method without")
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
