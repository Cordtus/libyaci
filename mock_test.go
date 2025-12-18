package libyaci

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	bufSize               = 1024 * 1024
	testProtoName         = "test.proto"
	testPackage           = "test"
	testServiceName       = "TestService"
	testMethodName        = "TestMethod"
	testInputName         = "TestInput"
	testOutputName        = "TestOutput"
	dependencyProtoName   = "dependency.proto"
	dependencyMessageName = "DependencyMessage"
)

var (
	mockFileDescriptor = &descriptorpb.FileDescriptorProto{
		Name:    proto.String(testProtoName),
		Package: proto.String(testPackage),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String(testServiceName),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String(testMethodName),
						InputType:  proto.String("." + testPackage + "." + testInputName),
						OutputType: proto.String("." + testPackage + "." + testOutputName),
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String(testInputName)},
			{Name: proto.String(testOutputName)},
		},
		Dependency: []string{dependencyProtoName},
	}

	mockDependencyFileDescriptor = &descriptorpb.FileDescriptorProto{
		Name:    proto.String(dependencyProtoName),
		Package: proto.String("dependency"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String(dependencyMessageName)},
		},
	}
)

type mockReflectionServer struct {
	reflectionpb.UnimplementedServerReflectionServer
}

func (s *mockReflectionServer) ServerReflectionInfo(stream reflectionpb.ServerReflection_ServerReflectionInfoServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}

		var resp *reflectionpb.ServerReflectionResponse
		switch req.MessageRequest.(type) {
		case *reflectionpb.ServerReflectionRequest_ListServices:
			resp = createListServicesResponse(testPackage + "." + testServiceName)
		case *reflectionpb.ServerReflectionRequest_FileByFilename:
			resp = createFileDescriptorResponse(mockDependencyFileDescriptor)
		case *reflectionpb.ServerReflectionRequest_FileContainingSymbol:
			resp = createFileDescriptorResponse(mockFileDescriptor)
		default:
			resp = createErrorResponse("unknown request type")
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func mustMarshal(pb proto.Message) []byte {
	data, err := proto.Marshal(pb)
	if err != nil {
		panic(err)
	}
	return data
}

func createListServicesResponse(serviceName string) *reflectionpb.ServerReflectionResponse {
	return &reflectionpb.ServerReflectionResponse{
		MessageResponse: &reflectionpb.ServerReflectionResponse_ListServicesResponse{
			ListServicesResponse: &reflectionpb.ListServiceResponse{
				Service: []*reflectionpb.ServiceResponse{
					{Name: serviceName},
				},
			},
		},
	}
}

func createFileDescriptorResponse(fd *descriptorpb.FileDescriptorProto) *reflectionpb.ServerReflectionResponse {
	return &reflectionpb.ServerReflectionResponse{
		MessageResponse: &reflectionpb.ServerReflectionResponse_FileDescriptorResponse{
			FileDescriptorResponse: &reflectionpb.FileDescriptorResponse{
				FileDescriptorProto: [][]byte{
					mustMarshal(fd),
				},
			},
		},
	}
}

func createErrorResponse(msg string) *reflectionpb.ServerReflectionResponse {
	return &reflectionpb.ServerReflectionResponse{
		MessageResponse: &reflectionpb.ServerReflectionResponse_ErrorResponse{
			ErrorResponse: &reflectionpb.ErrorResponse{
				ErrorCode:    1,
				ErrorMessage: msg,
			},
		},
	}
}

// mockServer provides a bufconn-based mock gRPC server for testing.
type mockServer struct {
	lis    *bufconn.Listener
	server *grpc.Server
}

func newMockServer() *mockServer {
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	reflectionpb.RegisterServerReflectionServer(s, &mockReflectionServer{})

	go func() {
		_ = s.Serve(lis)
	}()

	return &mockServer{
		lis:    lis,
		server: s,
	}
}

func (m *mockServer) dial(ctx context.Context) (*grpc.ClientConn, error) {
	_ = ctx // context not used with NewClient (connects lazily)
	return grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return m.lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

func (m *mockServer) stop() {
	m.server.Stop()
}
