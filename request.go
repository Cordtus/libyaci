package libyaci

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Request is a descriptor-backed dynamic protobuf request.
type Request struct {
	resolver *Resolver
	msg      *dynamicpb.Message
}

// Message returns the underlying dynamic protobuf message.
func (r *Request) Message() *dynamicpb.Message {
	return r.msg
}

// Set assigns a top-level field by proto name or JSON name.
func (r *Request) Set(field string, value any) error {
	if strings.Contains(field, ".") {
		return r.SetPath(field, value)
	}
	fd := findField(r.msg.Descriptor().Fields(), field)
	if fd == nil {
		return fmt.Errorf("field %q not found in %s", field, r.msg.Descriptor().FullName())
	}
	return setField(r.msg, fd, value, field)
}

// SetPath assigns a nested field by dot-separated proto or JSON names.
func (r *Request) SetPath(path string, value any) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return fmt.Errorf("field path is empty")
	}

	msg := r.msg
	for i, part := range parts {
		fd := findField(msg.Descriptor().Fields(), part)
		if fd == nil {
			return fmt.Errorf("field %q not found in path %q on %s", part, path, msg.Descriptor().FullName())
		}
		if i == len(parts)-1 {
			return setField(msg, fd, value, path)
		}
		if fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind {
			return fmt.Errorf("field %q in path %q is not a message", part, path)
		}
		msg = msg.Mutable(fd).Message().Interface().(*dynamicpb.Message)
	}
	return nil
}

// SetMap assigns multiple fields by proto name or JSON name.
func (r *Request) SetMap(values map[string]any) error {
	for field, value := range values {
		if err := r.Set(field, value); err != nil {
			return err
		}
	}
	return nil
}

// JSON marshals the request to protobuf JSON.
func (r *Request) JSON() ([]byte, error) {
	return protojson.MarshalOptions{Resolver: r.resolver}.Marshal(r.msg)
}

// LoadJSON unmarshals protobuf JSON into the request.
func (r *Request) LoadJSON(data []byte) error {
	return protojson.UnmarshalOptions{Resolver: r.resolver}.Unmarshal(data, r.msg)
}

func findField(fields protoreflect.FieldDescriptors, name string) protoreflect.FieldDescriptor {
	if fd := fields.ByName(protoreflect.Name(name)); fd != nil {
		return fd
	}
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.JSONName() == name || string(fd.TextName()) == name {
			return fd
		}
	}
	return nil
}

func setField(msg *dynamicpb.Message, fd protoreflect.FieldDescriptor, value any, path string) error {
	if fd.IsList() {
		return setListField(msg, fd, value, path)
	}
	if fd.IsMap() {
		return setMapField(msg, fd, value, path)
	}
	v, err := toProtoValue(fd, value, path)
	if err != nil {
		return err
	}
	msg.Set(fd, v)
	return nil
}

func setListField(msg *dynamicpb.Message, fd protoreflect.FieldDescriptor, value any, path string) error {
	rv := reflect.ValueOf(value)
	if value == nil || rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return fmt.Errorf("field %s expects repeated %s", path, fd.Kind())
	}
	list := msg.Mutable(fd).List()
	list.Truncate(0)
	for i := 0; i < rv.Len(); i++ {
		item, err := toProtoValue(fd, rv.Index(i).Interface(), fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return err
		}
		list.Append(item)
	}
	return nil
}

func setMapField(msg *dynamicpb.Message, fd protoreflect.FieldDescriptor, value any, path string) error {
	rv := reflect.ValueOf(value)
	if value == nil || rv.Kind() != reflect.Map {
		return fmt.Errorf("field %s expects map", path)
	}
	pm := msg.Mutable(fd).Map()
	pm.Range(func(key protoreflect.MapKey, _ protoreflect.Value) bool {
		pm.Clear(key)
		return true
	})
	iter := rv.MapRange()
	for iter.Next() {
		key, err := toMapKey(fd.MapKey(), iter.Key().Interface(), path)
		if err != nil {
			return err
		}
		mapValue, err := toProtoValue(fd.MapValue(), iter.Value().Interface(), fmt.Sprintf("%s[%v]", path, iter.Key().Interface()))
		if err != nil {
			return err
		}
		pm.Set(key, mapValue)
	}
	return nil
}

func toMapKey(fd protoreflect.FieldDescriptor, value any, path string) (protoreflect.MapKey, error) {
	v, err := toProtoValue(fd, value, path)
	if err != nil {
		return protoreflect.MapKey{}, err
	}
	return v.MapKey(), nil
}

func toProtoValue(fd protoreflect.FieldDescriptor, value any, path string) (protoreflect.Value, error) {
	if value == nil {
		return protoreflect.Value{}, fmt.Errorf("field %s cannot be nil", path)
	}

	switch fd.Kind() {
	case protoreflect.BoolKind:
		v, ok := value.(bool)
		if !ok {
			return protoreflect.Value{}, typeErr(path, "bool", value)
		}
		return protoreflect.ValueOfBool(v), nil
	case protoreflect.EnumKind:
		switch v := value.(type) {
		case string:
			if ev := fd.Enum().Values().ByName(protoreflect.Name(v)); ev != nil {
				return protoreflect.ValueOfEnum(ev.Number()), nil
			}
			return protoreflect.Value{}, fmt.Errorf("field %s unknown enum value %q", path, v)
		default:
			n, ok := signedInt(value)
			if !ok {
				return protoreflect.Value{}, typeErr(path, "enum name or number", value)
			}
			return protoreflect.ValueOfEnum(protoreflect.EnumNumber(n)), nil
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		v, ok := signedInt(value)
		if !ok || v < math.MinInt32 || v > math.MaxInt32 {
			return protoreflect.Value{}, typeErr(path, "int32", value)
		}
		return protoreflect.ValueOfInt32(int32(v)), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		v, ok := signedInt(value)
		if !ok {
			return protoreflect.Value{}, typeErr(path, "int64", value)
		}
		return protoreflect.ValueOfInt64(v), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		v, ok := unsignedInt(value)
		if !ok || v > math.MaxUint32 {
			return protoreflect.Value{}, typeErr(path, "uint32", value)
		}
		return protoreflect.ValueOfUint32(uint32(v)), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		v, ok := unsignedInt(value)
		if !ok {
			return protoreflect.Value{}, typeErr(path, "uint64", value)
		}
		return protoreflect.ValueOfUint64(v), nil
	case protoreflect.FloatKind:
		v, ok := floatNumber(value)
		if !ok {
			return protoreflect.Value{}, typeErr(path, "float", value)
		}
		return protoreflect.ValueOfFloat32(float32(v)), nil
	case protoreflect.DoubleKind:
		v, ok := floatNumber(value)
		if !ok {
			return protoreflect.Value{}, typeErr(path, "double", value)
		}
		return protoreflect.ValueOfFloat64(v), nil
	case protoreflect.StringKind:
		v, ok := value.(string)
		if !ok {
			return protoreflect.Value{}, typeErr(path, "string", value)
		}
		return protoreflect.ValueOfString(v), nil
	case protoreflect.BytesKind:
		switch v := value.(type) {
		case []byte:
			return protoreflect.ValueOfBytes(v), nil
		case string:
			return protoreflect.ValueOfBytes([]byte(v)), nil
		default:
			return protoreflect.Value{}, typeErr(path, "bytes", value)
		}
	case protoreflect.MessageKind, protoreflect.GroupKind:
		msg, err := toMessage(fd.Message(), value, path)
		if err != nil {
			return protoreflect.Value{}, err
		}
		return protoreflect.ValueOfMessage(msg), nil
	default:
		return protoreflect.Value{}, fmt.Errorf("field %s unsupported kind %s", path, fd.Kind())
	}
}

func toMessage(desc protoreflect.MessageDescriptor, value any, path string) (protoreflect.Message, error) {
	switch v := value.(type) {
	case *Request:
		if v.msg.Descriptor().FullName() != desc.FullName() {
			return nil, fmt.Errorf("field %s message type %s does not match %s", path, v.msg.Descriptor().FullName(), desc.FullName())
		}
		return v.msg.ProtoReflect(), nil
	case *dynamicpb.Message:
		if v.Descriptor().FullName() != desc.FullName() {
			return nil, fmt.Errorf("field %s message type %s does not match %s", path, v.Descriptor().FullName(), desc.FullName())
		}
		return v.ProtoReflect(), nil
	case proto.Message:
		if v.ProtoReflect().Descriptor().FullName() != desc.FullName() {
			return nil, fmt.Errorf("field %s message type %s does not match %s", path, v.ProtoReflect().Descriptor().FullName(), desc.FullName())
		}
		return v.ProtoReflect(), nil
	case map[string]any:
		msg := dynamicpb.NewMessage(desc)
		req := &Request{msg: msg}
		if err := req.SetMap(v); err != nil {
			return nil, err
		}
		return msg.ProtoReflect(), nil
	default:
		return nil, typeErr(path, "message or map", value)
	}
}

func signedInt(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		if uint64(v) > math.MaxInt64 {
			return 0, false
		}
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v > math.MaxInt64 {
			return 0, false
		}
		return int64(v), true
	case json.Number:
		n, err := v.Int64()
		return n, err == nil
	default:
		return 0, false
	}
}

func unsignedInt(value any) (uint64, bool) {
	switch v := value.(type) {
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int8:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int16:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int32:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case json.Number:
		n, err := v.Int64()
		if err != nil || n < 0 {
			return 0, false
		}
		return uint64(n), true
	default:
		return 0, false
	}
}

func floatNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	default:
		if n, ok := signedInt(value); ok {
			return float64(n), true
		}
		if n, ok := unsignedInt(value); ok {
			return float64(n), true
		}
		return 0, false
	}
}

func typeErr(path, want string, got any) error {
	return fmt.Errorf("field %s expects %s, got %T", path, want, got)
}
