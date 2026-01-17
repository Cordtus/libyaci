package libyaci

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// FallbackRegistry holds pre-compiled proto descriptors for types that may not
// be available via server reflection (e.g., deprecated modules).
type FallbackRegistry struct {
	files    *protoregistry.Files
	protoDir *ProtoDir // optional local proto directory
	mu       sync.RWMutex
}

// globalFallback is the default fallback registry used when no custom one is provided.
var globalFallback = &FallbackRegistry{
	files: &protoregistry.Files{},
}

// GlobalFallback returns the global fallback registry.
// Use this to register fallback descriptors that should be available to all clients.
func GlobalFallback() *FallbackRegistry {
	return globalFallback
}

// NewFallbackRegistry creates a new empty fallback registry.
func NewFallbackRegistry() *FallbackRegistry {
	return &FallbackRegistry{
		files: &protoregistry.Files{},
	}
}

// RegisterFileDescriptor registers a single file descriptor proto.
// Dependencies must be registered first.
func (r *FallbackRegistry) RegisterFileDescriptor(fdProto *descriptorpb.FileDescriptorProto) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	if _, err := r.files.FindFileByPath(fdProto.GetName()); err == nil {
		return nil // Already registered
	}

	fd, err := protodesc.NewFile(fdProto, r.files)
	if err != nil {
		return fmt.Errorf("failed to create file descriptor for %s: %w", fdProto.GetName(), err)
	}

	if err := r.files.RegisterFile(fd); err != nil {
		return fmt.Errorf("failed to register file %s: %w", fdProto.GetName(), err)
	}

	return nil
}

// RegisterFileDescriptorSet registers multiple file descriptor protos.
// Files are registered in order, so dependencies should come first.
func (r *FallbackRegistry) RegisterFileDescriptorSet(fds *descriptorpb.FileDescriptorSet) error {
	for _, fdProto := range fds.File {
		if err := r.RegisterFileDescriptor(fdProto); err != nil {
			return err
		}
	}
	return nil
}

// FindDescriptorByName looks up a descriptor by its full name.
// First checks the in-memory registry, then the local proto directory if configured.
func (r *FallbackRegistry) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	r.mu.RLock()

	// First check in-memory registry
	desc, err := r.files.FindDescriptorByName(name)
	if err == nil {
		r.mu.RUnlock()
		return desc, nil
	}

	// Check proto directory (lazy load if needed)
	protoDir := r.protoDir
	r.mu.RUnlock()

	if protoDir != nil {
		return protoDir.FindDescriptorByName(context.Background(), name)
	}

	return nil, err
}

// SetProtoDir configures a local proto directory for fallback resolution.
// The directory will be loaded lazily when first needed.
func (r *FallbackRegistry) SetProtoDir(dir *ProtoDir) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.protoDir = dir
}

// ProtoDir returns the configured proto directory, or nil if none.
func (r *FallbackRegistry) ProtoDir() *ProtoDir {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.protoDir
}

// Files returns the underlying file registry.
func (r *FallbackRegistry) Files() *protoregistry.Files {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.files
}

// MergeInto merges this fallback registry into a target protoregistry.Files.
// This is used during resolver initialization.
func (r *FallbackRegistry) MergeInto(target *protoregistry.Files) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var mergeErr error
	r.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		// Skip if already registered
		if _, err := target.FindFileByPath(fd.Path()); err == nil {
			return true
		}
		if err := target.RegisterFile(fd); err != nil {
			mergeErr = fmt.Errorf("failed to merge file %s: %w", fd.Path(), err)
			return false
		}
		return true
	})

	return mergeErr
}

// RegisterDeprecatedCosmosModules registers proto descriptors for deprecated
// Cosmos SDK modules that are no longer available via server reflection.
// Currently includes: tendermint.liquidity.v1beta1 (Gravity DEX)
func (r *FallbackRegistry) RegisterDeprecatedCosmosModules() error {
	// Register base dependencies first
	if err := r.registerCosmosCoinProto(); err != nil {
		return fmt.Errorf("failed to register cosmos coin proto: %w", err)
	}

	// Register liquidity module
	if err := r.registerLiquidityProto(); err != nil {
		return fmt.Errorf("failed to register liquidity proto: %w", err)
	}

	return nil
}

// registerCosmosCoinProto registers the cosmos.base.v1beta1.Coin message.
// This is a dependency for many Cosmos messages.
func (r *FallbackRegistry) registerCosmosCoinProto() error {
	// Check if already registered (might come from server reflection)
	if _, err := r.FindDescriptorByName("cosmos.base.v1beta1.Coin"); err == nil {
		return nil
	}

	fdProto := &descriptorpb.FileDescriptorProto{
		Name:    strPtr("cosmos/base/v1beta1/coin.proto"),
		Package: strPtr("cosmos.base.v1beta1"),
		Syntax:  strPtr("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strPtr("Coin"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("denom"),
						Number: int32Ptr(1),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:   strPtr("amount"),
						Number: int32Ptr(2),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
				},
			},
			{
				Name: strPtr("DecCoin"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("denom"),
						Number: int32Ptr(1),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:   strPtr("amount"),
						Number: int32Ptr(2),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
				},
			},
		},
	}

	return r.RegisterFileDescriptor(fdProto)
}

// registerLiquidityProto registers the tendermint.liquidity.v1beta1 messages.
// This module was deprecated and removed from Cosmos Hub.
func (r *FallbackRegistry) registerLiquidityProto() error {
	fdProto := &descriptorpb.FileDescriptorProto{
		Name:       strPtr("tendermint/liquidity/v1beta1/tx.proto"),
		Package:    strPtr("tendermint.liquidity.v1beta1"),
		Syntax:     strPtr("proto3"),
		Dependency: []string{"cosmos/base/v1beta1/coin.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			// MsgCreatePool
			{
				Name: strPtr("MsgCreatePool"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("pool_creator_address"),
						Number: int32Ptr(1),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:   strPtr("pool_type_id"),
						Number: int32Ptr(2),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_UINT32),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:     strPtr("deposit_coins"),
						Number:   int32Ptr(4),
						Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
						TypeName: strPtr(".cosmos.base.v1beta1.Coin"),
						Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED),
					},
				},
			},
			{
				Name:  strPtr("MsgCreatePoolResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{},
			},
			// MsgDepositWithinBatch
			{
				Name: strPtr("MsgDepositWithinBatch"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("depositor_address"),
						Number: int32Ptr(1),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:   strPtr("pool_id"),
						Number: int32Ptr(2),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_UINT64),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:     strPtr("deposit_coins"),
						Number:   int32Ptr(3),
						Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
						TypeName: strPtr(".cosmos.base.v1beta1.Coin"),
						Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED),
					},
				},
			},
			{
				Name:  strPtr("MsgDepositWithinBatchResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{},
			},
			// MsgWithdrawWithinBatch
			{
				Name: strPtr("MsgWithdrawWithinBatch"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("withdrawer_address"),
						Number: int32Ptr(1),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:   strPtr("pool_id"),
						Number: int32Ptr(2),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_UINT64),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:     strPtr("pool_coin"),
						Number:   int32Ptr(3),
						Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
						TypeName: strPtr(".cosmos.base.v1beta1.Coin"),
						Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
				},
			},
			{
				Name:  strPtr("MsgWithdrawWithinBatchResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{},
			},
			// MsgSwapWithinBatch
			{
				Name: strPtr("MsgSwapWithinBatch"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   strPtr("swap_requester_address"),
						Number: int32Ptr(1),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:   strPtr("pool_id"),
						Number: int32Ptr(2),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_UINT64),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:   strPtr("swap_type_id"),
						Number: int32Ptr(3),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_UINT32),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:     strPtr("offer_coin"),
						Number:   int32Ptr(4),
						Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
						TypeName: strPtr(".cosmos.base.v1beta1.Coin"),
						Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:   strPtr("demand_coin_denom"),
						Number: int32Ptr(5),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:     strPtr("offer_coin_fee"),
						Number:   int32Ptr(6),
						Type:     typePtr(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE),
						TypeName: strPtr(".cosmos.base.v1beta1.Coin"),
						Label:    labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
					{
						Name:   strPtr("order_price"),
						Number: int32Ptr(7),
						Type:   typePtr(descriptorpb.FieldDescriptorProto_TYPE_STRING),
						Label:  labelPtr(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					},
				},
			},
			{
				Name:  strPtr("MsgSwapWithinBatchResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{},
			},
		},
	}

	return r.RegisterFileDescriptor(fdProto)
}

// Helper functions for creating descriptor pointers
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
