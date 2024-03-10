// Package protodef provides functions to make dynamic protobuf messages
// with the use of the groxypb options in protobuf snippets.
package protodef

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/Semior001/groxy/groxypb"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/types/descriptorpb"
	"gopkg.in/yaml.v3"
)

// Definer parses protobuf snippets and builds protobuf messages
// according to the specified groxy option values.
type Definer struct{ loadFromOS bool }

// NewDefiner returns a new Definer with the given options applied.
func NewDefiner(opts ...Option) *Definer {
	d := &Definer{}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// BuildTarget seeks the target message in the given protobuf snippet and
// returns a proto.Message that can be used to respond requests or match requests to.
func (b *Definer) BuildTarget(def string) (proto.Message, error) {
	def = b.enrich(def)

	fd, err := b.parseDefinition(def)
	if err != nil {
		return nil, fmt.Errorf("parse enriched definition: %w", err)
	}

	target, err := b.findTarget(fd)
	if err != nil {
		return nil, fmt.Errorf("find target message: %w", err)
	}

	msg, err := b.buildMessage(target)
	if err != nil {
		return nil, fmt.Errorf("parse message: %w", err)
	}

	return protoadapt.MessageV2Of(msg), nil
}

func (b *Definer) enrich(def string) string {
	sb := &strings.Builder{}
	_, _ = fmt.Fprintln(sb, `syntax = "proto3";`)
	_, _ = fmt.Fprintln(sb, `import "groxypb/annotations.proto";`)
	_, _ = fmt.Fprintln(sb)
	_, _ = fmt.Fprintln(sb, def)
	return sb.String()
}

func (b *Definer) parseDefinition(def string) (*desc.FileDescriptor, error) {
	p := protoparse.Parser{
		Accessor: func(f string) (io.ReadCloser, error) {
			switch {
			case f == "groxy-runtime-gen.proto":
				return io.NopCloser(strings.NewReader(def)), nil
			case b.loadFromOS:
				return os.Open(f) //nolint:gosec // this is on user's behalf
			default:
				return nil, fmt.Errorf("can't resolve not well-known file: %s", f)
			}
		},
		LookupImport: func(s string) (*desc.FileDescriptor, error) {
			if s != "groxypb/annotations.proto" {
				return nil, fmt.Errorf("imports in snippets are not supported: %s", s)
			}
			return desc.WrapFile(groxypb.File_groxypb_annotations_proto)
		},
	}

	fds, err := p.ParseFiles("groxy-runtime-gen.proto")
	if err != nil {
		var esp reporter.ErrorWithPos
		if errors.As(err, &esp) {
			pos := esp.GetPosition()
			return nil, errSyntax{
				Line: pos.Line - 3, // sub 3 lines to remove enriched parts
				Col:  pos.Col, Err: esp.Unwrap().Error(),
			}
		}
		return nil, fmt.Errorf("parse protobuf snippet: %w", err)
	}

	if len(fds) != 1 {
		panic("unexpected number of file descriptors")
	}

	return fds[0], nil
}

func (b *Definer) findTarget(fd *desc.FileDescriptor) (*desc.MessageDescriptor, error) {
	var targets []*desc.MessageDescriptor
	for _, md := range fd.GetMessageTypes() {
		b, ok := proto.GetExtension(protoadapt.MessageV2Of(md.GetOptions()), groxypb.E_Target).(bool)
		if !ok || !b {
			continue
		}

		targets = append(targets, md)
	}

	switch {
	case len(targets) == 0:
		return nil, errNoTarget
	case len(targets) > 1:
		return nil, errMultipleTarget(lo.Map(targets, func(md *desc.MessageDescriptor, _ int) string {
			return md.GetName()
		}))
	default:
		return targets[0], nil
	}
}

func (b *Definer) buildMessage(target *desc.MessageDescriptor) (*dynamic.Message, error) {
	msg := dynamic.NewMessage(target)
	for _, field := range target.GetFields() {
		if err := b.setField(msg, field); err != nil {
			return nil, fmt.Errorf("set field %q: %w", field.GetName(), err)
		}
	}
	return msg, nil
}

func (b *Definer) setField(msg *dynamic.Message, field *desc.FieldDescriptor) error {
	val, _ := proto.GetExtension(protoadapt.MessageV2Of(field.GetOptions()), groxypb.E_Value).(string)
	v, err := b.buildValue(fieldDescriptorWrapper{FieldDescriptor: field}, val)
	if err != nil {
		return fmt.Errorf("parse value: %w", err)
	}

	if err = msg.TrySetField(field, v); err != nil {
		return fmt.Errorf("set value: %w", err)
	}

	return nil
}

func (b *Definer) buildValue(field fieldDescriptor, s string) (any, error) {
	if s == "" {
		switch {
		case field.IsMap():
			return b.buildMap(field, "{}")
		case field.IsRepeated():
			return b.buildRepeated(field, "[]")
		case isMsg(field.GetType()): // repeated and map is just a repeated message
			return b.buildMessage(field.GetMessageType())
		case isEnum(field.GetType()):
			return int32(0), nil
		default:
			zeroVal, ok := types[field.GetType()]
			if !ok {
				return nil, fmt.Errorf("unknown field type: %s", field.GetType())
			}
			return zeroVal, nil
		}
	}

	switch {
	case field.IsMap():
		return b.buildMap(field, s)
	case field.IsRepeated():
		return b.buildRepeated(field, s)
	default:
		parser, ok := parsers[field.GetType()]
		if !ok {
			return nil, fmt.Errorf("unknown field type: %s", field.GetType())
		}

		parsed, err := parser(field, s)
		if err != nil {
			return nil, fmt.Errorf("parse value %q of type %s: %w", s, field.GetType(), err)
		}

		return cast(field.GetType(), parsed), nil
	}
}

func (b *Definer) buildMap(field fieldDescriptor, s string) (any, error) {
	kDescTyp, vDescTyp := field.GetMapKeyType().GetType(), field.GetMapValueType().GetType()

	kTyp, ok := types[kDescTyp]
	if !ok {
		return nil, fmt.Errorf("unknown map key type: %s", kDescTyp)
	}

	vTyp, ok := types[vDescTyp]
	if !ok && vDescTyp != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		return nil, fmt.Errorf("unknown map value type: %s", vDescTyp)
	}

	var m map[string]any
	if err := yaml.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("unmarshal map[%T]%T: %w", kTyp, vTyp, err)
	}

	if vDescTyp != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		// convert keys and values to the proper types
		mm := reflect.MakeMap(reflect.MapOf(reflect.TypeOf(kTyp), reflect.TypeOf(vTyp)))
		for k, v := range m {
			kk, err := parsers[kDescTyp](field, k)
			if err != nil {
				return nil, fmt.Errorf("parse map key %q of type %s: %w", k, kDescTyp, err)
			}

			vv := cast(vDescTyp, v)

			if str, ok := vv.(string); ok {
				parsed, err := parsers[vDescTyp](field, str)
				if err != nil {
					return nil, fmt.Errorf("parse map value %q of type %s: %w", v, vDescTyp, err)
				}
				vv = cast(vDescTyp, parsed)
			}

			mm.SetMapIndex(reflect.ValueOf(cast(kDescTyp, kk)), reflect.ValueOf(vv))
		}
		return mm.Interface(), nil
	}

	result := map[string]*dynamic.Message{}
	for k, v := range m {
		bts, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal map value: %w", err)
		}

		msg := dynamic.NewMessage(field.GetMapValueType().GetMessageType())
		if err = msg.UnmarshalJSON(bts); err != nil {
			return nil, fmt.Errorf("unmarshal map value: %w", err)
		}
		result[k] = msg
	}

	return result, nil
}

func (b *Definer) buildRepeated(field fieldDescriptor, s string) (any, error) {
	vTyp, ok := types[field.GetType()]
	if !ok && field.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		return nil, fmt.Errorf("unknown repeated value type: %s", field.GetType())
	}

	var iface []interface{}
	if err := yaml.Unmarshal([]byte(s), &iface); err != nil {
		return nil, fmt.Errorf("unmarshal []%T: %w", vTyp, err)
	}

	if field.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		// convert elements to the proper type
		sl := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(vTyp)), len(iface), len(iface))
		for i, v := range iface {
			sl.Index(i).Set(reflect.ValueOf(cast(field.GetType(), v)))
		}
		return sl.Interface(), nil
	}

	var result []*dynamic.Message
	for _, v := range iface {
		bts, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal nested message: %w", err)
		}

		msg := dynamic.NewMessage(field.GetMessageType())
		if err = msg.UnmarshalJSON(bts); err != nil {
			return nil, fmt.Errorf("unmarshal nested message: %w", err)
		}
		result = append(result, msg)
	}
	return result, nil
}

func isMsg(t descriptorpb.FieldDescriptorProto_Type) bool {
	return t == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
}

func isEnum(t descriptorpb.FieldDescriptorProto_Type) bool {
	return t == descriptorpb.FieldDescriptorProto_TYPE_ENUM
}

func cast(descTyp descriptorpb.FieldDescriptorProto_Type, v any) any {
	typ, ok := types[descTyp]
	if !ok {
		return v
	}

	vv := reflect.ValueOf(v)
	targetTyp := reflect.TypeOf(typ)
	if !vv.CanConvert(targetTyp) {
		return v
	}

	return vv.Convert(targetTyp).Interface()
}

var types = map[descriptorpb.FieldDescriptorProto_Type]any{
	// FieldDescriptorProto_TYPE_MESSAGE
	descriptorpb.FieldDescriptorProto_TYPE_ENUM:     int32(0),
	descriptorpb.FieldDescriptorProto_TYPE_BYTES:    []byte(nil),
	descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:   float64(0),
	descriptorpb.FieldDescriptorProto_TYPE_FLOAT:    float32(0),
	descriptorpb.FieldDescriptorProto_TYPE_INT64:    int64(0),
	descriptorpb.FieldDescriptorProto_TYPE_INT32:    int32(0),
	descriptorpb.FieldDescriptorProto_TYPE_FIXED64:  int64(0),
	descriptorpb.FieldDescriptorProto_TYPE_FIXED32:  int32(0),
	descriptorpb.FieldDescriptorProto_TYPE_BOOL:     false,
	descriptorpb.FieldDescriptorProto_TYPE_STRING:   "",
	descriptorpb.FieldDescriptorProto_TYPE_UINT32:   uint32(0),
	descriptorpb.FieldDescriptorProto_TYPE_UINT64:   uint64(0),
	descriptorpb.FieldDescriptorProto_TYPE_SFIXED32: int32(0),
	descriptorpb.FieldDescriptorProto_TYPE_SFIXED64: int64(0),
	descriptorpb.FieldDescriptorProto_TYPE_SINT32:   int32(0),
	descriptorpb.FieldDescriptorProto_TYPE_SINT64:   int64(0),
}

var parsers = map[descriptorpb.FieldDescriptorProto_Type]func(fd fieldDescriptor, s string) (any, error){
	descriptorpb.FieldDescriptorProto_TYPE_MESSAGE: func(fd fieldDescriptor, s string) (any, error) {
		msg := dynamic.NewMessage(fd.GetMessageType())
		if err := msg.UnmarshalJSON([]byte(s)); err != nil {
			return nil, fmt.Errorf("unmarshal nested message: %w", err)
		}
		return msg, nil
	},

	descriptorpb.FieldDescriptorProto_TYPE_ENUM: func(fd fieldDescriptor, s string) (any, error) {
		if enumVal := fd.GetEnumType().FindValueByName(s); enumVal != nil {
			return enumVal.GetNumber(), nil
		}
		return nil, fmt.Errorf("unknown enum value: %s", s)
	},

	descriptorpb.FieldDescriptorProto_TYPE_STRING: func(_ fieldDescriptor, s string) (any, error) { return s, nil },
	descriptorpb.FieldDescriptorProto_TYPE_BYTES:  func(_ fieldDescriptor, s string) (any, error) { return base64.StdEncoding.DecodeString(s) },
	descriptorpb.FieldDescriptorProto_TYPE_BOOL:   func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseBool(s) },

	descriptorpb.FieldDescriptorProto_TYPE_DOUBLE: func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseFloat(s, 64) },
	descriptorpb.FieldDescriptorProto_TYPE_FLOAT:  func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseFloat(s, 64) },

	descriptorpb.FieldDescriptorProto_TYPE_INT64:    func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseInt(s, 10, 64) },
	descriptorpb.FieldDescriptorProto_TYPE_INT32:    func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseInt(s, 10, 64) },
	descriptorpb.FieldDescriptorProto_TYPE_FIXED64:  func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseInt(s, 10, 64) },
	descriptorpb.FieldDescriptorProto_TYPE_FIXED32:  func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseInt(s, 10, 64) },
	descriptorpb.FieldDescriptorProto_TYPE_SFIXED32: func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseInt(s, 10, 64) },
	descriptorpb.FieldDescriptorProto_TYPE_SFIXED64: func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseInt(s, 10, 64) },
	descriptorpb.FieldDescriptorProto_TYPE_SINT32:   func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseInt(s, 10, 64) },
	descriptorpb.FieldDescriptorProto_TYPE_SINT64:   func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseInt(s, 10, 64) },

	descriptorpb.FieldDescriptorProto_TYPE_UINT32: func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseUint(s, 10, 64) },
	descriptorpb.FieldDescriptorProto_TYPE_UINT64: func(_ fieldDescriptor, s string) (any, error) { return strconv.ParseUint(s, 10, 64) },
}

// fieldDescriptor is an interface that allows to mock the desc.FieldDescriptor
type fieldDescriptor interface {
	IsMap() bool
	IsRepeated() bool
	GetType() descriptorpb.FieldDescriptorProto_Type
	GetMessageType() *desc.MessageDescriptor
	GetEnumType() *desc.EnumDescriptor

	GetMapKeyType() fieldDescriptor
	GetMapValueType() fieldDescriptor
}

type fieldDescriptorWrapper struct{ *desc.FieldDescriptor }

func (f fieldDescriptorWrapper) GetMapKeyType() fieldDescriptor {
	return fieldDescriptorWrapper{FieldDescriptor: f.FieldDescriptor.GetMapKeyType()}
}

func (f fieldDescriptorWrapper) GetMapValueType() fieldDescriptor {
	return fieldDescriptorWrapper{FieldDescriptor: f.FieldDescriptor.GetMapValueType()}
}
