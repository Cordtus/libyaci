package libyaci

//lint:file-ignore SA1019 v1alpha reflection API is deprecated but required for backwards compatibility with older gRPC servers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// reflectionVersion tracks which reflection API version is supported by a connection.
type reflectionVersion int

const (
	reflectionUnknown reflectionVersion = iota
	reflectionV1
	reflectionV1Alpha
)

func reflectionVersionName(v reflectionVersion) string {
	switch v {
	case reflectionV1:
		return "grpc.reflection.v1"
	case reflectionV1Alpha:
		return "grpc.reflection.v1alpha"
	default:
		return "unknown"
	}
}

// connReflectionVersion caches which reflection version each connection supports.
var (
	connVersions   = make(map[*grpc.ClientConn]reflectionVersion)
	connVersionsMu sync.RWMutex
)

// getConnVersion returns the cached reflection version for a connection.
func getConnVersion(conn *grpc.ClientConn) reflectionVersion {
	connVersionsMu.RLock()
	defer connVersionsMu.RUnlock()
	return connVersions[conn]
}

// setConnVersion caches which reflection version works for a connection.
func setConnVersion(conn *grpc.ClientConn, v reflectionVersion) {
	connVersionsMu.Lock()
	defer connVersionsMu.Unlock()
	connVersions[conn] = v
}

func clearConnVersion(conn *grpc.ClientConn) {
	connVersionsMu.Lock()
	defer connVersionsMu.Unlock()
	delete(connVersions, conn)
}

// fetchAllDescriptors retrieves all file descriptors from the server via reflection.
func fetchAllDescriptors(ctx context.Context, conn *grpc.ClientConn, maxRetries uint) ([]*descriptorpb.FileDescriptorProto, error) {
	descriptors, _, err := fetchDescriptorSnapshot(ctx, conn, maxRetries)
	return descriptors, err
}

func fetchDescriptorSnapshot(ctx context.Context, conn *grpc.ClientConn, maxRetries uint) ([]*descriptorpb.FileDescriptorProto, []string, error) {
	seenFiles := make(map[string]*descriptorpb.FileDescriptorProto)

	services, err := listServices(ctx, conn, maxRetries)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list services: %w", err)
	}

	for _, service := range services {
		if err := fetchFileDescriptorsForSymbol(ctx, conn, service, seenFiles, maxRetries); err != nil {
			return nil, nil, fmt.Errorf("failed to fetch descriptors for service %s: %w", service, err)
		}
	}

	result := make([]*descriptorpb.FileDescriptorProto, 0, len(seenFiles))
	for _, fd := range seenFiles {
		result = append(result, fd)
	}

	return result, services, nil
}

// listServicesRequest represents a request to list services (version-agnostic).
type listServicesRequest struct{}

// fileContainingSymbolRequest represents a request to get file by symbol (version-agnostic).
type fileContainingSymbolRequest struct {
	symbol string
}

// fileByFilenameRequest represents a request to get file by name (version-agnostic).
type fileByFilenameRequest struct {
	filename string
}

// reflectionRequest is a union type for version-agnostic requests.
type reflectionRequest interface{}

// reflectionResponse holds version-agnostic response data.
type reflectionResponse struct {
	services        []string
	fileDescriptors [][]byte
}

func listServices(ctx context.Context, conn *grpc.ClientConn, maxRetries uint) ([]string, error) {
	resp, err := sendReflectionRequestWithRetry(ctx, conn, listServicesRequest{}, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	return resp.services, nil
}

func fetchFileDescriptorsForSymbol(ctx context.Context, conn *grpc.ClientConn, symbol string, seen map[string]*descriptorpb.FileDescriptorProto, maxRetries uint) error {
	// Note: seen is keyed by file name, not symbol, so it cannot be used to
	// short-circuit a symbol lookup here. processDescriptors deduplicates files.
	fdProtos, err := fetchFileDescriptorsBySymbol(ctx, conn, symbol, maxRetries)
	if err != nil {
		return err
	}

	return processDescriptors(ctx, conn, fdProtos, seen, maxRetries)
}

func fetchFileDescriptorsBySymbol(ctx context.Context, conn *grpc.ClientConn, symbol string, maxRetries uint) ([]*descriptorpb.FileDescriptorProto, error) {
	return fetchFileDescriptorsFromRequest(ctx, conn, fileContainingSymbolRequest{symbol: symbol}, maxRetries)
}

func fetchFileDescriptorsByName(ctx context.Context, conn *grpc.ClientConn, name string, maxRetries uint) ([]*descriptorpb.FileDescriptorProto, error) {
	return fetchFileDescriptorsFromRequest(ctx, conn, fileByFilenameRequest{filename: name}, maxRetries)
}

func fetchFileDescriptorsFromRequest(ctx context.Context, conn *grpc.ClientConn, req reflectionRequest, maxRetries uint) ([]*descriptorpb.FileDescriptorProto, error) {
	resp, err := sendReflectionRequestWithRetry(ctx, conn, req, maxRetries)
	if err != nil {
		return nil, err
	}

	fdProtos := make([]*descriptorpb.FileDescriptorProto, 0, len(resp.fileDescriptors))
	for _, fdBytes := range resp.fileDescriptors {
		fdProto := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(fdBytes, fdProto); err != nil {
			return nil, fmt.Errorf("failed to unmarshal file descriptor: %w", err)
		}
		fdProtos = append(fdProtos, fdProto)
	}

	return fdProtos, nil
}

func processDescriptors(ctx context.Context, conn *grpc.ClientConn, fdProtos []*descriptorpb.FileDescriptorProto, seen map[string]*descriptorpb.FileDescriptorProto, maxRetries uint) error {
	for _, fdProto := range fdProtos {
		name := fdProto.GetName()
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = fdProto

		// Recursively fetch dependencies
		for _, dep := range fdProto.Dependency {
			if _, exists := seen[dep]; !exists {
				if err := fetchFileDescriptorByName(ctx, conn, dep, seen, maxRetries); err != nil {
					return fmt.Errorf("failed to fetch dependency %s: %w", dep, err)
				}
			}
		}
	}
	return nil
}

func fetchFileDescriptorByName(ctx context.Context, conn *grpc.ClientConn, name string, seen map[string]*descriptorpb.FileDescriptorProto, maxRetries uint) error {
	if _, exists := seen[name]; exists {
		return nil
	}

	fdProtos, err := fetchFileDescriptorsByName(ctx, conn, name, maxRetries)
	if err != nil {
		return err
	}

	return processDescriptors(ctx, conn, fdProtos, seen, maxRetries)
}

func sendReflectionRequestWithRetry(ctx context.Context, conn *grpc.ClientConn, req reflectionRequest, maxRetries uint) (*reflectionResponse, error) {
	var resp *reflectionResponse
	var err error
	maxAttempts := maxRetries
	if maxAttempts == 0 {
		maxAttempts = 1
	}

	attempts := uint(0)
	for attempt := uint(1); attempt <= maxAttempts; attempt++ {
		attempts = attempt
		resp, err = sendReflectionRequest(ctx, conn, req)
		if err == nil {
			return resp, nil
		}
		if attempt < maxAttempts && isRetryableError(err) {
			if sleepErr := sleepContext(ctx, time.Duration(2*attempt)*time.Second); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}
		break
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", attempts, err)
}

func sendReflectionRequest(ctx context.Context, conn *grpc.ClientConn, req reflectionRequest) (*reflectionResponse, error) {
	version := getConnVersion(conn)

	// If we already know the version, use it directly
	if version == reflectionV1 {
		return sendReflectionRequestV1(ctx, conn, req)
	}
	if version == reflectionV1Alpha {
		return sendReflectionRequestV1Alpha(ctx, conn, req)
	}

	// Try v1 first, then fall back to v1alpha
	resp, err := sendReflectionRequestV1(ctx, conn, req)
	if err == nil {
		setConnVersion(conn, reflectionV1)
		return resp, nil
	}

	// Check if it's an "Unimplemented" error (service not found)
	if isUnimplementedError(err) {
		resp, err = sendReflectionRequestV1Alpha(ctx, conn, req)
		if err == nil {
			setConnVersion(conn, reflectionV1Alpha)
			return resp, nil
		}
	}

	return nil, err
}

// isUnimplementedError checks if the error indicates the service is not implemented.
func isUnimplementedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Unimplemented") ||
		strings.Contains(errStr, "unknown service")
}

// sendReflectionRequestV1 sends a request using the v1 reflection API.
func sendReflectionRequestV1(ctx context.Context, conn *grpc.ClientConn, req reflectionRequest) (*reflectionResponse, error) {
	refClient := reflectionv1.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reflection stream: %w", err)
	}
	defer stream.CloseSend()

	v1Req := buildV1Request(req)
	if err := stream.Send(v1Req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	v1Resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	return parseV1Response(v1Resp)
}

// sendReflectionRequestV1Alpha sends a request using the v1alpha reflection API.
func sendReflectionRequestV1Alpha(ctx context.Context, conn *grpc.ClientConn, req reflectionRequest) (*reflectionResponse, error) {
	refClient := reflectionv1alpha.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reflection stream: %w", err)
	}
	defer stream.CloseSend()

	v1alphaReq := buildV1AlphaRequest(req)
	if err := stream.Send(v1alphaReq); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	v1alphaResp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	return parseV1AlphaResponse(v1alphaResp)
}

// buildV1Request converts a version-agnostic request to v1 format.
func buildV1Request(req reflectionRequest) *reflectionv1.ServerReflectionRequest {
	switch r := req.(type) {
	case listServicesRequest:
		return &reflectionv1.ServerReflectionRequest{
			MessageRequest: &reflectionv1.ServerReflectionRequest_ListServices{
				ListServices: "*",
			},
		}
	case fileContainingSymbolRequest:
		return &reflectionv1.ServerReflectionRequest{
			MessageRequest: &reflectionv1.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: r.symbol,
			},
		}
	case fileByFilenameRequest:
		return &reflectionv1.ServerReflectionRequest{
			MessageRequest: &reflectionv1.ServerReflectionRequest_FileByFilename{
				FileByFilename: r.filename,
			},
		}
	default:
		return nil
	}
}

// buildV1AlphaRequest converts a version-agnostic request to v1alpha format.
func buildV1AlphaRequest(req reflectionRequest) *reflectionv1alpha.ServerReflectionRequest {
	switch r := req.(type) {
	case listServicesRequest:
		return &reflectionv1alpha.ServerReflectionRequest{
			MessageRequest: &reflectionv1alpha.ServerReflectionRequest_ListServices{
				ListServices: "*",
			},
		}
	case fileContainingSymbolRequest:
		return &reflectionv1alpha.ServerReflectionRequest{
			MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: r.symbol,
			},
		}
	case fileByFilenameRequest:
		return &reflectionv1alpha.ServerReflectionRequest{
			MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileByFilename{
				FileByFilename: r.filename,
			},
		}
	default:
		return nil
	}
}

// parseV1Response extracts data from a v1 response into version-agnostic format.
func parseV1Response(resp *reflectionv1.ServerReflectionResponse) (*reflectionResponse, error) {
	// Check for error response
	if errResp, ok := resp.MessageResponse.(*reflectionv1.ServerReflectionResponse_ErrorResponse); ok {
		return nil, fmt.Errorf("reflection error: %s (code: %d)", errResp.ErrorResponse.ErrorMessage, errResp.ErrorResponse.ErrorCode)
	}

	result := &reflectionResponse{}

	switch r := resp.MessageResponse.(type) {
	case *reflectionv1.ServerReflectionResponse_ListServicesResponse:
		result.services = make([]string, 0, len(r.ListServicesResponse.Service))
		for _, svc := range r.ListServicesResponse.Service {
			result.services = append(result.services, svc.Name)
		}
	case *reflectionv1.ServerReflectionResponse_FileDescriptorResponse:
		result.fileDescriptors = r.FileDescriptorResponse.FileDescriptorProto
	}

	return result, nil
}

// parseV1AlphaResponse extracts data from a v1alpha response into version-agnostic format.
func parseV1AlphaResponse(resp *reflectionv1alpha.ServerReflectionResponse) (*reflectionResponse, error) {
	// Check for error response
	if errResp, ok := resp.MessageResponse.(*reflectionv1alpha.ServerReflectionResponse_ErrorResponse); ok {
		return nil, fmt.Errorf("reflection error: %s (code: %d)", errResp.ErrorResponse.ErrorMessage, errResp.ErrorResponse.ErrorCode)
	}

	result := &reflectionResponse{}

	switch r := resp.MessageResponse.(type) {
	case *reflectionv1alpha.ServerReflectionResponse_ListServicesResponse:
		result.services = make([]string, 0, len(r.ListServicesResponse.Service))
		for _, svc := range r.ListServicesResponse.Service {
			result.services = append(result.services, svc.Name)
		}
	case *reflectionv1alpha.ServerReflectionResponse_FileDescriptorResponse:
		result.fileDescriptors = r.FileDescriptorResponse.FileDescriptorProto
	}

	return result, nil
}

// buildFileDescriptorSet builds a protoregistry.Files from the given descriptors.
func buildFileDescriptorSet(descriptors []*descriptorpb.FileDescriptorProto) (*protoregistry.Files, error) {
	files, _, err := buildFileDescriptorSetReport(descriptors)
	return files, err
}

// buildFileDescriptorSetReport builds a protoregistry.Files from the given
// descriptors and also returns the names of files that could not be registered.
// A single malformed reflected file no longer aborts the whole build; it is
// skipped and reported so the rest of the client remains usable.
func buildFileDescriptorSetReport(descriptors []*descriptorpb.FileDescriptorProto) (*protoregistry.Files, []string, error) {
	files := &protoregistry.Files{}

	fdMap := make(map[string]*descriptorpb.FileDescriptorProto, len(descriptors))
	for _, fdProto := range descriptors {
		fdMap[fdProto.GetName()] = fdProto
	}

	sorted, err := topologicalSort(fdMap)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sort descriptors: %w", err)
	}

	var skipped []string
	for _, fdProto := range sorted {
		applyDescriptorPatches(fdProto)

		fd, err := protodesc.NewFile(fdProto, files)
		if err != nil {
			skipped = append(skipped, fdProto.GetName())
			continue
		}

		if err := files.RegisterFile(fd); err != nil {
			skipped = append(skipped, fdProto.GetName())
		}
	}

	return files, skipped, nil
}

// applyDescriptorPatches applies known patches for problematic proto definitions.
func applyDescriptorPatches(fdProto *descriptorpb.FileDescriptorProto) {
	// Fix for cosmos/base/abci/v1beta1/abci.proto
	// The `raw_log` field can contain invalid UTF-8, so we change its type from string to bytes.
	// See https://github.com/cosmos/cosmos-sdk/issues/22414
	if fdProto.GetName() == "cosmos/base/abci/v1beta1/abci.proto" {
		for _, msgType := range fdProto.GetMessageType() {
			for _, field := range msgType.GetField() {
				if field.GetName() == "raw_log" && field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_STRING {
					field.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()
				}
			}
		}
	}
}

func topologicalSort(fdMap map[string]*descriptorpb.FileDescriptorProto) ([]*descriptorpb.FileDescriptorProto, error) {
	visited := make(map[string]bool)
	tempMarked := make(map[string]bool)
	var sorted []*descriptorpb.FileDescriptorProto

	var visit func(string) error
	visit = func(name string) error {
		if tempMarked[name] {
			return fmt.Errorf("circular dependency at %s", name)
		}
		if visited[name] {
			return nil
		}

		tempMarked[name] = true
		fdProto := fdMap[name]
		if fdProto == nil {
			tempMarked[name] = false
			return fmt.Errorf("file descriptor not found: %s", name)
		}

		for _, dep := range fdProto.Dependency {
			if _, exists := fdMap[dep]; exists {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		visited[name] = true
		tempMarked[name] = false
		sorted = append(sorted, fdProto)
		return nil
	}

	for name := range fdMap {
		if !visited[name] {
			if err := visit(name); err != nil {
				return nil, err
			}
		}
	}

	return sorted, nil
}
