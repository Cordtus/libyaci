package libyaci

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Method is a reusable reflected gRPC method handle.
type Method struct {
	client *Client
	desc   protoreflect.MethodDescriptor
	info   MethodInfo
	call   func(context.Context, *Request) (*Response, error)
}

// Info returns reflected method metadata.
func (m *Method) Info() MethodInfo {
	return m.info
}

// NewRequest creates an empty descriptor-backed request for this method.
func (m *Method) NewRequest() *Request {
	var resolver *Resolver
	if m.client != nil {
		resolver = m.client.resolver
	}
	return &Request{
		resolver: resolver,
		msg:      dynamicpb.NewMessage(m.desc.Input()),
	}
}

// RequestFromMap creates a request and populates it from field names or JSON names.
func (m *Method) RequestFromMap(values map[string]any) (*Request, error) {
	req := m.NewRequest()
	if err := req.SetMap(values); err != nil {
		return nil, err
	}
	return req, nil
}

// Call invokes this reflected method with a descriptor-backed request.
func (m *Method) Call(ctx context.Context, req *Request) (*Response, error) {
	if m.call != nil {
		return m.call(ctx, req)
	}
	if req == nil {
		req = m.NewRequest()
	}
	if req.msg.Descriptor().FullName() != m.desc.Input().FullName() {
		return nil, fmt.Errorf("request type %s does not match method input %s", req.msg.Descriptor().FullName(), m.desc.Input().FullName())
	}

	fullMethodPath := buildFullMethodPath(m.desc)
	callCtx, cancel := m.client.withClientContext(ctx)
	defer cancel()

	outputMsg, err := m.client.invokeRawOnce(callCtx, fullMethodPath, m.desc, req.msg)
	if err != nil {
		return nil, err
	}
	return &Response{resolver: m.client.resolver, msg: outputMsg}, nil
}

// CallMap creates a request from a map and invokes this method.
func (m *Method) CallMap(ctx context.Context, values map[string]any) (*Response, error) {
	req, err := m.RequestFromMap(values)
	if err != nil {
		return nil, err
	}
	return m.Call(ctx, req)
}

// Method returns a reflected method handle.
func (c *Client) Method(name string) (*Method, error) {
	desc, err := c.methodDescriptor(name)
	if err != nil {
		return nil, err
	}
	info, _ := c.catalog.Method(name)
	return &Method{client: c, desc: desc, info: info}, nil
}

// NewMessage creates an empty reflected message by full type name.
func (c *Client) NewMessage(fullName string) (*Request, error) {
	msgType, err := c.resolver.FindMessageByName(protoreflect.FullName(fullName))
	if err != nil {
		return nil, err
	}
	return &Request{
		resolver: c.resolver,
		msg:      msgType.New().Interface().(*dynamicpb.Message),
	}, nil
}
