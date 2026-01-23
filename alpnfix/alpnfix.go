// Package alpnfix provides a workaround for grpc-go's ALPN enforcement.
//
// Starting with grpc-go v1.67, ALPN (Application-Layer Protocol Negotiation)
// is enforced by default during TLS handshakes. Some gRPC servers don't properly
// support ALPN negotiation, causing connection failures with errors like:
//
//	"transport: authentication handshake failed: credentials: cannot check peer:
//	missing selected ALPN property"
//
// This package disables ALPN enforcement by setting the GRPC_ENFORCE_ALPN_ENABLED
// environment variable to "false" at init time, before grpc-go reads it.
//
// # Usage
//
// Import this package with a blank identifier before any grpc imports:
//
//	import (
//	    _ "github.com/Cordtus/libyaci/alpnfix" // Must be first!
//
//	    "github.com/Cordtus/libyaci"
//	)
//
// Alternatively, set the environment variable before running your program:
//
//	GRPC_ENFORCE_ALPN_ENABLED=false ./your-program
//
// # How It Works
//
// Go's init functions run in dependency order. By importing this package before
// grpc-go (directly or transitively), the environment variable is set before
// grpc-go's internal envconfig package caches it.
//
// # Caveats
//
// This affects all gRPC connections in the process. If you need ALPN enforcement
// for some connections, use the environment variable approach with selective
// program invocation instead.
package alpnfix

import "os"

func init() {
	// Set before grpc-go's envconfig package reads it
	os.Setenv("GRPC_ENFORCE_ALPN_ENABLED", "false")
}
