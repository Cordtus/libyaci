package libyaci

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

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

func TestFallbackRegistry_LiquidityModule(t *testing.T) {
	fb := NewFallbackRegistry()

	err := fb.RegisterDeprecatedCosmosModules()
	if err != nil {
		t.Fatalf("RegisterDeprecatedCosmosModules failed: %v", err)
	}

	// Check all liquidity messages are resolvable
	messages := []protoreflect.FullName{
		"tendermint.liquidity.v1beta1.MsgSwapWithinBatch",
		"tendermint.liquidity.v1beta1.MsgSwapWithinBatchResponse",
		"tendermint.liquidity.v1beta1.MsgDepositWithinBatch",
		"tendermint.liquidity.v1beta1.MsgDepositWithinBatchResponse",
		"tendermint.liquidity.v1beta1.MsgWithdrawWithinBatch",
		"tendermint.liquidity.v1beta1.MsgWithdrawWithinBatchResponse",
		"tendermint.liquidity.v1beta1.MsgCreatePool",
		"tendermint.liquidity.v1beta1.MsgCreatePoolResponse",
	}

	for _, msgName := range messages {
		desc, err := fb.FindDescriptorByName(msgName)
		if err != nil {
			t.Errorf("Failed to find %s: %v", msgName, err)
			continue
		}
		if desc == nil {
			t.Errorf("FindDescriptorByName returned nil for %s", msgName)
			continue
		}
		if desc.FullName() != msgName {
			t.Errorf("Expected full name '%s', got '%s'", msgName, desc.FullName())
		}
	}
}

func TestFallbackRegistry_CoinDependency(t *testing.T) {
	fb := NewFallbackRegistry()

	err := fb.RegisterDeprecatedCosmosModules()
	if err != nil {
		t.Fatalf("RegisterDeprecatedCosmosModules failed: %v", err)
	}

	// Verify cosmos.base.v1beta1.Coin is registered (dependency of liquidity)
	desc, err := fb.FindDescriptorByName("cosmos.base.v1beta1.Coin")
	if err != nil {
		t.Fatalf("Failed to find Coin: %v", err)
	}
	if desc == nil {
		t.Fatal("FindDescriptorByName returned nil for Coin")
	}

	// Check Coin has the expected fields
	msgDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		t.Fatal("Coin descriptor is not a MessageDescriptor")
	}

	fields := msgDesc.Fields()
	if fields.Len() != 2 {
		t.Fatalf("Expected 2 fields on Coin, got %d", fields.Len())
	}

	denomField := fields.ByName("denom")
	if denomField == nil {
		t.Fatal("Coin missing 'denom' field")
	}
	amountField := fields.ByName("amount")
	if amountField == nil {
		t.Fatal("Coin missing 'amount' field")
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

	// Run concurrent registrations and lookups
	done := make(chan bool)

	// Goroutine 1: Register deprecated modules
	go func() {
		for i := 0; i < 10; i++ {
			_ = fb.RegisterDeprecatedCosmosModules()
		}
		done <- true
	}()

	// Goroutine 2: Lookup messages
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = fb.FindDescriptorByName("tendermint.liquidity.v1beta1.MsgSwapWithinBatch")
		}
		done <- true
	}()

	// Goroutine 3: Access files
	go func() {
		for i := 0; i < 100; i++ {
			_ = fb.Files()
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done
}

func TestWithDeprecatedCosmosModules_Option(t *testing.T) {
	// Test that the option sets the correct flags
	o := defaultOptions()

	opt := WithDeprecatedCosmosModules()
	opt(o)

	if !o.registerDeprecated {
		t.Error("WithDeprecatedCosmosModules should set registerDeprecated to true")
	}
	if !o.useGlobalFallback {
		t.Error("WithDeprecatedCosmosModules should set useGlobalFallback to true when no fallback is configured")
	}
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
