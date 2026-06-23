package libyaci

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestRequestBuilderSetsScalarRepeatedEnumAndNestedFields(t *testing.T) {
	msg := testRequestMessage(t)
	req := &Request{msg: msg}

	err := req.SetMap(map[string]any{
		"address": "cosmos1abc",
		"height":  int64(123),
		"status":  "ACTIVE",
		"tags":    []any{"one", "two"},
		"page": map[string]any{
			"limit": uint64(50),
		},
	})
	if err != nil {
		t.Fatalf("SetMap failed: %v", err)
	}

	fields := msg.Descriptor().Fields()
	if got := msg.Get(fields.ByName("address")).String(); got != "cosmos1abc" {
		t.Fatalf("address = %q", got)
	}
	if got := msg.Get(fields.ByName("height")).Int(); got != 123 {
		t.Fatalf("height = %d", got)
	}
	if got := msg.Get(fields.ByName("status")).Enum(); got != 1 {
		t.Fatalf("status = %d", got)
	}
	if got := msg.Get(fields.ByName("tags")).List().Len(); got != 2 {
		t.Fatalf("tags len = %d", got)
	}
	page := msg.Get(fields.ByName("page")).Message()
	if got := page.Get(page.Descriptor().Fields().ByName("limit")).Uint(); got != 50 {
		t.Fatalf("page.limit = %d", got)
	}
}

func TestRequestBuilderEscapesUnsafeStrings(t *testing.T) {
	msg := testRequestMessage(t)
	req := &Request{msg: msg}
	unsafe := `cosmos1"bad\value`

	if err := req.Set("address", unsafe); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	data, err := req.JSON()
	if err != nil {
		t.Fatalf("JSON failed: %v", err)
	}
	if !strings.Contains(string(data), `cosmos1\"bad\\value`) {
		t.Fatalf("JSON did not safely escape string: %s", data)
	}
}

func TestRequestBuilderRejectsUnknownFieldWithPath(t *testing.T) {
	req := &Request{msg: testRequestMessage(t)}
	err := req.SetPath("page.missing", 1)
	if err == nil || !strings.Contains(err.Error(), "page.missing") {
		t.Fatalf("expected path-aware unknown field error, got %v", err)
	}
}

func TestRequestBuilderRejectsWrongTypeWithFieldContext(t *testing.T) {
	req := &Request{msg: testRequestMessage(t)}
	err := req.Set("height", "not a number")
	if err == nil || !strings.Contains(err.Error(), "height") || !strings.Contains(err.Error(), "int64") {
		t.Fatalf("expected typed field error, got %v", err)
	}
}

func testRequestMessage(t *testing.T) *dynamicpb.Message {
	t.Helper()
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("request.proto"),
		Package: proto.String("request.test"),
		Syntax:  proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("Status"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("UNKNOWN"), Number: proto.Int32(0)},
					{Name: proto.String("ACTIVE"), Number: proto.Int32(1)},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Page"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("limit"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("Request"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("address"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
					{
						Name:   proto.String("height"),
						Number: proto.Int32(2),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
					{
						Name:     proto.String("status"),
						Number:   proto.Int32(3),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
						TypeName: proto.String(".request.test.Status"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
					{
						Name:   proto.String("tags"),
						Number: proto.Int32(4),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
					},
					{
						Name:     proto.String("page"),
						Number:   proto.Int32(5),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".request.test.Page"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{fd})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}
	desc, err := files.FindDescriptorByName(protoreflect.FullName("request.test.Request"))
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}
	return dynamicpb.NewMessage(desc.(protoreflect.MessageDescriptor))
}
