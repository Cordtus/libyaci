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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
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
}

// Dial creates a new reflection-based gRPC client connected to the specified address.
// The client will automatically fetch proto descriptors from the server using reflection.
func Dial(ctx context.Context, address string, opts ...Option) (*Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	// Create the client context (for ongoing operations)
	clientCtx, cancel := context.WithCancel(ctx)

	// Create a separate context for dial/init operations with optional timeout
	var initCtx context.Context
	var initCancel context.CancelFunc
	if o.dialTimeout > 0 {
		initCtx, initCancel = context.WithTimeout(clientCtx, o.dialTimeout)
	} else {
		initCtx, initCancel = context.WithCancel(clientCtx)
	}
	defer initCancel()

	conn, err := dial(initCtx, address, o)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Fetch all descriptors from the server
	descriptors, err := fetchAllDescriptors(initCtx, conn, o.maxRetries)
	if err != nil {
		conn.Close()
		cancel()
		return nil, fmt.Errorf("failed to fetch descriptors: %w", err)
	}

	// Build the file descriptor registry
	files, err := buildFileDescriptorSet(descriptors)
	if err != nil {
		conn.Close()
		cancel()
		return nil, fmt.Errorf("failed to build descriptor set: %w", err)
	}

	// Setup fallback registry
	var fallback *FallbackRegistry
	if o.fallback != nil {
		fallback = o.fallback
	} else if o.useGlobalFallback {
		fallback = GlobalFallback()
	}

	// Register deprecated modules if requested
	if o.registerDeprecated && fallback != nil {
		if err := fallback.RegisterDeprecatedCosmosModules(); err != nil {
			// Log warning but don't fail - deprecated modules are optional
			// The error is silently ignored as this is a best-effort feature
		}
	}

	// Setup local proto directory if configured
	if o.protoDir != "" && fallback != nil {
		protoDir := NewProtoDir(o.protoDir)
		fallback.SetProtoDir(protoDir)
	}

	resolver := newResolver(clientCtx, files, conn, o.maxRetries, fallback)

	return &Client{
		conn:     conn,
		ctx:      clientCtx,
		cancel:   cancel,
		resolver: resolver,
		opts:     o,
	}, nil
}

// Invoke calls the specified gRPC method with the given JSON request payload.
// The method should be specified as "package.Service.Method" (e.g., "cosmos.bank.v1beta1.Query.Balance").
// Returns the response as JSON bytes.
func (c *Client) Invoke(method string, request []byte) ([]byte, error) {
	return c.InvokeWithRetry(method, request, c.opts.maxRetries)
}

// InvokeWithRetry calls the specified gRPC method with custom retry count.
func (c *Client) InvokeWithRetry(method string, request []byte, maxRetries uint) ([]byte, error) {
	serviceName, methodName, err := parseMethodFullName(method)
	if err != nil {
		return nil, err
	}

	methodDesc, err := c.resolver.FindMethodDescriptor(serviceName, methodName)
	if err != nil {
		return nil, fmt.Errorf("method not found: %w", err)
	}

	fullMethodPath := buildFullMethodPath(methodDesc)

	var resp []byte
	var lastErr error

	for attempt := uint(1); attempt <= maxRetries; attempt++ {
		resp, lastErr = c.invokeOnce(fullMethodPath, methodDesc, request)
		if lastErr == nil {
			return resp, nil
		}
		if attempt < maxRetries {
			time.Sleep(time.Duration(2*attempt) * time.Second)
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// InvokeRaw calls the method and returns the dynamic protobuf message directly.
// This is useful when you need to access specific fields without JSON marshaling.
func (c *Client) InvokeRaw(method string, request []byte) (*dynamicpb.Message, error) {
	serviceName, methodName, err := parseMethodFullName(method)
	if err != nil {
		return nil, err
	}

	methodDesc, err := c.resolver.FindMethodDescriptor(serviceName, methodName)
	if err != nil {
		return nil, fmt.Errorf("method not found: %w", err)
	}

	fullMethodPath := buildFullMethodPath(methodDesc)
	return c.invokeRawOnce(fullMethodPath, methodDesc, request)
}

// Resolver returns the underlying type resolver, useful for custom protojson operations.
func (c *Client) Resolver() *Resolver {
	return c.resolver
}

// Conn returns the underlying gRPC connection.
func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.cancel()
	return c.conn.Close()
}

func (c *Client) invokeOnce(fullMethodPath string, methodDesc protoreflect.MethodDescriptor, request []byte) ([]byte, error) {
	outputMsg, err := c.invokeRawOnce(fullMethodPath, methodDesc, request)
	if err != nil {
		return nil, err
	}

	mo := protojson.MarshalOptions{Resolver: c.resolver}
	result, err := mo.Marshal(outputMsg)
	if err != nil {
		// Check if this is a UTF-8 error that we can recover from
		if msgType, fieldName, ok := parseUTF8Error(err.Error()); ok {
			// Create a temporary patched resolver (does not modify the original)
			if patchedResolver, patchErr := c.resolver.CreatePatchedResolver(msgType, fieldName); patchErr == nil {
				// Retry marshal with the temporary patched resolver
				patchedMo := protojson.MarshalOptions{Resolver: patchedResolver}
				return patchedMo.Marshal(outputMsg)
			}
		}
		return nil, err
	}
	return result, nil
}

func (c *Client) invokeRawOnce(fullMethodPath string, methodDesc protoreflect.MethodDescriptor, request []byte) (*dynamicpb.Message, error) {
	inputMsg := dynamicpb.NewMessage(methodDesc.Input())
	outputMsg := dynamicpb.NewMessage(methodDesc.Output())

	if len(request) > 0 {
		uo := protojson.UnmarshalOptions{Resolver: c.resolver}
		if err := uo.Unmarshal(request, inputMsg); err != nil {
			return nil, fmt.Errorf("failed to parse request: %w", err)
		}
	}

	if err := c.conn.Invoke(c.ctx, fullMethodPath, inputMsg, outputMsg); err != nil {
		return nil, err
	}

	return outputMsg, nil
}

func dial(ctx context.Context, address string, o *options) (*grpc.ClientConn, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                60 * time.Second,
			Timeout:             30 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(o.maxRecvMsgSize)),
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
	mu         sync.Mutex

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

	desc, err := r.fallback.FindDescriptorByName(name)
	if err != nil || desc == nil {
		atomic.AddUint64(&r.fallbackMisses, 1)
		return nil, &TypeNotFoundError{
			TypeName:    string(name),
			OriginalErr: originalErr,
			Hint:        suggestProtoFile(string(name)),
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

// FindExtensionByName is not implemented (returns NotFound).
func (r *Resolver) FindExtensionByName(_ protoreflect.FullName) (protoreflect.ExtensionType, error) {
	return nil, protoregistry.NotFound
}

// FindExtensionByNumber is not implemented (returns NotFound).
func (r *Resolver) FindExtensionByNumber(_ protoreflect.FullName, _ protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	return nil, protoregistry.NotFound
}

// Files returns the underlying file registry.
func (r *Resolver) Files() *protoregistry.Files {
	return r.files
}

// processFileDescriptorsLocked processes file descriptors while holding the lock
func (r *Resolver) processFileDescriptorsLocked(fdProtos []*descriptorpb.FileDescriptorProto) error {
	for _, fdProto := range fdProtos {
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
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find the message descriptor
	desc, err := r.files.FindDescriptorByName(protoreflect.FullName(messageType))
	if err != nil {
		return nil, fmt.Errorf("message type %s not found: %w", messageType, err)
	}

	msgDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, fmt.Errorf("%s is not a message type", messageType)
	}

	// Get the file descriptor that contains this message
	fileDesc := msgDesc.ParentFile()
	if fileDesc == nil {
		return nil, fmt.Errorf("no parent file for message %s", messageType)
	}

	// Get the file path to fetch the raw proto
	filePath := fileDesc.Path()

	// Fetch the file descriptor proto again so we can modify it
	fdProtos, fetchErr := fetchFileDescriptorsByName(r.ctx, r.conn, filePath, r.maxRetries)
	if fetchErr != nil {
		return nil, fmt.Errorf("failed to fetch descriptor for %s: %w", filePath, fetchErr)
	}

	// Find and patch the field
	var patched bool
	for _, fdProto := range fdProtos {
		if fdProto.GetName() != filePath {
			continue
		}
		if patchFieldInProto(fdProto, messageType, fieldName) {
			patched = true
			break
		}
	}

	if !patched {
		return nil, fmt.Errorf("field %s.%s not found in descriptors", messageType, fieldName)
	}

	// Create a new Files registry for the patched resolver
	newFiles := &protoregistry.Files{}

	// Copy all existing files except the one we're patching
	var copyErr error
	r.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if fd.Path() != filePath {
			if err := newFiles.RegisterFile(fd); err != nil {
				copyErr = err
				return false
			}
		}
		return true
	})
	if copyErr != nil {
		return nil, fmt.Errorf("failed to copy file descriptors: %w", copyErr)
	}

	// Register the patched file
	for _, fdProto := range fdProtos {
		fd, err := protodesc.NewFile(fdProto, newFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to create patched file descriptor for %s: %w", filePath, err)
		}
		if err := newFiles.RegisterFile(fd); err != nil {
			return nil, fmt.Errorf("failed to register patched file %s: %w", filePath, err)
		}
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
