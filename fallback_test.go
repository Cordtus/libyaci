package libyaci

import (
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

// Helper functions for creating proto descriptors in tests
func strPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}

func typePtr(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type {
	return &t
}

func labelPtr(l descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label {
	return &l
}

func TestNewFallbackRegistry(t *testing.T) {
	fb := NewFallbackRegistry()
	if fb == nil {
		t.Fatal("NewFallbackRegistry returned nil")
	}
	if fb.files == nil {
		t.Fatal("FallbackRegistry.files is nil")
	}
}

func TestGlobalFallback(t *testing.T) {
	fb1 := GlobalFallback()
	fb2 := GlobalFallback()
	if fb1 != fb2 {
		t.Fatal("GlobalFallback should return the same instance")
	}
}

func TestFallbackRegistry_RegisterFileDescriptor(t *testing.T) {
	fb := NewFallbackRegistry()

	// Create a simple file descriptor
	fdProto := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("test/test.proto"),
		Package: strPtr("test"),
		Syntax:  strPtr("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("TestMessage"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("value"),
						Number: int32Ptr(1),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
				},
			},
		},
	}

	err := fb.RegisterFileDescriptor(fdProto)
	if err != nil {
		t.Fatalf("RegisterFileDescriptor failed: %v", err)
	}

	// Registering again should be a no-op (not an error)
	err = fb.RegisterFileDescriptor(fdProto)
	if err != nil {
		t.Fatalf("Re-registering file descriptor failed: %v", err)
	}
}

func TestFallbackRegistry_FindDescriptorByName(t *testing.T) {
	fb := NewFallbackRegistry()

	// Create and register a simple file descriptor
	fdProto := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("test/find.proto"),
		Package: strPtr("test.find"),
		Syntax:  strPtr("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("FindMe"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("id"),
						Number: int32Ptr(1),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_INT64),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
				},
			},
		},
	}

	err := fb.RegisterFileDescriptor(fdProto)
	if err != nil {
		t.Fatalf("RegisterFileDescriptor failed: %v", err)
	}

	// Find the registered message
	desc, err := fb.FindDescriptorByName("test.find.FindMe")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}
	if desc == nil {
		t.Fatal("FindDescriptorByName returned nil descriptor")
	}
	if desc.FullName() != "test.find.FindMe" {
		t.Fatalf("Expected full name 'test.find.FindMe', got '%s'", desc.FullName())
	}

	// Try to find non-existent message
	_, err = fb.FindDescriptorByName("test.find.NotThere")
	if err == nil {
		t.Fatal("Expected error for non-existent message")
	}
}

func TestFallbackRegistry_MergeInto(t *testing.T) {
	fb := NewFallbackRegistry()

	// Register a test file
	fdProto := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("merge/test.proto"),
		Package: strPtr("merge.test"),
		Syntax:  strPtr("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  strPtr("MergeTest"),
				Field: []*descriptorpb.FieldDescriptorProto{},
			},
		},
	}

	err := fb.RegisterFileDescriptor(fdProto)
	if err != nil {
		t.Fatalf("RegisterFileDescriptor failed: %v", err)
	}

	// Create a new fallback to merge into
	target := NewFallbackRegistry()

	err = fb.MergeInto(target.files)
	if err != nil {
		t.Fatalf("MergeInto failed: %v", err)
	}

	// Verify the message is findable in target
	desc, err := target.FindDescriptorByName("merge.test.MergeTest")
	if err != nil {
		t.Fatalf("Failed to find merged message: %v", err)
	}
	if desc == nil {
		t.Fatal("FindDescriptorByName returned nil for merged message")
	}
}

func TestFallbackRegistry_ThreadSafety(t *testing.T) {
	fb := NewFallbackRegistry()

	// Register a test file for lookup
	fdProto := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("thread/test.proto"),
		Package: strPtr("thread.test"),
		Syntax:  strPtr("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  strPtr("ThreadTest"),
				Field: []*descriptorpb.FieldDescriptorProto{},
			},
		},
	}
	if err := fb.RegisterFileDescriptor(fdProto); err != nil {
		t.Fatalf("RegisterFileDescriptor failed: %v", err)
	}

	// Run concurrent lookups
	done := make(chan bool)

	// Goroutine 1: Lookup messages
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = fb.FindDescriptorByName("thread.test.ThreadTest")
		}
		done <- true
	}()

	// Goroutine 2: Access files
	go func() {
		for i := 0; i < 100; i++ {
			_ = fb.Files()
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
}

func TestWithFallbackRegistry_Option(t *testing.T) {
	fb := NewFallbackRegistry()
	o := defaultOptions()

	opt := WithFallbackRegistry(fb)
	opt(o)

	if o.fallback != fb {
		t.Error("WithFallbackRegistry should set the fallback registry")
	}
}

func TestWithGlobalFallback_Option(t *testing.T) {
	o := defaultOptions()

	opt := WithGlobalFallback()
	opt(o)

	if !o.useGlobalFallback {
		t.Error("WithGlobalFallback should set useGlobalFallback to true")
	}
}

func TestWithProtoDir_Option(t *testing.T) {
	o := defaultOptions()

	opt := WithProtoDir("/path/to/protos")
	opt(o)

	if o.protoDir != "/path/to/protos" {
		t.Error("WithProtoDir should set protoDir")
	}
	if !o.useGlobalFallback {
		t.Error("WithProtoDir should enable useGlobalFallback when no fallback is configured")
	}
}
