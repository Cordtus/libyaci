package libyaci

import (
	"time"

	"google.golang.org/grpc"
)

const (
	defaultMaxRetries     = 3
	defaultMaxRecvMsgSize = 4 * 1024 * 1024 // 4MB
)

type options struct {
	insecure          bool
	maxRetries        uint
	maxRecvMsgSize    int
	dialTimeout       time.Duration
	dialOpts          []grpc.DialOption
	fallback          *FallbackRegistry
	useGlobalFallback bool
	protoDir          string // path to local proto files for fallback
}

func defaultOptions() *options {
	return &options{
		insecure:       false,
		maxRetries:     defaultMaxRetries,
		maxRecvMsgSize: defaultMaxRecvMsgSize,
		dialTimeout:    0, // no timeout by default (uses context deadline)
	}
}

// Option configures the Client.
type Option func(*options)

// WithInsecure disables TLS for the connection.
func WithInsecure() Option {
	return func(o *options) {
		o.insecure = true
	}
}

// WithMaxRetries sets the maximum number of retries for failed calls.
func WithMaxRetries(n uint) Option {
	return func(o *options) {
		o.maxRetries = n
	}
}

// WithMaxRecvMsgSize sets the maximum message size the client can receive.
func WithMaxRecvMsgSize(size int) Option {
	return func(o *options) {
		o.maxRecvMsgSize = size
	}
}

// WithDialTimeout sets a timeout for the initial connection and descriptor fetching.
// If not set, the timeout is controlled by the context passed to Dial().
func WithDialTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.dialTimeout = timeout
	}
}

// WithDialOptions appends additional gRPC dial options.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(o *options) {
		o.dialOpts = append(o.dialOpts, opts...)
	}
}

// WithFallbackRegistry sets a custom fallback registry for resolving types
// that are not available via server reflection (e.g., deprecated modules).
func WithFallbackRegistry(fb *FallbackRegistry) Option {
	return func(o *options) {
		o.fallback = fb
	}
}

// WithGlobalFallback uses the global fallback registry (shared across clients).
// This is useful when multiple clients need to share the same fallback descriptors.
func WithGlobalFallback() Option {
	return func(o *options) {
		o.useGlobalFallback = true
	}
}

// WithProtoDir configures a directory containing .proto files to use as
// fallback when server reflection fails to resolve a type. Proto files
// are compiled and registered, providing definitions for deprecated types.
//
// The directory should contain .proto files with proper package declarations.
// Imports are resolved relative to the directory root, and standard protobuf
// imports (google/protobuf/*) are automatically available.
//
// Proto files are loaded lazily on first type resolution failure.
//
// Example:
//
//	client, err := libyaci.Dial(ctx, addr,
//	    libyaci.WithProtoDir("/path/to/protos"),
//	)
//
// When a type like "tendermint.liquidity.v1beta1.MsgSwapWithinBatch" cannot
// be resolved via server reflection, the client will look for it in the
// local proto files.
//
// Directory structure should mirror the proto package path:
//
//	protos/
//	  tendermint/liquidity/v1beta1/tx.proto
//	  cosmos/base/v1beta1/coin.proto
func WithProtoDir(path string) Option {
	return func(o *options) {
		o.protoDir = path
		// Enable global fallback if not already configured
		if o.fallback == nil && !o.useGlobalFallback {
			o.useGlobalFallback = true
		}
	}
}
