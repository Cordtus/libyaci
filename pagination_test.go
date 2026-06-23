package libyaci

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestMethodEachPagePassesNextKeyAndStopsWhenEmpty(t *testing.T) {
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

	var seenKeys []string
	calls := 0
	method := &Method{
		desc: methodDesc.(protoreflect.MethodDescriptor),
		call: func(_ context.Context, req *Request) (*Response, error) {
			calls++
			paginationField := req.Message().Descriptor().Fields().ByName("pagination")
			if req.Message().Has(paginationField) {
				pagination := req.Message().Get(paginationField).Message()
				keyField := pagination.Descriptor().Fields().ByName("key")
				seenKeys = append(seenKeys, string(pagination.Get(keyField).Bytes()))
			} else {
				seenKeys = append(seenKeys, "")
			}

			msg := dynamicpb.NewMessage(respDesc.(protoreflect.MessageDescriptor))
			pagination := msg.Mutable(msg.Descriptor().Fields().ByName("pagination")).Message()
			nextKeyField := pagination.Descriptor().Fields().ByName("next_key")
			if calls == 1 {
				pagination.Set(nextKeyField, protoreflect.ValueOfBytes([]byte("next")))
			}
			return &Response{msg: msg}, nil
		},
	}

	var pages int
	if err := method.EachPage(context.Background(), nil, func(*Response) error {
		pages++
		return nil
	}); err != nil {
		t.Fatalf("EachPage failed: %v", err)
	}
	if pages != 2 {
		t.Fatalf("pages = %d, want 2", pages)
	}
	if len(seenKeys) != 2 || seenKeys[0] != "" || seenKeys[1] != "next" {
		t.Fatalf("seen keys = %#v, want empty then next", seenKeys)
	}
}

func TestResponseNextKeyReadsStandardPagination(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{paginationFile()})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}
	respDesc, err := files.FindDescriptorByName("pagination.test.Response")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}
	msg := dynamicpb.NewMessage(respDesc.(protoreflect.MessageDescriptor))
	paginationField := msg.Descriptor().Fields().ByName("pagination")
	pagination := msg.Mutable(paginationField).Message()
	nextKeyField := pagination.Descriptor().Fields().ByName("next_key")
	pagination.Set(nextKeyField, protoreflect.ValueOfBytes([]byte("next")))

	got, ok, err := responseNextKey(&Response{msg: msg})
	if err != nil {
		t.Fatalf("responseNextKey failed: %v", err)
	}
	if !ok {
		t.Fatal("responseNextKey should report a present key")
	}
	if string(got) != "next" {
		t.Fatalf("next key = %q, want next", string(got))
	}
}

func paginationFile() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("pagination.proto"),
		Package: proto.String("pagination.test"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("PageResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("next_key"),
						JsonName: proto.String("nextKey"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("Response"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("pagination"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pagination.test.PageResponse"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}
}

func paginationLoopFile() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("pagination_loop.proto"),
		Package: proto.String("pagination.loop"),
		Syntax:  proto.String("proto3"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String("Query"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String("List"),
						InputType:  proto.String(".pagination.loop.Request"),
						OutputType: proto.String(".pagination.loop.Response"),
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("PageRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String("key"),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("PageResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("next_key"),
						JsonName: proto.String("nextKey"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("Request"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("pagination"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pagination.loop.PageRequest"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
			{
				Name: proto.String("Response"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("pagination"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".pagination.loop.PageResponse"),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}
}
