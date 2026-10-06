package libyaci

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// ServerStream is a handle for a server-streaming RPC. Call Recv until it
// returns io.EOF, then Close.
type ServerStream struct {
	stream grpc.ClientStream
	desc   protoreflect.MethodDescriptor
	cancel context.CancelFunc
}

// Recv receives the next response message.
func (s *ServerStream) Recv() (*dynamicpb.Message, error) {
	msg := dynamicpb.NewMessage(s.desc.Output())
	if err := s.stream.RecvMsg(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// CloseSend half-closes the send direction.
func (s *ServerStream) CloseSend() error { return s.stream.CloseSend() }

// Context returns the stream context.
func (s *ServerStream) Context() context.Context { return s.stream.Context() }

// Close cancels the stream and releases its resources.
func (s *ServerStream) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

// ClientStream is a handle for a client-streaming RPC. Send messages, then call
// CloseAndRecv for the single response.
type ClientStream struct {
	stream grpc.ClientStream
	desc   protoreflect.MethodDescriptor
	cancel context.CancelFunc
}

// Send sends the next request message.
func (s *ClientStream) Send(msg *dynamicpb.Message) error { return s.stream.SendMsg(msg) }

// CloseAndRecv half-closes the send direction and receives the response.
func (s *ClientStream) CloseAndRecv() (*dynamicpb.Message, error) {
	if err := s.stream.CloseSend(); err != nil {
		return nil, err
	}
	out := dynamicpb.NewMessage(s.desc.Output())
	if err := s.stream.RecvMsg(out); err != nil {
		return nil, err
	}
	return out, nil
}

// Context returns the stream context.
func (s *ClientStream) Context() context.Context { return s.stream.Context() }

// Close cancels the stream and releases its resources.
func (s *ClientStream) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

// BidiStream is a handle for a bidirectional-streaming RPC.
type BidiStream struct {
	stream grpc.ClientStream
	desc   protoreflect.MethodDescriptor
	cancel context.CancelFunc
}

// Send sends the next request message.
func (s *BidiStream) Send(msg *dynamicpb.Message) error { return s.stream.SendMsg(msg) }

// Recv receives the next response message.
func (s *BidiStream) Recv() (*dynamicpb.Message, error) {
	msg := dynamicpb.NewMessage(s.desc.Output())
	if err := s.stream.RecvMsg(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// CloseSend half-closes the send direction.
func (s *BidiStream) CloseSend() error { return s.stream.CloseSend() }

// Context returns the stream context.
func (s *BidiStream) Context() context.Context { return s.stream.Context() }

// Close cancels the stream and releases its resources.
func (s *BidiStream) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

// ServerStream opens a server-streaming call and sends the single request.
func (m *Method) ServerStream(ctx context.Context, req *Request) (*ServerStream, error) {
	if !m.desc.IsStreamingServer() || m.desc.IsStreamingClient() {
		return nil, fmt.Errorf("method %s is not server-streaming", m.desc.FullName())
	}
	if req == nil {
		req = m.NewRequest()
	}
	if req.msg.Descriptor().FullName() != m.desc.Input().FullName() {
		return nil, fmt.Errorf("request type %s does not match method input %s", req.msg.Descriptor().FullName(), m.desc.Input().FullName())
	}

	callCtx, cancel := m.client.withClientContext(ctx)
	stream, err := m.client.conn.NewStream(callCtx, &grpc.StreamDesc{ServerStreams: true}, buildFullMethodPath(m.desc))
	if err != nil {
		cancel()
		return nil, err
	}
	if err := stream.SendMsg(req.msg); err != nil {
		cancel()
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		cancel()
		return nil, err
	}
	return &ServerStream{stream: stream, desc: m.desc, cancel: cancel}, nil
}

// ClientStream opens a client-streaming call.
func (m *Method) ClientStream(ctx context.Context) (*ClientStream, error) {
	if !m.desc.IsStreamingClient() || m.desc.IsStreamingServer() {
		return nil, fmt.Errorf("method %s is not client-streaming", m.desc.FullName())
	}
	callCtx, cancel := m.client.withClientContext(ctx)
	stream, err := m.client.conn.NewStream(callCtx, &grpc.StreamDesc{ClientStreams: true}, buildFullMethodPath(m.desc))
	if err != nil {
		cancel()
		return nil, err
	}
	return &ClientStream{stream: stream, desc: m.desc, cancel: cancel}, nil
}

// BidiStream opens a bidirectional-streaming call.
func (m *Method) BidiStream(ctx context.Context) (*BidiStream, error) {
	if !m.desc.IsStreamingClient() || !m.desc.IsStreamingServer() {
		return nil, fmt.Errorf("method %s is not bidirectional-streaming", m.desc.FullName())
	}
	callCtx, cancel := m.client.withClientContext(ctx)
	stream, err := m.client.conn.NewStream(callCtx, &grpc.StreamDesc{ClientStreams: true, ServerStreams: true}, buildFullMethodPath(m.desc))
	if err != nil {
		cancel()
		return nil, err
	}
	return &BidiStream{stream: stream, desc: m.desc, cancel: cancel}, nil
}

// ServerStream opens a server-streaming call with a protobuf-JSON request.
func (c *Client) ServerStream(ctx context.Context, method string, request []byte) (*ServerStream, error) {
	m, err := c.Method(method)
	if err != nil {
		return nil, err
	}
	req := m.NewRequest()
	if len(request) > 0 {
		if err := req.LoadJSON(request); err != nil {
			return nil, err
		}
	}
	return m.ServerStream(ctx, req)
}

// ClientStream opens a client-streaming call.
func (c *Client) ClientStream(ctx context.Context, method string) (*ClientStream, error) {
	m, err := c.Method(method)
	if err != nil {
		return nil, err
	}
	return m.ClientStream(ctx)
}

// BidiStream opens a bidirectional-streaming call.
func (c *Client) BidiStream(ctx context.Context, method string) (*BidiStream, error) {
	m, err := c.Method(method)
	if err != nil {
		return nil, err
	}
	return m.BidiStream(ctx)
}
