package libyaci

import "google.golang.org/grpc"

const (
	defaultMaxRetries     = 3
	defaultMaxRecvMsgSize = 4 * 1024 * 1024 // 4MB
)

type options struct {
	insecure       bool
	maxRetries     uint
	maxRecvMsgSize int
	dialOpts       []grpc.DialOption
}

func defaultOptions() *options {
	return &options{
		insecure:       false,
		maxRetries:     defaultMaxRetries,
		maxRecvMsgSize: defaultMaxRecvMsgSize,
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

// WithDialOptions appends additional gRPC dial options.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(o *options) {
		o.dialOpts = append(o.dialOpts, opts...)
	}
}
