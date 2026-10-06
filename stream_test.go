package libyaci

import (
	"context"
	"io"
	"net"
	"testing"

	"google.golang.org/grpc"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	streamProtoName   = "stream.proto"
	streamServiceName = "stream.test.StreamService"
)

var streamFile = &descriptorpb.FileDescriptorProto{
	Name:    proto.String(streamProtoName),
	Package: proto.String("stream.test"),
	Syntax:  proto.String("proto3"),
	MessageType: []*descriptorpb.DescriptorProto{
		{Name: proto.String("CountRequest"), Field: []*descriptorpb.FieldDescriptorProto{streamInt32Field("count", 1)}},
		{Name: proto.String("CountResponse"), Field: []*descriptorpb.FieldDescriptorProto{streamInt32Field("value", 1)}},
		{Name: proto.String("SumRequest"), Field: []*descriptorpb.FieldDescriptorProto{streamInt32Field("value", 1)}},
		{Name: proto.String("SumResponse"), Field: []*descriptorpb.FieldDescriptorProto{streamInt32Field("total", 1)}},
		{Name: proto.String("ChatMessage"), Field: []*descriptorpb.FieldDescriptorProto{streamStringField("text", 1)}},
	},
	Service: []*descriptorpb.ServiceDescriptorProto{
		{
			Name: proto.String("StreamService"),
			Method: []*descriptorpb.MethodDescriptorProto{
				{
					Name:            proto.String("ServerCount"),
					InputType:       proto.String(".stream.test.CountRequest"),
					OutputType:      proto.String(".stream.test.CountResponse"),
					ServerStreaming: proto.Bool(true),
				},
				{
					Name:            proto.String("ClientSum"),
					InputType:       proto.String(".stream.test.SumRequest"),
					OutputType:      proto.String(".stream.test.SumResponse"),
					ClientStreaming: proto.Bool(true),
				},
				{
					Name:            proto.String("Chat"),
					InputType:       proto.String(".stream.test.ChatMessage"),
					OutputType:      proto.String(".stream.test.ChatMessage"),
					ClientStreaming: proto.Bool(true),
					ServerStreaming: proto.Bool(true),
				},
			},
		},
	},
}

var (
	streamCountReqDesc  protoreflect.MessageDescriptor
	streamCountRespDesc protoreflect.MessageDescriptor
	streamSumReqDesc    protoreflect.MessageDescriptor
	streamSumRespDesc   protoreflect.MessageDescriptor
	streamChatDesc      protoreflect.MessageDescriptor
)

var streamServiceDesc = grpc.ServiceDesc{
	ServiceName: streamServiceName,
	HandlerType: (*any)(nil),
	Streams: []grpc.StreamDesc{
		{StreamName: "ServerCount", Handler: handleServerCount, ServerStreams: true},
		{StreamName: "ClientSum", Handler: handleClientSum, ClientStreams: true},
		{StreamName: "Chat", Handler: handleChat, ClientStreams: true, ServerStreams: true},
	},
	Metadata: streamProtoName,
}

func handleServerCount(_ any, stream grpc.ServerStream) error {
	in := dynamicpb.NewMessage(streamCountReqDesc)
	if err := stream.RecvMsg(in); err != nil {
		return err
	}
	count := in.Get(in.Descriptor().Fields().ByName("count")).Int()
	for i := int64(0); i < count; i++ {
		out := dynamicpb.NewMessage(streamCountRespDesc)
		out.Set(out.Descriptor().Fields().ByName("value"), protoreflect.ValueOfInt32(int32(i)))
		if err := stream.SendMsg(out); err != nil {
			return err
		}
	}
	return nil
}

func handleClientSum(_ any, stream grpc.ServerStream) error {
	var total int32
	for {
		in := dynamicpb.NewMessage(streamSumReqDesc)
		if err := stream.RecvMsg(in); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		total += int32(in.Get(in.Descriptor().Fields().ByName("value")).Int())
	}
	out := dynamicpb.NewMessage(streamSumRespDesc)
	out.Set(out.Descriptor().Fields().ByName("total"), protoreflect.ValueOfInt32(total))
	return stream.SendMsg(out)
}

func handleChat(_ any, stream grpc.ServerStream) error {
	for {
		in := dynamicpb.NewMessage(streamChatDesc)
		if err := stream.RecvMsg(in); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		out := dynamicpb.NewMessage(streamChatDesc)
		out.Set(out.Descriptor().Fields().ByName("text"),
			protoreflect.ValueOfString("echo:"+in.Get(in.Descriptor().Fields().ByName("text")).String()))
		if err := stream.SendMsg(out); err != nil {
			return err
		}
	}
}

type streamReflectionServer struct {
	reflectionpb.UnimplementedServerReflectionServer
}

func (s *streamReflectionServer) ServerReflectionInfo(stream reflectionpb.ServerReflection_ServerReflectionInfoServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		var resp *reflectionpb.ServerReflectionResponse
		switch req.MessageRequest.(type) {
		case *reflectionpb.ServerReflectionRequest_ListServices:
			resp = createListServicesResponse(streamServiceName)
		case *reflectionpb.ServerReflectionRequest_FileContainingSymbol:
			resp = createFileDescriptorResponse(streamFile)
		case *reflectionpb.ServerReflectionRequest_FileByFilename:
			resp = createFileDescriptorResponse(streamFile)
		default:
			resp = createErrorResponse("unknown request type")
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func TestStreaming(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{streamFile})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}
	streamCountReqDesc = streamMessageDesc(t, files, "stream.test.CountRequest")
	streamCountRespDesc = streamMessageDesc(t, files, "stream.test.CountResponse")
	streamSumReqDesc = streamMessageDesc(t, files, "stream.test.SumRequest")
	streamSumRespDesc = streamMessageDesc(t, files, "stream.test.SumResponse")
	streamChatDesc = streamMessageDesc(t, files, "stream.test.ChatMessage")

	lis := bufconn.Listen(bufSize)
	server := grpc.NewServer()
	reflectionpb.RegisterServerReflectionServer(server, &streamReflectionServer{})
	server.RegisterService(&streamServiceDesc, struct{}{})
	go func() { _ = server.Serve(lis) }()
	defer server.Stop()

	ctx := context.Background()
	client, err := Dial(ctx, "passthrough://bufnet",
		WithInsecure(),
		WithDialOptions(grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		})),
	)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	if !client.SupportsMethod(streamServiceName + ".ServerCount") {
		t.Fatal("streaming service should be advertised")
	}

	// Server streaming.
	serverStream, err := client.ServerStream(ctx, streamServiceName+".ServerCount", []byte(`{"count":3}`))
	if err != nil {
		t.Fatalf("ServerStream failed: %v", err)
	}
	var values []int64
	for {
		msg, err := serverStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		values = append(values, msg.Get(msg.Descriptor().Fields().ByName("value")).Int())
	}
	serverStream.Close()
	if len(values) != 3 || values[0] != 0 || values[2] != 2 {
		t.Fatalf("server stream values = %v, want [0 1 2]", values)
	}

	// Client streaming.
	clientStream, err := client.ClientStream(ctx, streamServiceName+".ClientSum")
	if err != nil {
		t.Fatalf("ClientStream failed: %v", err)
	}
	for _, v := range []int32{1, 2, 3} {
		msg := dynamicpb.NewMessage(streamSumReqDesc)
		msg.Set(msg.Descriptor().Fields().ByName("value"), protoreflect.ValueOfInt32(v))
		if err := clientStream.Send(msg); err != nil {
			t.Fatalf("Send failed: %v", err)
		}
	}
	reply, err := clientStream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv failed: %v", err)
	}
	clientStream.Close()
	if got := reply.Get(reply.Descriptor().Fields().ByName("total")).Int(); got != 6 {
		t.Fatalf("client stream total = %d, want 6", got)
	}

	// Bidirectional streaming.
	bidi, err := client.BidiStream(ctx, streamServiceName+".Chat")
	if err != nil {
		t.Fatalf("BidiStream failed: %v", err)
	}
	for _, text := range []string{"a", "b"} {
		msg := dynamicpb.NewMessage(streamChatDesc)
		msg.Set(msg.Descriptor().Fields().ByName("text"), protoreflect.ValueOfString(text))
		if err := bidi.Send(msg); err != nil {
			t.Fatalf("bidi Send failed: %v", err)
		}
		got, err := bidi.Recv()
		if err != nil {
			t.Fatalf("bidi Recv failed: %v", err)
		}
		if value := got.Get(got.Descriptor().Fields().ByName("text")).String(); value != "echo:"+text {
			t.Fatalf("bidi reply = %q, want %q", value, "echo:"+text)
		}
	}
	_ = bidi.CloseSend()
	bidi.Close()

	// Kind mismatches must be rejected.
	if _, err := client.ServerStream(ctx, streamServiceName+".ClientSum", nil); err == nil {
		t.Fatal("ClientSum should not open as a server stream")
	}
	if _, err := client.ClientStream(ctx, streamServiceName+".ServerCount"); err == nil {
		t.Fatal("ServerCount should not open as a client stream")
	}
}

func streamMessageDesc(t *testing.T, files *protoregistry.Files, name string) protoreflect.MessageDescriptor {
	t.Helper()
	desc, err := files.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		t.Fatalf("find %s: %v", name, err)
	}
	md, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		t.Fatalf("%s is not a message", name)
	}
	return md
}

func streamInt32Field(name string, num int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(num),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
	}
}

func streamStringField(name string, num int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(num),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
	}
}
