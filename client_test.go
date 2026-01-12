package libyaci

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestParseMethodFullName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantService string
		wantMethod  string
		wantErr     bool
	}{
		{
			name:        "valid cosmos method",
			input:       "cosmos.bank.v1beta1.Query.Balance",
			wantService: "cosmos.bank.v1beta1.Query",
			wantMethod:  "Balance",
			wantErr:     false,
		},
		{
			name:        "valid simple method",
			input:       "Service.Method",
			wantService: "Service",
			wantMethod:  "Method",
			wantErr:     false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no dot",
			input:   "NoServiceMethod",
			wantErr: true,
		},
		{
			name:    "trailing dot",
			input:   "Service.",
			wantErr: true,
		},
		{
			name:    "leading dot",
			input:   ".Method",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, method, err := parseMethodFullName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if service != tt.wantService {
				t.Errorf("service = %q, want %q", service, tt.wantService)
			}
			if method != tt.wantMethod {
				t.Errorf("method = %q, want %q", method, tt.wantMethod)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()

	if opts.insecure {
		t.Error("insecure should default to false")
	}
	if opts.maxRetries != defaultMaxRetries {
		t.Errorf("maxRetries = %d, want %d", opts.maxRetries, defaultMaxRetries)
	}
	if opts.maxRecvMsgSize != defaultMaxRecvMsgSize {
		t.Errorf("maxRecvMsgSize = %d, want %d", opts.maxRecvMsgSize, defaultMaxRecvMsgSize)
	}
}

func TestWithOptions(t *testing.T) {
	opts := defaultOptions()

	WithInsecure()(opts)
	if !opts.insecure {
		t.Error("WithInsecure should set insecure to true")
	}

	WithMaxRetries(10)(opts)
	if opts.maxRetries != 10 {
		t.Errorf("WithMaxRetries: got %d, want 10", opts.maxRetries)
	}

	WithMaxRecvMsgSize(8 * 1024 * 1024)(opts)
	if opts.maxRecvMsgSize != 8*1024*1024 {
		t.Errorf("WithMaxRecvMsgSize: got %d, want %d", opts.maxRecvMsgSize, 8*1024*1024)
	}

	WithDialTimeout(30 * time.Second)(opts)
	if opts.dialTimeout != 30*time.Second {
		t.Errorf("WithDialTimeout: got %v, want %v", opts.dialTimeout, 30*time.Second)
	}
}

func TestFetchAllDescriptors(t *testing.T) {
	ctx := context.Background()
	mock := newMockServer()
	defer mock.stop()

	conn, err := mock.dial(ctx)
	if err != nil {
		t.Fatalf("failed to dial mock server: %v", err)
	}
	defer conn.Close()

	descriptors, err := fetchAllDescriptors(ctx, conn, 3)
	if err != nil {
		t.Fatalf("fetchAllDescriptors failed: %v", err)
	}

	if len(descriptors) == 0 {
		t.Error("expected at least one descriptor")
	}
}

func TestBuildFileDescriptorSet(t *testing.T) {
	tests := []struct {
		name        string
		descriptors []*descriptorpb.FileDescriptorProto
		wantErr     string
	}{
		{
			name: "valid descriptors with dependency",
			descriptors: []*descriptorpb.FileDescriptorProto{
				{
					Name:       proto.String("file1.proto"),
					Dependency: []string{"file2.proto"},
				},
				{
					Name: proto.String("file2.proto"),
				},
			},
		},
		{
			name: "circular dependency",
			descriptors: []*descriptorpb.FileDescriptorProto{
				{
					Name:       proto.String("file1.proto"),
					Dependency: []string{"file2.proto"},
				},
				{
					Name:       proto.String("file2.proto"),
					Dependency: []string{"file1.proto"},
				},
			},
			wantErr: "circular dependency",
		},
		{
			name: "single file no dependencies",
			descriptors: []*descriptorpb.FileDescriptorProto{
				{
					Name: proto.String("single.proto"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := buildFileDescriptorSet(tt.descriptors)
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
					return
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if files == nil {
				t.Error("expected non-nil files")
				return
			}
			for _, fd := range tt.descriptors {
				if _, err := files.FindFileByPath(fd.GetName()); err != nil {
					t.Errorf("file %s not found in registry: %v", fd.GetName(), err)
				}
			}
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name    string
		fdMap   map[string]*descriptorpb.FileDescriptorProto
		wantErr bool
	}{
		{
			name: "linear dependency chain",
			fdMap: map[string]*descriptorpb.FileDescriptorProto{
				"a.proto": {Name: proto.String("a.proto"), Dependency: []string{"b.proto"}},
				"b.proto": {Name: proto.String("b.proto"), Dependency: []string{"c.proto"}},
				"c.proto": {Name: proto.String("c.proto")},
			},
		},
		{
			name: "diamond dependency",
			fdMap: map[string]*descriptorpb.FileDescriptorProto{
				"a.proto": {Name: proto.String("a.proto"), Dependency: []string{"b.proto", "c.proto"}},
				"b.proto": {Name: proto.String("b.proto"), Dependency: []string{"d.proto"}},
				"c.proto": {Name: proto.String("c.proto"), Dependency: []string{"d.proto"}},
				"d.proto": {Name: proto.String("d.proto")},
			},
		},
		{
			name: "circular dependency",
			fdMap: map[string]*descriptorpb.FileDescriptorProto{
				"a.proto": {Name: proto.String("a.proto"), Dependency: []string{"b.proto"}},
				"b.proto": {Name: proto.String("b.proto"), Dependency: []string{"a.proto"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorted, err := topologicalSort(tt.fdMap)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(sorted) != len(tt.fdMap) {
				t.Errorf("sorted length = %d, want %d", len(sorted), len(tt.fdMap))
			}
			// Verify dependencies come before dependents
			seen := make(map[string]bool)
			for _, fd := range sorted {
				for _, dep := range fd.Dependency {
					if _, exists := tt.fdMap[dep]; exists && !seen[dep] {
						t.Errorf("dependency %s should come before %s", dep, fd.GetName())
					}
				}
				seen[fd.GetName()] = true
			}
		})
	}
}

func TestBuildFullMethodPath(t *testing.T) {
	// Create a minimal file descriptor with a service and method
	fdProto := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("test.package"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("TestService"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("TestMethod"),
						InputType:  proto.String(".test.package.Request"),
						OutputType: proto.String(".test.package.Response"),
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Request")},
			{Name: proto.String("Response")},
		},
	}

	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{fdProto})
	if err != nil {
		t.Fatalf("failed to build file descriptor set: %v", err)
	}

	resolver := &Resolver{files: files}
	methodDesc, err := resolver.FindMethodDescriptor("test.package.TestService", "TestMethod")
	if err != nil {
		t.Fatalf("failed to find method descriptor: %v", err)
	}

	path := buildFullMethodPath(methodDesc)
	expected := "/test.package.TestService/TestMethod"
	if path != expected {
		t.Errorf("buildFullMethodPath = %q, want %q", path, expected)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestParseUTF8Error(t *testing.T) {
	tests := []struct {
		name          string
		errStr        string
		wantMsgType   string
		wantFieldName string
		wantOK        bool
	}{
		{
			name:          "interchain security consumer_key error",
			errStr:        `proto: google.protobuf.Any: unable to unmarshal "/interchain_security.ccv.provider.v1.MsgAssignConsumerKey": field interchain_security.ccv.provider.v1.MsgAssignConsumerKey.consumer_key contains invalid UTF-8`,
			wantMsgType:   "interchain_security.ccv.provider.v1.MsgAssignConsumerKey",
			wantFieldName: "consumer_key",
			wantOK:        true,
		},
		{
			name:          "simple message.field error",
			errStr:        `field cosmos.base.abci.v1beta1.TxResponse.raw_log contains invalid UTF-8`,
			wantMsgType:   "cosmos.base.abci.v1beta1.TxResponse",
			wantFieldName: "raw_log",
			wantOK:        true,
		},
		{
			name:          "nested message error",
			errStr:        `field some.package.OuterMsg.inner_field contains invalid UTF-8`,
			wantMsgType:   "some.package.OuterMsg",
			wantFieldName: "inner_field",
			wantOK:        true,
		},
		{
			name:   "no UTF-8 error",
			errStr: `failed to unmarshal: unexpected end of JSON input`,
			wantOK: false,
		},
		{
			name:   "UTF-8 mentioned but different format",
			errStr: `invalid UTF-8 in file`,
			wantOK: false,
		},
		{
			name:   "empty string",
			errStr: ``,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgType, fieldName, ok := parseUTF8Error(tt.errStr)
			if ok != tt.wantOK {
				t.Errorf("parseUTF8Error ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if !tt.wantOK {
				return
			}
			if msgType != tt.wantMsgType {
				t.Errorf("parseUTF8Error msgType = %q, want %q", msgType, tt.wantMsgType)
			}
			if fieldName != tt.wantFieldName {
				t.Errorf("parseUTF8Error fieldName = %q, want %q", fieldName, tt.wantFieldName)
			}
		})
	}
}

func TestPatchFieldInProto(t *testing.T) {
	tests := []struct {
		name        string
		fdProto     *descriptorpb.FileDescriptorProto
		messageType string
		fieldName   string
		wantPatched bool
	}{
		{
			name: "patch simple field",
			fdProto: &descriptorpb.FileDescriptorProto{
				Name:    proto.String("test.proto"),
				Package: proto.String("test.package"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("TestMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("string_field"),
								Number: proto.Int32(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
						},
					},
				},
			},
			messageType: "test.package.TestMessage",
			fieldName:   "string_field",
			wantPatched: true,
		},
		{
			name: "field not found",
			fdProto: &descriptorpb.FileDescriptorProto{
				Name:    proto.String("test.proto"),
				Package: proto.String("test.package"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("TestMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("other_field"),
								Number: proto.Int32(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
						},
					},
				},
			},
			messageType: "test.package.TestMessage",
			fieldName:   "nonexistent_field",
			wantPatched: false,
		},
		{
			name: "field is already bytes",
			fdProto: &descriptorpb.FileDescriptorProto{
				Name:    proto.String("test.proto"),
				Package: proto.String("test.package"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("TestMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("bytes_field"),
								Number: proto.Int32(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
							},
						},
					},
				},
			},
			messageType: "test.package.TestMessage",
			fieldName:   "bytes_field",
			wantPatched: false,
		},
		{
			name: "message not found",
			fdProto: &descriptorpb.FileDescriptorProto{
				Name:    proto.String("test.proto"),
				Package: proto.String("test.package"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("OtherMessage"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   proto.String("string_field"),
								Number: proto.Int32(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
						},
					},
				},
			},
			messageType: "test.package.TestMessage",
			fieldName:   "string_field",
			wantPatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patched := patchFieldInProto(tt.fdProto, tt.messageType, tt.fieldName)
			if patched != tt.wantPatched {
				t.Errorf("patchFieldInProto = %v, want %v", patched, tt.wantPatched)
			}
			if tt.wantPatched {
				// Verify the field type was actually changed
				for _, msg := range tt.fdProto.GetMessageType() {
					for _, field := range msg.GetField() {
						if field.GetName() == tt.fieldName {
							if field.GetType() != descriptorpb.FieldDescriptorProto_TYPE_BYTES {
								t.Errorf("field type = %v, want TYPE_BYTES", field.GetType())
							}
						}
					}
				}
			}
		})
	}
}
