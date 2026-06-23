package libyaci

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestCatalogReportsAdvertisedServicesOnly(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{
		serviceFile("test.proto", "test", "Query", "Balance"),
		serviceFile("dep.proto", "dep", "Internal", "Hidden"),
	})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}

	catalog := newCatalog(files, []string{"test.Query"}, ChainInfo{SDKVersion: "0.47.0"})

	if !catalog.HasService("test.Query") {
		t.Fatal("catalog should report advertised service")
	}
	if catalog.HasService("dep.Internal") {
		t.Fatal("catalog should not report descriptor-only dependency service")
	}
	if !catalog.HasMethod("test.Query.Balance") {
		t.Fatal("catalog should report advertised method")
	}
	if catalog.HasMethod("dep.Internal.Hidden") {
		t.Fatal("catalog should not report method on non-advertised service")
	}
}

func TestUnsupportedMethodErrorSupportsErrorsIs(t *testing.T) {
	err := &UnsupportedMethodError{
		Method:     "cosmos.foo.v1.Query.Missing",
		Service:    "cosmos.foo.v1.Query",
		SDKVersion: "0.47.0",
		Reason:     "service is not advertised by reflection",
	}

	if !errors.Is(err, ErrUnsupportedMethod) {
		t.Fatal("UnsupportedMethodError should unwrap ErrUnsupportedMethod")
	}
	if got := err.Error(); got == "" || !contains(got, "cosmos.foo.v1.Query.Missing") || !contains(got, "0.47.0") {
		t.Fatalf("error should include method and SDK version, got %q", got)
	}
}

func TestReflectionWinsOverConfiguredSDKVersion(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{
		serviceFile("blockresults.proto", "cosmos.base.tendermint.v1beta1", "Service", "GetBlockResults"),
	})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}

	catalog := newCatalog(files, []string{"cosmos.base.tendermint.v1beta1.Service"}, ChainInfo{SDKVersion: "0.47.0"})
	if !catalog.HasMethod(methodGetBlockResults) {
		t.Fatal("reflected method should be supported even when configured SDK version is older")
	}
}

func TestGetBlockResultsReturnsUnsupportedWhenNotReflected(t *testing.T) {
	files, err := buildFileDescriptorSet([]*descriptorpb.FileDescriptorProto{
		serviceFile("tm.proto", "cosmos.base.tendermint.v1beta1", "Service", "GetLatestBlock"),
	})
	if err != nil {
		t.Fatalf("buildFileDescriptorSet failed: %v", err)
	}
	client := &Client{
		catalog:  newCatalog(files, []string{"cosmos.base.tendermint.v1beta1.Service"}, ChainInfo{SDKVersion: "0.47.0"}),
		resolver: &Resolver{files: files},
	}

	_, err = client.GetBlockResults(10)
	if !errors.Is(err, ErrUnsupportedMethod) {
		t.Fatalf("GetBlockResults should return ErrUnsupportedMethod, got %v", err)
	}
}

func serviceFile(path, pkg, service, method string) *descriptorpb.FileDescriptorProto {
	req := method + "Request"
	resp := method + "Response"
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String(path),
		Package: proto.String(pkg),
		Syntax:  proto.String("proto3"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: proto.String(service),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       proto.String(method),
						InputType:  proto.String("." + pkg + "." + req),
						OutputType: proto.String("." + pkg + "." + resp),
					},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String(req)},
			{Name: proto.String(resp)},
		},
	}
}
