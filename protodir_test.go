package libyaci

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestProtoDir_Load(t *testing.T) {
	// Create temp directory with test proto
	tmpDir, err := os.MkdirTemp("", "libyaci-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write test proto file
	protoContent := `syntax = "proto3";
package test.example;

message TestMessage {
	string value = 1;
	int64 count = 2;
}

message AnotherMessage {
	TestMessage nested = 1;
	repeated string items = 2;
}
`
	protoPath := filepath.Join(tmpDir, "test.proto")
	if err := os.WriteFile(protoPath, []byte(protoContent), 0644); err != nil {
		t.Fatalf("failed to write proto: %v", err)
	}

	// Load and verify
	pd := NewProtoDir(tmpDir)
	if err := pd.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check messages are resolvable
	for _, name := range []string{"test.example.TestMessage", "test.example.AnotherMessage"} {
		desc, err := pd.FindDescriptorByName(context.Background(), protoreflect.FullName(name))
		if err != nil {
			t.Errorf("FindDescriptorByName(%s) failed: %v", name, err)
			continue
		}
		if desc == nil {
			t.Errorf("descriptor for %s is nil", name)
			continue
		}
		if string(desc.FullName()) != name {
			t.Errorf("got %s, want %s", desc.FullName(), name)
		}
	}

	// Verify IsLoaded returns true
	if !pd.IsLoaded() {
		t.Error("IsLoaded() should return true after Load()")
	}
}

func TestProtoDir_WithDependencies(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "libyaci-test-deps-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create cosmos/base/v1beta1 subdirectory structure
	coinDir := filepath.Join(tmpDir, "cosmos", "base", "v1beta1")
	if err := os.MkdirAll(coinDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Write coin.proto (dependency)
	coinProto := `syntax = "proto3";
package cosmos.base.v1beta1;

message Coin {
	string denom = 1;
	string amount = 2;
}
`
	if err := os.WriteFile(filepath.Join(coinDir, "coin.proto"), []byte(coinProto), 0644); err != nil {
		t.Fatalf("failed to write coin.proto: %v", err)
	}

	// Write proto that imports coin
	txProto := `syntax = "proto3";
package test.tx;

import "cosmos/base/v1beta1/coin.proto";

message Transfer {
	string sender = 1;
	string recipient = 2;
	cosmos.base.v1beta1.Coin amount = 3;
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "transfer.proto"), []byte(txProto), 0644); err != nil {
		t.Fatalf("failed to write transfer.proto: %v", err)
	}

	// Load and verify
	pd := NewProtoDir(tmpDir)
	if err := pd.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check both messages are resolvable
	for _, name := range []string{"cosmos.base.v1beta1.Coin", "test.tx.Transfer"} {
		desc, err := pd.FindDescriptorByName(context.Background(), protoreflect.FullName(name))
		if err != nil {
			t.Errorf("FindDescriptorByName(%s) failed: %v", name, err)
		}
		if desc == nil {
			t.Errorf("descriptor for %s is nil", name)
		}
	}
}

func TestProtoDir_EmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "libyaci-test-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pd := NewProtoDir(tmpDir)
	if err := pd.Load(context.Background()); err != nil {
		t.Fatalf("Load on empty dir should not error: %v", err)
	}

	if !pd.IsLoaded() {
		t.Error("IsLoaded() should return true even for empty dir")
	}
}

func TestProtoDir_NonexistentPath(t *testing.T) {
	pd := NewProtoDir("/nonexistent/path/that/does/not/exist")

	// Path should be set correctly
	if pd.Path() != "/nonexistent/path/that/does/not/exist" {
		t.Errorf("Path() = %s, want /nonexistent/path/that/does/not/exist", pd.Path())
	}

	// Load should fail for nonexistent path
	err := pd.Load(context.Background())
	if err == nil {
		t.Error("Load should fail for nonexistent path")
	}
}

func TestProtoDir_NotADirectory(t *testing.T) {
	// Create a file (not a directory)
	tmpFile, err := os.CreateTemp("", "libyaci-test-file-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	pd := NewProtoDir(tmpFile.Name())

	// Load should fail for file path
	err = pd.Load(context.Background())
	if err == nil {
		t.Error("Load should fail when path is a file, not a directory")
	}
}

func TestProtoDir_LazyLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "libyaci-test-lazy-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write test proto
	protoContent := `syntax = "proto3";
package lazy.test;

message LazyMessage {
	string value = 1;
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "lazy.proto"), []byte(protoContent), 0644); err != nil {
		t.Fatalf("failed to write proto: %v", err)
	}

	pd := NewProtoDir(tmpDir)

	// Should not be loaded yet
	if pd.IsLoaded() {
		t.Error("IsLoaded() should return false before Load()")
	}

	// FindDescriptorByName should trigger lazy load
	desc, err := pd.FindDescriptorByName(context.Background(), "lazy.test.LazyMessage")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}
	if desc == nil {
		t.Fatal("descriptor is nil")
	}

	// Should be loaded now
	if !pd.IsLoaded() {
		t.Error("IsLoaded() should return true after FindDescriptorByName")
	}
}

func TestProtoDir_MultipleLoadCalls(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "libyaci-test-multi-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write test proto
	protoContent := `syntax = "proto3";
package multi.test;

message MultiMessage {
	int32 id = 1;
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "multi.proto"), []byte(protoContent), 0644); err != nil {
		t.Fatalf("failed to write proto: %v", err)
	}

	pd := NewProtoDir(tmpDir)

	// Load multiple times - should be idempotent
	for i := 0; i < 3; i++ {
		if err := pd.Load(context.Background()); err != nil {
			t.Fatalf("Load() call %d failed: %v", i+1, err)
		}
	}

	// Verify message is still accessible
	desc, err := pd.FindDescriptorByName(context.Background(), "multi.test.MultiMessage")
	if err != nil {
		t.Fatalf("FindDescriptorByName failed: %v", err)
	}
	if desc == nil {
		t.Fatal("descriptor is nil")
	}
}

func TestSuggestProtoFile(t *testing.T) {
	tests := []struct {
		typeName     string
		wantContains string
	}{
		{
			typeName:     "tendermint.liquidity.v1beta1.MsgSwapWithinBatch",
			wantContains: "tendermint/liquidity/v1beta1",
		},
		{
			typeName:     "cosmos.bank.v1beta1.MsgSend",
			wantContains: "cosmos/bank/v1beta1",
		},
		{
			typeName:     "simple",
			wantContains: "add the proto definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			hint := suggestProtoFile(tt.typeName)
			if hint == "" {
				t.Error("suggestProtoFile returned empty hint")
			}
			if !strings.Contains(hint, tt.wantContains) {
				t.Errorf("hint %q should contain %q", hint, tt.wantContains)
			}
		})
	}
}

func TestTypeNotFoundError(t *testing.T) {
	originalErr := os.ErrNotExist
	err := &TypeNotFoundError{
		TypeName:    "test.Type",
		OriginalErr: originalErr,
		Hint:        "add proto file",
	}

	// Test Error() method
	msg := err.Error()
	if !strings.Contains(msg, "test.Type") {
		t.Errorf("error message should contain type name: %s", msg)
	}
	if !strings.Contains(msg, "add proto file") {
		t.Errorf("error message should contain hint: %s", msg)
	}

	// Test Unwrap() method
	if err.Unwrap() != originalErr {
		t.Error("Unwrap() should return original error")
	}
}
