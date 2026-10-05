package libyaci

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Client is a dynamic gRPC client that uses server reflection to invoke methods.
type Client struct {
	conn     *grpc.ClientConn
	ctx      context.Context
	cancel   context.CancelFunc
	resolver *Resolver
	opts     *options
	catalog  *Catalog
	skipped  []string // reflected files that could not be registered
}

// Dial creates a new reflection-based gRPC client connected to the specified address.
// The client will automatically fetch proto descriptors from the server using reflection.
func Dial(ctx context.Context, address string, opts ...Option) (*Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	// Create a client lifetime context independent from the dial/init context.
	// A caller may pass a timeout context to Dial; that deadline must not cancel
	// all future RPCs after initialization succeeds.
	clientCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))

	// Fail fast on a misconfigured proto directory instead of surfacing the
	// error later, only if a type happens to miss reflection.
	if o.protoDir != "" {
		if err := validateProtoDir(o.protoDir); err != nil {
			cancel()
			return nil, err
		}
	}

	// Create a separate context for dial/init operations with optional timeout
	var initCtx context.Context
	var initCancel context.CancelFunc
	if o.dialTimeout > 0 {
		initCtx, initCancel = context.WithTimeout(ctx, o.dialTimeout)
	} else {
		initCtx, initCancel = context.WithCancel(ctx)
	}
	defer initCancel()

	conn, err := dial(initCtx, address, o)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Fetch all descriptors from the server
	descriptors, advertisedServices, err := fetchDescriptorSnapshot(initCtx, conn, o.maxRetries)
	if err != nil {
		conn.Close()
		cancel()
		return nil, fmt.Errorf("failed to fetch descriptors: %w", err)
	}

	// Build the file descriptor registry. Individual files that cannot be
	// registered are skipped and recorded so one malformed reflected file does
	// not make the whole client unusable.
	files, skippedFiles, err := buildFileDescriptorSetReport(descriptors)
	if err != nil {
		conn.Close()
		cancel()
		return nil, fmt.Errorf("failed to build descriptor set: %w", err)
	}

	// Setup fallback registry. A configured proto directory is attached to a
	// clone so per-client settings never mutate a shared or caller-owned
	// registry.
	var fallback *FallbackRegistry
	switch {
	case o.fallback != nil:
		fallback = o.fallback
	case o.useGlobalFallback:
		fallback = GlobalFallback()
	}

	if o.protoDir != "" {
		base := fallback
		if base == nil {
			base = NewFallbackRegistry()
		}
		fallback = base.clone()
		fallback.SetProtoDir(NewProtoDir(o.protoDir))
	}

	resolver := newResolver(clientCtx, files, conn, o.maxRetries, fallback)
	catalog := newCatalog(files, advertisedServices, ChainInfo{
		SDKVersion:        o.sdkVersion,
		MinSDKVersion:     o.minSDKVersion,
		ReflectionVersion: reflectionVersionName(getConnVersion(conn)),
	})

	return &Client{
		conn:     conn,
		ctx:      clientCtx,
		cancel:   cancel,
		resolver: resolver,
		opts:     o,
		catalog:  catalog,
		skipped:  skippedFiles,
	}, nil
}

// Invoke calls the specified gRPC method with the given JSON request payload.
// The method should be specified as "package.Service.Method" (e.g., "cosmos.bank.v1beta1.Query.Balance").
// Returns the response as JSON bytes.
func (c *Client) Invoke(method string, request []byte) ([]byte, error) {
	return c.InvokeWithRetry(method, request, c.opts.maxRetries)
}

// InvokeContext calls the specified gRPC method with a caller-controlled context.
func (c *Client) InvokeContext(ctx context.Context, method string, request []byte) ([]byte, error) {
	return c.InvokeWithRetryContext(ctx, method, request, c.opts.maxRetries)
}

// InvokeWithRetry calls the specified gRPC method with custom retry count.
func (c *Client) InvokeWithRetry(method string, request []byte, maxRetries uint) ([]byte, error) {
	return c.InvokeWithRetryContext(c.ctx, method, request, maxRetries)
}

// InvokeWithTimeout calls the method with a per-invocation timeout that also
// bounds retries and backoff. A timeout <= 0 falls back to the default.
func (c *Client) InvokeWithTimeout(method string, request []byte, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		return c.InvokeWithRetryContext(c.ctx, method, request, c.opts.maxRetries)
	}
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return c.InvokeWithRetryContext(ctx, method, request, c.opts.maxRetries)
}

// InvokeWithRetryContext calls the specified gRPC method with custom retry count
// and a caller-controlled context.
func (c *Client) InvokeWithRetryContext(ctx context.Context, method string, request []byte, maxRetries uint) ([]byte, error) {
	methodDesc, err := c.methodDescriptor(method)
	if err != nil {
		return nil, err
	}

	requestPayload, err := c.prepareRequest(methodDesc, request)
	if err != nil {
		return nil, err
	}

	fullMethodPath := buildFullMethodPath(methodDesc)
	callCtx, cancel := c.withClientContext(ctx)
	defer cancel()

	var resp []byte
	var lastErr error
	maxAttempts := maxRetries
	if maxAttempts == 0 {
		maxAttempts = 1
	}

	attempts := uint(0)
	for attempt := uint(1); attempt <= maxAttempts; attempt++ {
		attempts = attempt
		resp, lastErr = c.invokeOnce(callCtx, fullMethodPath, methodDesc, requestPayload)
		if lastErr == nil {
			return resp, nil
		}
		if attempt < maxAttempts && isRetryableError(lastErr) {
			if err := sleepContext(callCtx, time.Duration(2*attempt)*time.Second); err != nil {
				return nil, err
			}
			continue
		}
		break
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", attempts, lastErr)
}

// InvokeRaw calls the method and returns the dynamic protobuf message directly.
// This is useful when you need to access specific fields without JSON marshaling.
func (c *Client) InvokeRaw(method string, request []byte) (*dynamicpb.Message, error) {
	return c.InvokeRawContext(c.ctx, method, request)
}

// InvokeRawContext calls the method and returns the dynamic protobuf message
// directly using a caller-controlled context.
func (c *Client) InvokeRawContext(ctx context.Context, method string, request []byte) (*dynamicpb.Message, error) {
	methodDesc, err := c.methodDescriptor(method)
	if err != nil {
		return nil, err
	}
	requestPayload, err := c.prepareRequest(methodDesc, request)
	if err != nil {
		return nil, err
	}

	fullMethodPath := buildFullMethodPath(methodDesc)
	callCtx, cancel := c.withClientContext(ctx)
	defer cancel()
	return c.invokeRawOnce(callCtx, fullMethodPath, methodDesc, requestPayload)
}

// Resolver returns the underlying type resolver, useful for custom protojson operations.
func (c *Client) Resolver() *Resolver {
	return c.resolver
}

// Conn returns the underlying gRPC connection.
func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}

// SkippedFiles returns reflected proto files that could not be registered
// during Dial. These are typically files with unresolvable dependencies or
// custom options; methods that depend on them will not be callable.
func (c *Client) SkippedFiles() []string {
	return append([]string(nil), c.skipped...)
}

// ProtoDir returns the configured local proto directory, or nil if none.
func (c *Client) ProtoDir() *ProtoDir {
	if c.resolver == nil {
		return nil
	}
	return c.resolver.ProtoDir()
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.cancel()
	clearConnVersion(c.conn)
	return c.conn.Close()
}

func (c *Client) invokeOnce(ctx context.Context, fullMethodPath string, methodDesc protoreflect.MethodDescriptor, request any) ([]byte, error) {
	outputMsg, err := c.invokeRawOnce(ctx, fullMethodPath, methodDesc, request)
	if err != nil {
		return nil, err
	}

	return marshalDynamicJSON(c.resolver, outputMsg)
}

func (c *Client) invokeRawOnce(ctx context.Context, fullMethodPath string, methodDesc protoreflect.MethodDescriptor, request any) (*dynamicpb.Message, error) {
	if methodDesc.IsStreamingClient() || methodDesc.IsStreamingServer() {
		return nil, fmt.Errorf("method %s is a streaming RPC; libyaci currently supports unary methods only", methodDesc.FullName())
	}

	inputMsg := dynamicpb.NewMessage(methodDesc.Input())
	outputMsg := dynamicpb.NewMessage(methodDesc.Output())

	switch req := request.(type) {
	case nil:
	case []byte:
		if len(req) == 0 {
			break
		}
		uo := protojson.UnmarshalOptions{Resolver: c.resolver}
		if err := uo.Unmarshal(req, inputMsg); err != nil {
			return nil, fmt.Errorf("failed to parse request: %w", err)
		}
	case *Request:
		if req == nil {
			break
		}
		if req.msg.Descriptor().FullName() != methodDesc.Input().FullName() {
			return nil, fmt.Errorf("request type %s does not match method input %s", req.msg.Descriptor().FullName(), methodDesc.Input().FullName())
		}
		inputMsg = req.msg
	case *dynamicpb.Message:
		if req == nil {
			break
		}
		if req.Descriptor().FullName() != methodDesc.Input().FullName() {
			return nil, fmt.Errorf("request type %s does not match method input %s", req.Descriptor().FullName(), methodDesc.Input().FullName())
		}
		inputMsg = req
	default:
		return nil, fmt.Errorf("unsupported request type %T", request)
	}

	if err := c.conn.Invoke(ctx, fullMethodPath, inputMsg, outputMsg); err != nil {
		return nil, err
	}

	return outputMsg, nil
}

func (c *Client) methodDescriptor(method string) (protoreflect.MethodDescriptor, error) {
	serviceName, methodName, err := parseMethodFullName(method)
	if err != nil {
		if c.catalog != nil {
			return nil, c.catalog.unsupportedMethod(method)
		}
		return nil, err
	}

	if c.catalog != nil && !c.catalog.HasMethod(method) {
		return nil, c.catalog.unsupportedMethod(method)
	}

	methodDesc, err := c.resolver.FindMethodDescriptor(serviceName, methodName)
	if err != nil {
		return nil, fmt.Errorf("method descriptor lookup failed: %w", err)
	}
	return methodDesc, nil
}

func (c *Client) withClientContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = c.ctx
	}
	callCtx, cancel := context.WithCancel(ctx)
	var stop func() bool
	if ctx != c.ctx {
		stop = context.AfterFunc(c.ctx, cancel)
	}
	if c.opts != nil && c.opts.defaultTimeout > 0 {
		var timeoutCancel context.CancelFunc
		callCtx, timeoutCancel = context.WithTimeout(callCtx, c.opts.defaultTimeout)
		baseCancel := cancel
		cancel = func() {
			timeoutCancel()
			baseCancel()
		}
	}
	return callCtx, func() {
		if stop != nil {
			stop()
		}
		cancel()
	}
}

func (c *Client) prepareRequest(methodDesc protoreflect.MethodDescriptor, request []byte) (any, error) {
	if len(request) == 0 {
		return nil, nil
	}
	inputMsg := dynamicpb.NewMessage(methodDesc.Input())
	uo := protojson.UnmarshalOptions{Resolver: c.resolver}
	if err := uo.Unmarshal(request, inputMsg); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}
	return inputMsg, nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// isRetryableError reports whether a gRPC call error is worth retrying. Only
// transient transport/server conditions are retried; application-level errors
// such as InvalidArgument, NotFound, or PermissionDenied are returned to the
// caller immediately.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted:
		return true
	default:
		return false
	}
}

func dial(_ context.Context, address string, o *options) (*grpc.ClientConn, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                60 * time.Second,
			Timeout:             30 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(o.maxRecvMsgSize)),
	}

	// Handle ALPN enforcement before creating credentials
	if o.disableALPNEnforcement {
		DisableALPNEnforcement()
	}

	if o.insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		creds := credentials.NewClientTLSFromCert(nil, "")
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	}

	dialOpts = append(dialOpts, o.dialOpts...)

	return grpc.NewClient(address, dialOpts...)
}

func parseMethodFullName(methodFullName string) (string, string, error) {
	if methodFullName == "" {
		return "", "", fmt.Errorf("method name is empty")
	}

	lastDot := strings.LastIndex(methodFullName, ".")
	if lastDot == -1 {
		return "", "", fmt.Errorf("invalid method name format: %s", methodFullName)
	}

	serviceName := methodFullName[:lastDot]
	methodName := methodFullName[lastDot+1:]

	if serviceName == "" || methodName == "" {
		return "", "", fmt.Errorf("invalid method name format: %s", methodFullName)
	}

	return serviceName, methodName, nil
}

func buildFullMethodPath(methodDesc protoreflect.MethodDescriptor) string {
	fullName := "/" + string(methodDesc.FullName())
	lastDot := strings.LastIndex(fullName, ".")
	if lastDot != -1 {
		fullName = fullName[:lastDot] + "/" + fullName[lastDot+1:]
	}
	return fullName
}

// symbolFetch tracks in-progress fetches for a symbol
type symbolFetch struct {
	done chan struct{}
	err  error
}

// ResolverStats contains cache hit/miss statistics for the resolver.
type ResolverStats struct {
	CacheHits      uint64 // Types found in cache (no gRPC call)
	CacheMisses    uint64 // Types that required gRPC fetch
	FallbackHits   uint64 // Types found in fallback registry
	FallbackMisses uint64 // Types not found anywhere
}

// Resolver provides thread-safe resolution of protobuf types using server reflection.
// It implements protoregistry.MessageTypeResolver and protoregistry.ExtensionTypeResolver.
type Resolver struct {
	files      *protoregistry.Files
	fallback   *FallbackRegistry // fallback for deprecated types not in server reflection
	conn       *grpc.ClientConn
	ctx        context.Context
	inProgress map[string]*symbolFetch // tracks in-progress fetches
	maxRetries uint
	mu         sync.RWMutex

	// Cache statistics (atomic for lock-free reads)
	cacheHits      uint64
	cacheMisses    uint64
	fallbackHits   uint64
	fallbackMisses uint64
}

func newResolver(ctx context.Context, files *protoregistry.Files, conn *grpc.ClientConn, maxRetries uint, fallback *FallbackRegistry) *Resolver {
	return &Resolver{
		files:      files,
		fallback:   fallback,
		conn:       conn,
		ctx:        ctx,
		inProgress: make(map[string]*symbolFetch),
		maxRetries: maxRetries,
	}
}

// FindMethodDescriptor finds a method descriptor by service and method name.
func (r *Resolver) FindMethodDescriptor(serviceName, methodName string) (protoreflect.MethodDescriptor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var methodDesc protoreflect.MethodDescriptor
	var found bool

	r.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			if string(svc.FullName()) == serviceName {
				methods := svc.Methods()
				for j := 0; j < methods.Len(); j++ {
					m := methods.Get(j)
					if string(m.Name()) == methodName {
						methodDesc = m
						found = true
						return false
					}
				}
			}
		}
		return true
	})

	if !found {
		return nil, fmt.Errorf("method %s not found in service %s", methodName, serviceName)
	}
	return methodDesc, nil
}

// FindMessageByName finds a message type by its full name.
// This method is called automatically when unmarshaling protobuf Any types.
// It first checks the primary registry, then attempts server reflection,
// and finally falls back to the fallback registry for deprecated types.
func (r *Resolver) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	// First, check if already registered
	r.mu.Lock()
	desc, err := r.files.FindDescriptorByName(name)
	if err == nil && desc != nil {
		r.mu.Unlock()
		atomic.AddUint64(&r.cacheHits, 1) // Cache hit - no gRPC needed
		return createMessageType(desc, name)
	}

	// Check if fetch is in progress
	if fetch, ok := r.inProgress[string(name)]; ok {
		r.mu.Unlock()
		// Wait for the in-progress fetch to complete
		<-fetch.done
		if fetch.err != nil {
			// Try fallback before returning error
			return r.tryFallback(name, fetch.err)
		}
		// Re-check after fetch completes (counted as cache hit since we didn't initiate fetch)
		r.mu.Lock()
		desc, err = r.files.FindDescriptorByName(name)
		r.mu.Unlock()
		if err != nil || desc == nil {
			return r.tryFallback(name, fmt.Errorf("message %s not found after fetch: %w", name, err))
		}
		atomic.AddUint64(&r.cacheHits, 1) // Found after another goroutine fetched it
		return createMessageType(desc, name)
	}

	// Start a new fetch - this is a cache miss
	atomic.AddUint64(&r.cacheMisses, 1)
	fetch := &symbolFetch{done: make(chan struct{})}
	r.inProgress[string(name)] = fetch
	r.mu.Unlock()

	// Perform the fetch
	fetchErr := r.doFetch(string(name))

	// Mark fetch as complete
	r.mu.Lock()
	fetch.err = fetchErr
	close(fetch.done)
	delete(r.inProgress, string(name))

	if fetchErr != nil {
		r.mu.Unlock()
		// Try fallback before returning error
		return r.tryFallback(name, fmt.Errorf("failed to fetch descriptor for %s: %w", name, fetchErr))
	}

	desc, err = r.files.FindDescriptorByName(name)
	r.mu.Unlock()

	if err != nil || desc == nil {
		return r.tryFallback(name, fmt.Errorf("message %s not found after fetch: %w", name, err))
	}

	return createMessageType(desc, name)
}

// tryFallback attempts to find the message in the fallback registry.
// Returns the message type if found, otherwise returns an error with helpful hints.
func (r *Resolver) tryFallback(name protoreflect.FullName, originalErr error) (protoreflect.MessageType, error) {
	if r.fallback == nil {
		atomic.AddUint64(&r.fallbackMisses, 1)
		return nil, &TypeNotFoundError{
			TypeName:    string(name),
			OriginalErr: originalErr,
			Hint:        "no fallback registry configured; use WithProtoDir() to provide local proto definitions",
		}
	}

	desc, err := r.fallback.FindDescriptorByNameContext(r.ctx, name)
	if err != nil || desc == nil {
		atomic.AddUint64(&r.fallbackMisses, 1)
		hint := suggestProtoFile(string(name))
		if r.fallback.ProtoDir() == nil {
			hint = "no local proto directory configured; use WithProtoDir() or register the type with WithFallbackRegistry()"
		}
		return nil, &TypeNotFoundError{
			TypeName:    string(name),
			OriginalErr: originalErr,
			Hint:        hint,
		}
	}

	atomic.AddUint64(&r.fallbackHits, 1)
	return createMessageType(desc, name)
}

// TypeNotFoundError provides detailed information when a type cannot be resolved
// via server reflection or fallback registries.
type TypeNotFoundError struct {
	TypeName    string // full proto type name that was not found
	OriginalErr error  // underlying error from the resolution attempt
	Hint        string // suggestion for how to fix the issue
}

// Error implements the error interface with a helpful message.
func (e *TypeNotFoundError) Error() string {
	msg := fmt.Sprintf("type %s not found", e.TypeName)
	if e.Hint != "" {
		msg += ": " + e.Hint
	}
	return msg
}

// Unwrap returns the underlying error for use with errors.Is/errors.As.
func (e *TypeNotFoundError) Unwrap() error {
	return e.OriginalErr
}

// Stats returns cache hit/miss statistics for the resolver.
// This is useful for diagnosing performance issues with type resolution.
func (r *Resolver) Stats() ResolverStats {
	return ResolverStats{
		CacheHits:      atomic.LoadUint64(&r.cacheHits),
		CacheMisses:    atomic.LoadUint64(&r.cacheMisses),
		FallbackHits:   atomic.LoadUint64(&r.fallbackHits),
		FallbackMisses: atomic.LoadUint64(&r.fallbackMisses),
	}
}

// ResetStats resets all cache statistics to zero.
func (r *Resolver) ResetStats() {
	atomic.StoreUint64(&r.cacheHits, 0)
	atomic.StoreUint64(&r.cacheMisses, 0)
	atomic.StoreUint64(&r.fallbackHits, 0)
	atomic.StoreUint64(&r.fallbackMisses, 0)
}

// doFetch performs the actual fetch without holding the lock
func (r *Resolver) doFetch(symbol string) error {
	fdProtos, err := fetchFileDescriptorsBySymbol(r.ctx, r.conn, symbol, r.maxRetries)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.processFileDescriptorsLocked(fdProtos)
}

// FindMessageByURL finds a message type by its type URL (used in protobuf Any types).
func (r *Resolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	// Type URLs are formatted as "type.googleapis.com/package.MessageName"
	// or just "/package.MessageName"
	name := url
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		name = url[idx+1:]
	}
	return r.FindMessageByName(protoreflect.FullName(name))
}

// FindExtensionByName resolves a protobuf extension field by its full name.
// It searches the reflected registry first, then the fallback registry.
func (r *Resolver) FindExtensionByName(name protoreflect.FullName) (protoreflect.ExtensionType, error) {
	var found protoreflect.ExtensionDescriptor
	r.forEachExtension(func(xd protoreflect.ExtensionDescriptor) bool {
		if xd.FullName() == name {
			found = xd
			return true
		}
		return false
	})
	if found == nil {
		return nil, protoregistry.NotFound
	}
	return dynamicpb.NewExtensionType(found), nil
}

// FindExtensionByNumber resolves an extension field by its containing message
// and field number. It searches the reflected registry first, then the fallback
// registry.
func (r *Resolver) FindExtensionByNumber(containingType protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	var found protoreflect.ExtensionDescriptor
	r.forEachExtension(func(xd protoreflect.ExtensionDescriptor) bool {
		if xd.ContainingMessage().FullName() == containingType && xd.Number() == field {
			found = xd
			return true
		}
		return false
	})
	if found == nil {
		return nil, protoregistry.NotFound
	}
	return dynamicpb.NewExtensionType(found), nil
}

// forEachExtension visits every extension field declared in the primary
// registry, then the fallback registry, stopping early when fn returns true.
// protoregistry.Files.FindDescriptorByName does not resolve extension fields,
// so extensions must be walked explicitly.
// ponytail: linear scan over all files; fine for typical descriptor sets,
// index by name/number if extension-heavy schemas become a bottleneck.
func (r *Resolver) forEachExtension(fn func(protoreflect.ExtensionDescriptor) bool) {
	scan := func(files *protoregistry.Files) bool {
		done := false
		files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
			if scanExtensionList(fd.Extensions(), fn) {
				done = true
				return false
			}
			var walk func(msgs protoreflect.MessageDescriptors) bool
			walk = func(msgs protoreflect.MessageDescriptors) bool {
				for i := 0; i < msgs.Len(); i++ {
					m := msgs.Get(i)
					if scanExtensionList(m.Extensions(), fn) {
						return true
					}
					if walk(m.Messages()) {
						return true
					}
				}
				return false
			}
			if walk(fd.Messages()) {
				done = true
				return false
			}
			return true
		})
		return done
	}

	r.mu.RLock()
	done := scan(r.files)
	r.mu.RUnlock()
	if !done && r.fallback != nil {
		scan(r.fallback.Files())
	}
}

func scanExtensionList(exts protoreflect.ExtensionDescriptors, fn func(protoreflect.ExtensionDescriptor) bool) bool {
	for i := 0; i < exts.Len(); i++ {
		if fn(exts.Get(i)) {
			return true
		}
	}
	return false
}

// Files returns the underlying file registry.
func (r *Resolver) Files() *protoregistry.Files {
	return r.files
}

// ProtoDir returns the configured local proto directory, or nil if none.
func (r *Resolver) ProtoDir() *ProtoDir {
	if r.fallback == nil {
		return nil
	}
	return r.fallback.ProtoDir()
}

// processFileDescriptorsLocked processes file descriptors while holding the lock
func (r *Resolver) processFileDescriptorsLocked(fdProtos []*descriptorpb.FileDescriptorProto) error {
	for _, fdProto := range fdProtos {
		applyDescriptorPatches(fdProto)

		name := fdProto.GetName()
		if _, err := r.files.FindFileByPath(name); err == nil {
			continue // Already registered
		}

		// Fetch dependencies first (recursive, needs to release lock)
		for _, dep := range fdProto.Dependency {
			if _, err := r.files.FindFileByPath(dep); err == nil {
				continue
			}
			r.mu.Unlock()
			if err := r.fetchDescriptorByName(dep); err != nil {
				r.mu.Lock()
				return fmt.Errorf("failed to fetch dependency %s: %w", dep, err)
			}
			r.mu.Lock()
		}

		fd, err := protodesc.NewFile(fdProto, r.files)
		if err != nil {
			return fmt.Errorf("failed to create file descriptor for %s: %w", name, err)
		}

		if err := r.files.RegisterFile(fd); err != nil {
			return fmt.Errorf("failed to register file %s: %w", name, err)
		}
	}

	return nil
}

func (r *Resolver) fetchDescriptorByName(name string) error {
	fdProtos, err := fetchFileDescriptorsByName(r.ctx, r.conn, name, r.maxRetries)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.processFileDescriptorsLocked(fdProtos)
}

func createMessageType(desc protoreflect.Descriptor, name protoreflect.FullName) (protoreflect.MessageType, error) {
	if desc == nil {
		return nil, fmt.Errorf("message %s not found", name)
	}

	msgDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, fmt.Errorf("descriptor %s is not a message", name)
	}

	return dynamicpb.NewMessageType(msgDesc), nil
}

// utf8ErrorRegex matches protojson UTF-8 errors like:
// "field package.Message.field_name contains invalid UTF-8"
var utf8ErrorRegex = regexp.MustCompile(`field\s+(\S+)\.(\w+)\s+contains\s+invalid\s+UTF-8`)

// parseUTF8Error parses a protojson error message to extract the message type and field name
// when the error is about invalid UTF-8 in a string field.
// Returns (messageType, fieldName, true) if successfully parsed, otherwise ("", "", false).
func parseUTF8Error(errStr string) (string, string, bool) {
	if !strings.Contains(errStr, "invalid UTF-8") {
		return "", "", false
	}

	matches := utf8ErrorRegex.FindStringSubmatch(errStr)
	if len(matches) < 3 {
		return "", "", false
	}

	// matches[1] is the full message type path (e.g., "interchain_security.ccv.provider.v1.MsgAssignConsumerKey")
	// matches[2] is the field name (e.g., "consumer_key")
	return matches[1], matches[2], true
}

// CreatePatchedResolver creates a new temporary Resolver with a specific field
// patched from TYPE_STRING to TYPE_BYTES. This is used to recover from UTF-8
// validation errors when string fields contain binary data.
// The original resolver is NOT modified - the returned resolver is for one-time use.
func (r *Resolver) CreatePatchedResolver(messageType, fieldName string) (*Resolver, error) {
	name := protoreflect.FullName(messageType)

	// Locate the message descriptor. Prefer the primary (reflected) registry,
	// then the fallback registry that carries deprecated/local types.
	r.mu.RLock()
	primaryDesc, primaryErr := r.files.FindDescriptorByName(name)
	r.mu.RUnlock()

	var msgDesc protoreflect.MessageDescriptor
	if primaryErr == nil {
		md, ok := primaryDesc.(protoreflect.MessageDescriptor)
		if !ok {
			return nil, fmt.Errorf("%s is not a message type", messageType)
		}
		msgDesc = md
	} else if r.fallback != nil {
		fbDesc, fbErr := r.fallback.FindDescriptorByName(name)
		if fbErr != nil {
			return nil, fmt.Errorf("message type %s not found: %w", messageType, fbErr)
		}
		md, ok := fbDesc.(protoreflect.MessageDescriptor)
		if !ok {
			return nil, fmt.Errorf("%s is not a message type", messageType)
		}
		msgDesc = md
	} else {
		return nil, fmt.Errorf("message type %s not found: %w", messageType, primaryErr)
	}

	fileDesc := msgDesc.ParentFile()
	if fileDesc == nil {
		return nil, fmt.Errorf("no parent file for message %s", messageType)
	}
	filePath := fileDesc.Path()

	// Reconstruct the raw file descriptor in-memory (no server round trip) and
	// patch the offending field.
	fdProto := protodesc.ToFileDescriptorProto(fileDesc)
	if !patchFieldInProto(fdProto, messageType, fieldName) {
		return nil, fmt.Errorf("field %s.%s not found in descriptors", messageType, fieldName)
	}

	// Build a fresh registry from every file we know about (primary and
	// fallback), then register the patched file. Registration is best-effort:
	// files with unresolved dependencies are skipped rather than aborting.
	newFiles := &protoregistry.Files{}
	var pending []protoreflect.FileDescriptor
	r.mu.RLock()
	r.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if fd.Path() != filePath {
			pending = append(pending, fd)
		}
		return true
	})
	r.mu.RUnlock()
	if r.fallback != nil {
		r.fallback.Files().RangeFiles(func(fd protoreflect.FileDescriptor) bool {
			if fd.Path() != filePath {
				pending = append(pending, fd)
			}
			return true
		})
		// Local proto files are compiled into the ProtoDir registry rather than
		// the fallback registry's own files, so include them too or patched
		// multi-file packages cannot resolve their imports.
		if pd := r.fallback.ProtoDir(); pd != nil {
			if err := pd.Load(r.ctx); err != nil {
				return nil, fmt.Errorf("failed to load proto directory: %w", err)
			}
			if reg := pd.Registry(); reg != nil {
				reg.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
					if fd.Path() != filePath {
						pending = append(pending, fd)
					}
					return true
				})
			}
		}
	}
	registerFilesBestEffort(newFiles, pending)

	patchedFD, err := protodesc.NewFile(fdProto, newFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to create patched file descriptor for %s: %w", filePath, err)
	}
	if err := newFiles.RegisterFile(patchedFD); err != nil {
		return nil, fmt.Errorf("failed to register patched file %s: %w", filePath, err)
	}

	// Return a new resolver with the patched files (original unchanged)
	return &Resolver{
		files:      newFiles,
		fallback:   r.fallback,
		conn:       r.conn,
		ctx:        r.ctx,
		inProgress: make(map[string]*symbolFetch),
		maxRetries: r.maxRetries,
	}, nil
}

// registerFilesBestEffort registers each descriptor, skipping ones that are
// already present. RegisterFile does not resolve imports, so a single pass is
// sufficient; unresolved imports are reported later by protodesc.NewFile.
func registerFilesBestEffort(dst *protoregistry.Files, fds []protoreflect.FileDescriptor) {
	for _, fd := range fds {
		if _, err := dst.FindFileByPath(fd.Path()); err == nil {
			continue
		}
		_ = dst.RegisterFile(fd)
	}
}

// patchFieldInProto finds and patches a field from STRING to BYTES in a file descriptor proto.
// Handles nested messages by splitting the message type path.
func patchFieldInProto(fdProto *descriptorpb.FileDescriptorProto, messageType, fieldName string) bool {
	// Get the simple message name (last part after the package)
	pkg := fdProto.GetPackage()
	msgName := messageType
	if pkg != "" && strings.HasPrefix(messageType, pkg+".") {
		msgName = messageType[len(pkg)+1:]
	}

	// Handle nested messages (e.g., "OuterMessage.InnerMessage")
	parts := strings.Split(msgName, ".")

	// Find the message in the file
	for _, msgType := range fdProto.GetMessageType() {
		if patchMessageField(msgType, parts, fieldName) {
			return true
		}
	}

	return false
}

// patchMessageField recursively searches for and patches a field in a message descriptor.
func patchMessageField(msgType *descriptorpb.DescriptorProto, pathParts []string, fieldName string) bool {
	if len(pathParts) == 0 {
		return false
	}

	if msgType.GetName() != pathParts[0] {
		return false
	}

	// If this is the target message, find and patch the field
	if len(pathParts) == 1 {
		for _, field := range msgType.GetField() {
			if field.GetName() == fieldName && field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_STRING {
				field.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()
				return true
			}
		}
		return false
	}

	// Otherwise, search in nested types
	for _, nested := range msgType.GetNestedType() {
		if patchMessageField(nested, pathParts[1:], fieldName) {
			return true
		}
	}

	return false
}
