package libyaci

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Response is a descriptor-backed dynamic protobuf response.
type Response struct {
	resolver *Resolver
	msg      *dynamicpb.Message
}

// Message returns the underlying dynamic protobuf message.
func (r *Response) Message() *dynamicpb.Message {
	return r.msg
}

// JSON marshals the response to protobuf JSON.
func (r *Response) JSON() ([]byte, error) {
	return marshalDynamicJSON(r.resolver, r.msg)
}

// Map marshals the response to protobuf JSON and decodes it into a map.
func (r *Response) Map() (map[string]any, error) {
	data, err := r.JSON()
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Field returns a top-level or dot-separated nested field value.
func (r *Response) Field(path string) (protoreflect.Value, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return protoreflect.Value{}, fmt.Errorf("field path is empty")
	}
	msg := r.msg.ProtoReflect()
	for i, part := range parts {
		fd := findField(msg.Descriptor().Fields(), part)
		if fd == nil {
			return protoreflect.Value{}, fmt.Errorf("field %q not found in path %q on %s", part, path, msg.Descriptor().FullName())
		}
		value := msg.Get(fd)
		if i == len(parts)-1 {
			return value, nil
		}
		if fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind {
			return protoreflect.Value{}, fmt.Errorf("field %q in path %q is not a message", part, path)
		}
		msg = value.Message()
	}
	return protoreflect.Value{}, fmt.Errorf("field path %q not found", path)
}

func marshalDynamicJSON(resolver *Resolver, msg *dynamicpb.Message) ([]byte, error) {
	mo := protojson.MarshalOptions{Resolver: resolver}
	result, err := mo.Marshal(msg)
	if err == nil {
		return result, nil
	}
	if msgType, fieldName, ok := parseUTF8Error(err.Error()); ok && resolver != nil {
		patchedResolver, patchErr := resolver.CreatePatchedResolver(msgType, fieldName)
		if patchErr != nil {
			return nil, fmt.Errorf("%w (UTF-8 recovery failed: %v)", err, patchErr)
		}
		patchedMo := protojson.MarshalOptions{Resolver: patchedResolver}
		patchedResult, patchedMarshalErr := patchedMo.Marshal(msg)
		if patchedMarshalErr != nil {
			return nil, fmt.Errorf("%w (patched marshal also failed: %v)", err, patchedMarshalErr)
		}
		return patchedResult, nil
	}
	return nil, err
}
