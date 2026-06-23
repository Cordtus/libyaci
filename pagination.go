package libyaci

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// EachPage invokes a reflected method once per page for standard Cosmos SDK
// request/response messages that use pagination.key and pagination.next_key.
// If the method has no pagination fields, it invokes the method once.
func (m *Method) EachPage(ctx context.Context, values map[string]any, fn func(*Response) error) error {
	if fn == nil {
		return fmt.Errorf("page callback is nil")
	}

	req, err := m.RequestFromMap(values)
	if err != nil {
		return err
	}

	inputFields := req.Message().Descriptor().Fields()
	inputPagination := findField(inputFields, "pagination")
	if inputPagination == nil {
		resp, err := m.Call(ctx, req)
		if err != nil {
			return err
		}
		return fn(resp)
	}

	for {
		resp, err := m.Call(ctx, req)
		if err != nil {
			return err
		}
		if err := fn(resp); err != nil {
			return err
		}

		nextKey, ok, err := responseNextKey(resp)
		if err != nil {
			return err
		}
		if !ok || len(nextKey) == 0 {
			return nil
		}
		if err := req.SetPath("pagination.key", nextKey); err != nil {
			return err
		}
	}
}

func responseNextKey(resp *Response) ([]byte, bool, error) {
	msg := resp.Message().ProtoReflect()
	paginationField := findField(msg.Descriptor().Fields(), "pagination")
	if paginationField == nil || !msg.Has(paginationField) {
		return nil, false, nil
	}
	pagination := msg.Get(paginationField).Message()
	nextKeyField := findField(pagination.Descriptor().Fields(), "next_key")
	if nextKeyField == nil {
		nextKeyField = findField(pagination.Descriptor().Fields(), "nextKey")
	}
	if nextKeyField == nil || !pagination.Has(nextKeyField) {
		return nil, false, nil
	}
	if nextKeyField.Kind() != protoreflect.BytesKind {
		return nil, false, fmt.Errorf("pagination next key field has kind %s, want bytes", nextKeyField.Kind())
	}
	return pagination.Get(nextKeyField).Bytes(), true, nil
}
