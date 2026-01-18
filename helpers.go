package libyaci

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ListServices returns all services available on the connected server.
func (c *Client) ListServices() []string {
	var services []string

	c.resolver.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			services = append(services, string(svcs.Get(i).FullName()))
		}
		return true
	})

	return services
}

// ListMethods returns all methods for a given service.
func (c *Client) ListMethods(serviceName string) ([]string, error) {
	var methods []string
	var found bool

	c.resolver.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		svcs := fd.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			if string(svc.FullName()) == serviceName {
				found = true
				methodList := svc.Methods()
				for j := 0; j < methodList.Len(); j++ {
					methods = append(methods, string(methodList.Get(j).Name()))
				}
				return false
			}
		}
		return true
	})

	if !found {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	return methods, nil
}

// GetMethodDescriptor returns the method descriptor for a given method.
func (c *Client) GetMethodDescriptor(method string) (protoreflect.MethodDescriptor, error) {
	serviceName, methodName, err := parseMethodFullName(method)
	if err != nil {
		return nil, err
	}
	return c.resolver.FindMethodDescriptor(serviceName, methodName)
}

// DescribeMethod returns a human-readable description of a method's input and output types.
func (c *Client) DescribeMethod(method string) (input, output string, err error) {
	desc, err := c.GetMethodDescriptor(method)
	if err != nil {
		return "", "", err
	}

	return string(desc.Input().FullName()), string(desc.Output().FullName()), nil
}

// InvokeJSON is a convenience method that accepts and returns Go values instead of JSON bytes.
// The request will be marshaled to JSON and the response will be unmarshaled into the result.
func (c *Client) InvokeJSON(method string, request interface{}, result interface{}) error {
	var reqBytes []byte
	var err error

	if request != nil {
		reqBytes, err = json.Marshal(request)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	respBytes, err := c.Invoke(method, reqBytes)
	if err != nil {
		return err
	}

	if result != nil {
		if err := json.Unmarshal(respBytes, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// ExtractField invokes a method and extracts a specific field from the response.
func (c *Client) ExtractField(method string, request []byte, fieldName string) (interface{}, error) {
	msg, err := c.InvokeRaw(method, request)
	if err != nil {
		return nil, err
	}

	field := msg.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field == nil {
		return nil, fmt.Errorf("field %s not found in response", fieldName)
	}

	value := msg.ProtoReflect().Get(field)
	if !value.IsValid() {
		return nil, fmt.Errorf("field %s is not set", fieldName)
	}

	return value.Interface(), nil
}

// DecodeTxBytes decodes raw protobuf transaction bytes to JSON.
// The raw bytes should be the protobuf-encoded cosmos.tx.v1beta1.Tx message.
// Handles UTF-8 errors by patching string fields that contain binary data.
func (c *Client) DecodeTxBytes(rawBytes []byte) ([]byte, error) {
	// Find the Tx message type
	txType, err := c.resolver.FindMessageByName("cosmos.tx.v1beta1.Tx")
	if err != nil {
		return nil, fmt.Errorf("find Tx message type: %w", err)
	}

	// Create a new message instance and unmarshal
	msg := txType.New().Interface()
	if err := proto.Unmarshal(rawBytes, msg); err != nil {
		return nil, fmt.Errorf("unmarshal tx bytes: %w", err)
	}

	// Marshal to JSON using protojson with resolver for Any type URLs
	mo := protojson.MarshalOptions{Resolver: c.resolver}
	jsonBytes, err := mo.Marshal(msg)
	if err != nil {
		// Check if this is a UTF-8 error that we can recover from
		if msgType, fieldName, ok := parseUTF8Error(err.Error()); ok {
			// Create a temporary patched resolver (does not modify the original)
			if patchedResolver, patchErr := c.resolver.CreatePatchedResolver(msgType, fieldName); patchErr == nil {
				// Retry marshal with the temporary patched resolver
				patchedMo := protojson.MarshalOptions{Resolver: patchedResolver}
				return patchedMo.Marshal(msg)
			}
		}
		return nil, fmt.Errorf("marshal to json: %w", err)
	}

	return jsonBytes, nil
}
