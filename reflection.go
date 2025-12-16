package libyaci

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	reflection "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// fetchAllDescriptors retrieves all file descriptors from the server via reflection.
func fetchAllDescriptors(ctx context.Context, conn *grpc.ClientConn, maxRetries uint) ([]*descriptorpb.FileDescriptorProto, error) {
	seenFiles := make(map[string]*descriptorpb.FileDescriptorProto)

	services, err := listServices(ctx, conn, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	for _, service := range services {
		if err := fetchFileDescriptorsForSymbol(ctx, conn, service, seenFiles, maxRetries); err != nil {
			return nil, fmt.Errorf("failed to fetch descriptors for service %s: %w", service, err)
		}
	}

	result := make([]*descriptorpb.FileDescriptorProto, 0, len(seenFiles))
	for _, fd := range seenFiles {
		result = append(result, fd)
	}

	return result, nil
}

func listServices(ctx context.Context, conn *grpc.ClientConn, maxRetries uint) ([]string, error) {
	req := &reflection.ServerReflectionRequest{
		MessageRequest: &reflection.ServerReflectionRequest_ListServices{
			ListServices: "*",
		},
	}

	resp, err := sendReflectionRequestWithRetry(ctx, conn, req, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	listResp, ok := resp.MessageResponse.(*reflection.ServerReflectionResponse_ListServicesResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp.MessageResponse)
	}

	services := make([]string, 0, len(listResp.ListServicesResponse.Service))
	for _, svc := range listResp.ListServicesResponse.Service {
		services = append(services, svc.Name)
	}

	return services, nil
}

func fetchFileDescriptorsForSymbol(ctx context.Context, conn *grpc.ClientConn, symbol string, seen map[string]*descriptorpb.FileDescriptorProto, maxRetries uint) error {
	if _, exists := seen[symbol]; exists {
		return nil
	}

	fdProtos, err := fetchFileDescriptorsBySymbol(ctx, conn, symbol, maxRetries)
	if err != nil {
		return err
	}

	return processDescriptors(ctx, conn, fdProtos, seen, maxRetries)
}

func fetchFileDescriptorsBySymbol(ctx context.Context, conn *grpc.ClientConn, symbol string, maxRetries uint) ([]*descriptorpb.FileDescriptorProto, error) {
	req := &reflection.ServerReflectionRequest{
		MessageRequest: &reflection.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: symbol,
		},
	}
	return fetchFileDescriptorsFromRequest(ctx, conn, req, maxRetries)
}

func fetchFileDescriptorsByName(ctx context.Context, conn *grpc.ClientConn, name string, maxRetries uint) ([]*descriptorpb.FileDescriptorProto, error) {
	req := &reflection.ServerReflectionRequest{
		MessageRequest: &reflection.ServerReflectionRequest_FileByFilename{
			FileByFilename: name,
		},
	}
	return fetchFileDescriptorsFromRequest(ctx, conn, req, maxRetries)
}

func fetchFileDescriptorsFromRequest(ctx context.Context, conn *grpc.ClientConn, req *reflection.ServerReflectionRequest, maxRetries uint) ([]*descriptorpb.FileDescriptorProto, error) {
	resp, err := sendReflectionRequestWithRetry(ctx, conn, req, maxRetries)
	if err != nil {
		return nil, err
	}

	fdResp, ok := resp.MessageResponse.(*reflection.ServerReflectionResponse_FileDescriptorResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp.MessageResponse)
	}

	fdProtos := make([]*descriptorpb.FileDescriptorProto, 0, len(fdResp.FileDescriptorResponse.FileDescriptorProto))
	for _, fdBytes := range fdResp.FileDescriptorResponse.FileDescriptorProto {
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

func sendReflectionRequestWithRetry(ctx context.Context, conn *grpc.ClientConn, req *reflection.ServerReflectionRequest, maxRetries uint) (*reflection.ServerReflectionResponse, error) {
	var resp *reflection.ServerReflectionResponse
	var err error

	for attempt := uint(1); attempt <= maxRetries; attempt++ {
		resp, err = sendReflectionRequest(ctx, conn, req)
		if err == nil {
			return resp, nil
		}
		if attempt < maxRetries {
			time.Sleep(time.Duration(2*attempt) * time.Second)
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, err)
}

func sendReflectionRequest(ctx context.Context, conn *grpc.ClientConn, req *reflection.ServerReflectionRequest) (*reflection.ServerReflectionResponse, error) {
	refClient := reflection.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reflection stream: %w", err)
	}
	defer stream.CloseSend()

	if err := stream.Send(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	if err := checkErrorResponse(resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// checkErrorResponse checks if the reflection response contains an error.
func checkErrorResponse(resp *reflection.ServerReflectionResponse) error {
	if errResp, ok := resp.MessageResponse.(*reflection.ServerReflectionResponse_ErrorResponse); ok {
		return fmt.Errorf("reflection error: %s (code: %d)", errResp.ErrorResponse.ErrorMessage, errResp.ErrorResponse.ErrorCode)
	}
	return nil
}

// buildFileDescriptorSet builds a protoregistry.Files from the given descriptors.
func buildFileDescriptorSet(descriptors []*descriptorpb.FileDescriptorProto) (*protoregistry.Files, error) {
	files := &protoregistry.Files{}

	fdMap := make(map[string]*descriptorpb.FileDescriptorProto, len(descriptors))
	for _, fdProto := range descriptors {
		fdMap[fdProto.GetName()] = fdProto
	}

	sorted, err := topologicalSort(fdMap)
	if err != nil {
		return nil, fmt.Errorf("failed to sort descriptors: %w", err)
	}

	for _, fdProto := range sorted {
		applyDescriptorPatches(fdProto)

		fd, err := protodesc.NewFile(fdProto, files)
		if err != nil {
			return nil, fmt.Errorf("failed to create file descriptor for %s: %w", fdProto.GetName(), err)
		}

		if err := files.RegisterFile(fd); err != nil {
			return nil, fmt.Errorf("failed to register %s: %w", fdProto.GetName(), err)
		}
	}

	return files, nil
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
