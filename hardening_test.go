package libyaci

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unavailable", status.Error(codes.Unavailable, "down"), true},
		{"deadline", status.Error(codes.DeadlineExceeded, "slow"), true},
		{"resource exhausted", status.Error(codes.ResourceExhausted, "busy"), true},
		{"aborted", status.Error(codes.Aborted, "retry"), true},
		{"invalid argument", status.Error(codes.InvalidArgument, "bad request"), false},
		{"not found", status.Error(codes.NotFound, "missing"), false},
		{"permission denied", status.Error(codes.PermissionDenied, "no"), false},
		{"plain error", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.want {
				t.Fatalf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsHeightUnavailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"not found", status.Error(codes.NotFound, "no block"), true},
		{"invalid argument", status.Error(codes.InvalidArgument, "height 1 is not available"), true},
		{"out of range", status.Error(codes.OutOfRange, "beyond tip"), true},
		{"unavailable", status.Error(codes.Unavailable, "down"), false},
		{"lowest height text", errors.New("lowest height is 5200791"), true},
		{"pruned text", errors.New("requested block is pruned"), true},
		{"plain error", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHeightUnavailable(tt.err); got != tt.want {
				t.Fatalf("isHeightUnavailable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestBuildFileDescriptorSetSkipsUnresolvableFile(t *testing.T) {
	descriptors := []*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("good.proto"),
			Package: proto.String("good"),
			Syntax:  proto.String("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{
				{Name: proto.String("Good")},
			},
		},
		{
			Name:       proto.String("bad.proto"),
			Package:    proto.String("bad"),
			Syntax:     proto.String("proto3"),
			Dependency: []string{"missing.proto"},
			MessageType: []*descriptorpb.DescriptorProto{
				{Name: proto.String("Bad")},
			},
		},
	}

	files, skipped, err := buildFileDescriptorSetReport(descriptors)
	if err != nil {
		t.Fatalf("buildFileDescriptorSetReport failed: %v", err)
	}
	if len(skipped) != 1 || skipped[0] != "bad.proto" {
		t.Fatalf("skipped = %#v, want [bad.proto]", skipped)
	}
	if _, err := files.FindFileByPath("good.proto"); err != nil {
		t.Fatalf("good.proto should be registered: %v", err)
	}
}

func TestResolverFindExtension(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{extensionFile()})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}
	resolver := &Resolver{files: files}

	xt, err := resolver.FindExtensionByName("ext.test.note")
	if err != nil {
		t.Fatalf("FindExtensionByName failed: %v", err)
	}
	if xt == nil || xt.TypeDescriptor().Number() != 100 {
		t.Fatalf("unexpected extension type: %v", xt)
	}

	xt, err = resolver.FindExtensionByNumber("ext.test.Base", 100)
	if err != nil {
		t.Fatalf("FindExtensionByNumber failed: %v", err)
	}
	if xt == nil || xt.TypeDescriptor().FullName() != "ext.test.note" {
		t.Fatalf("unexpected extension type: %v", xt)
	}

	if _, err := resolver.FindExtensionByNumber("ext.test.Base", 999); !errors.Is(err, protoregistry.NotFound) {
		t.Fatalf("expected NotFound for missing extension number, got %v", err)
	}
}

func TestCreatePatchedResolverUsesFallback(t *testing.T) {
	fb := NewFallbackRegistry()
	fdProto := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("deprecated/msg.proto"),
		Package: proto.String("deprecated.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Historical"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("blob"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}
	if err := fb.RegisterFileDescriptor(fdProto); err != nil {
		t.Fatalf("RegisterFileDescriptor failed: %v", err)
	}

	resolver := &Resolver{files: &protoregistry.Files{}, fallback: fb}
	patched, err := resolver.CreatePatchedResolver("deprecated.v1.Historical", "blob")
	if err != nil {
		t.Fatalf("CreatePatchedResolver failed: %v", err)
	}

	desc, err := patched.Files().FindDescriptorByName("deprecated.v1.Historical")
	if err != nil {
		t.Fatalf("patched descriptor not found: %v", err)
	}
	md, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		t.Fatal("patched descriptor is not a message")
	}
	if got := md.Fields().ByName("blob").Kind(); got != protoreflect.BytesKind {
		t.Fatalf("patched field kind = %s, want bytes", got)
	}
}

func TestCreatePatchedResolverPrimary(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("primary/msg.proto"),
			Package: proto.String("primary.v1"),
			Syntax:  proto.String("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("Live"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:   proto.String("payload"),
							Number: proto.Int32(1),
							Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}

	resolver := &Resolver{files: files}
	patched, err := resolver.CreatePatchedResolver("primary.v1.Live", "payload")
	if err != nil {
		t.Fatalf("CreatePatchedResolver failed: %v", err)
	}

	// The original resolver must be untouched.
	orig, err := resolver.Files().FindDescriptorByName("primary.v1.Live")
	if err != nil {
		t.Fatalf("original descriptor missing: %v", err)
	}
	if got := orig.(protoreflect.MessageDescriptor).Fields().ByName("payload").Kind(); got != protoreflect.StringKind {
		t.Fatalf("original field kind = %s, want string", got)
	}

	desc, err := patched.Files().FindDescriptorByName("primary.v1.Live")
	if err != nil {
		t.Fatalf("patched descriptor not found: %v", err)
	}
	if got := desc.(protoreflect.MessageDescriptor).Fields().ByName("payload").Kind(); got != protoreflect.BytesKind {
		t.Fatalf("patched field kind = %s, want bytes", got)
	}
}

func TestEachPageStopsOnRepeatedKey(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{paginationLoopFile()})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}
	respDesc, err := files.FindDescriptorByName("pagination.loop.Response")
	if err != nil {
		t.Fatalf("FindDescriptorByName response failed: %v", err)
	}
	methodDesc, err := files.FindDescriptorByName("pagination.loop.Query.List")
	if err != nil {
		t.Fatalf("FindDescriptorByName method failed: %v", err)
	}

	calls := 0
	method := &Method{
		desc: methodDesc.(protoreflect.MethodDescriptor),
		call: func(_ context.Context, _ *Request) (*Response, error) {
			calls++
			msg := dynamicpb.NewMessage(respDesc.(protoreflect.MessageDescriptor))
			pagination := msg.Mutable(msg.Descriptor().Fields().ByName("pagination")).Message()
			pagination.Set(pagination.Descriptor().Fields().ByName("next_key"), protoreflect.ValueOfBytes([]byte("same")))
			return &Response{msg: msg}, nil
		},
	}

	err = method.EachPage(context.Background(), nil, func(*Response) error { return nil })
	if err == nil {
		t.Fatal("EachPage should fail when the next key does not advance")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestDialAgainstMockServer(t *testing.T) {
	client := dialMock(t)

	if !client.SupportsMethod("test.TestService.TestMethod") {
		t.Fatal("advertised method should be supported")
	}
	if _, ok := client.Catalog().Method("test.TestService.TestMethod"); !ok {
		t.Fatal("catalog should contain advertised method")
	}
	if len(client.SkippedFiles()) != 0 {
		t.Fatalf("SkippedFiles = %#v, want none", client.SkippedFiles())
	}
	if client.ProtoDir() != nil {
		t.Fatal("ProtoDir should be nil when not configured")
	}
}

func TestDialWithProtoDirClonesFallback(t *testing.T) {
	dir := t.TempDir()
	fb := NewFallbackRegistry()

	client := dialMock(t, WithFallbackRegistry(fb), WithProtoDir(dir))

	if fb.ProtoDir() != nil {
		t.Fatal("caller-owned fallback registry must not be mutated")
	}
	if client.ProtoDir() == nil {
		t.Fatal("client should have a proto directory")
	}
	if client.ProtoDir().Path() != dir {
		t.Fatalf("ProtoDir path = %q, want %q", client.ProtoDir().Path(), dir)
	}
}

func TestDialValidatesProtoDir(t *testing.T) {
	_, err := Dial(context.Background(), "passthrough://bufnet",
		WithInsecure(),
		WithProtoDir(filepath.Join(t.TempDir(), "does-not-exist")),
	)
	if err == nil {
		t.Fatal("Dial should fail for a nonexistent proto directory")
	}
}

func TestStreamingMethodReturnsClearError(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("stream.proto"),
			Package: proto.String("stream.test"),
			Syntax:  proto.String("proto3"),
			Service: []*descriptorpb.ServiceDescriptorProto{
				{
					Name: proto.String("S"),
					Method: []*descriptorpb.MethodDescriptorProto{
						{
							Name:            proto.String("Watch"),
							InputType:       proto.String(".stream.test.Req"),
							OutputType:      proto.String(".stream.test.Resp"),
							ServerStreaming: proto.Bool(true),
						},
					},
				},
			},
			MessageType: []*descriptorpb.DescriptorProto{
				{Name: proto.String("Req")},
				{Name: proto.String("Resp")},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}
	desc, err := files.FindDescriptorByName("stream.test.S.Watch")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}

	_, err = (&Client{}).invokeRawOnce(context.Background(), "/stream.test.S/Watch", desc.(protoreflect.MethodDescriptor), nil)
	if err == nil || !strings.Contains(err.Error(), "streaming") {
		t.Fatalf("expected streaming error, got %v", err)
	}
}

func TestCreatePatchedResolverWithProtoDirDependencies(t *testing.T) {
	dir := t.TempDir()
	writeProto(t, dir, "b.proto", `syntax = "proto3";
package dep.b;
message Inner { string id = 1; }
`)
	writeProto(t, dir, "a.proto", `syntax = "proto3";
package dep.a;
import "b.proto";
message Historical {
  string blob = 1;
  dep.b.Inner inner = 2;
}
`)

	fb := NewFallbackRegistry()
	fb.SetProtoDir(NewProtoDir(dir))
	resolver := &Resolver{files: &protoregistry.Files{}, fallback: fb, ctx: context.Background()}

	patched, err := resolver.CreatePatchedResolver("dep.a.Historical", "blob")
	if err != nil {
		t.Fatalf("CreatePatchedResolver with ProtoDir failed: %v", err)
	}
	desc, err := patched.Files().FindDescriptorByName("dep.a.Historical")
	if err != nil {
		t.Fatalf("patched descriptor not found: %v", err)
	}
	md := desc.(protoreflect.MessageDescriptor)
	if got := md.Fields().ByName("blob").Kind(); got != protoreflect.BytesKind {
		t.Fatalf("patched field kind = %s, want bytes", got)
	}
	// The imported dependency must have resolved too.
	if md.Fields().ByName("inner").Message().FullName() != "dep.b.Inner" {
		t.Fatal("imported dependency was not preserved in the patched registry")
	}
}

func dialMock(t *testing.T, extra ...Option) *Client {
	t.Helper()
	mock := newMockServer()
	t.Cleanup(mock.stop)

	opts := []Option{
		WithInsecure(),
		WithDialOptions(grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return mock.lis.Dial()
		})),
	}
	opts = append(opts, extra...)

	client, err := Dial(context.Background(), "passthrough://bufnet", opts...)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func writeProto(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func TestDecodeTxBytes(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{
		{
			Name:    proto.String("cosmos/tx/v1beta1/tx.proto"),
			Package: proto.String("cosmos.tx.v1beta1"),
			Syntax:  proto.String("proto3"),
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("Tx"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:   proto.String("body"),
							Number: proto.Int32(1),
							Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}

	desc, err := files.FindDescriptorByName("cosmos.tx.v1beta1.Tx")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}
	msg := dynamicpb.NewMessage(desc.(protoreflect.MessageDescriptor))
	msg.Set(msg.Descriptor().Fields().ByName("body"), protoreflect.ValueOfString("hello"))
	raw, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	client := &Client{resolver: &Resolver{files: files}}
	out, err := client.DecodeTxBytes(raw)
	if err != nil {
		t.Fatalf("DecodeTxBytes failed: %v", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Fatalf("decoded JSON = %s, want it to contain hello", out)
	}
}

func extensionFile() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("ext.proto"),
		Package: proto.String("ext.test"),
		Syntax:  proto.String("proto2"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Base"),
				ExtensionRange: []*descriptorpb.DescriptorProto_ExtensionRange{
					{Start: proto.Int32(100), End: proto.Int32(200)},
				},
			},
		},
		Extension: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("note"),
				Number:   proto.Int32(100),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Extendee: proto.String(".ext.test.Base"),
			},
		},
	}
}
