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
	"text/template"
	"text/template/parse"

	"github.com/expr-lang/expr"

	"github.com/Masterminds/sprig/v3"
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

var defaultFuncs = sprig.FuncMap()

// Definer parses protobuf snippets and builds protobuf messages
// according to the specified groxy option values.
type Definer struct {
	loadFromOS bool
	funcs      template.FuncMap
}

// NewDefiner returns a new Definer with the given options applied.
func NewDefiner(opts ...Option) *Definer {
	d := &Definer{funcs: defaultFuncs}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// BuildTarget seeks the target message in the given protobuf snippet and
// returns a proto.Message that can be used to respond requests or match requests to.
func (b *Definer) BuildTarget(def string) (Template, error) {
	def, err := b.joinMultilineStrings(def)
	if err != nil {
		return nil, fmt.Errorf("invalid file: %w", err)
	}

	def = b.enrich(def)

	fd, err := b.parseDefinition(def)
	if err != nil {
		return nil, fmt.Errorf("parse enriched definition: %w", err)
	}

	target, err := b.findTarget(fd)
	if err != nil {
		return nil, fmt.Errorf("find target message: %w", err)
	}

	tmpl, err := b.parseTemplate(target)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	return tmpl, nil
}

func (b *Definer) parseTemplate(target *desc.MessageDescriptor) (Template, error) {
	msg := &combined{desc: target, static: dynamic.NewMessage(target)}
	for _, field := range target.GetFields() {
		val, _ := proto.GetExtension(protoadapt.MessageV2Of(field.GetOptions()), groxypb.E_Value).(string)
		tmpl, err := template.New("").Funcs(b.funcs).Parse(val)
		if err != nil {
			return nil, fmt.Errorf("parse template for field %q: %w", field.GetName(), err)
		}

		if isTemplated(tmpl) {
			msg.dynamic = append(msg.dynamic, templatedField{tmpl: tmpl, desc: field})
			continue
		}

		matcher, _ := proto.GetExtension(protoadapt.MessageV2Of(field.GetOptions()), groxypb.E_Matcher).(string)
		if matcher != "" {
			prog, err := expr.Compile(matcher, expr.AsBool(), expr.AllowUndefinedVariables())
			if err != nil {
				return nil, fmt.Errorf("compile matcher for field %q: %w", field.GetName(), err)
			}
			msg.matchers = append(msg.matchers, matcherField{matcher: prog, desc: field})
			continue
		}

		if err = setField(msg.static, field, val); err != nil {
			return nil, fmt.Errorf("set static field %q: %w", field.GetName(), err)
		}
	}

	if len(msg.dynamic) == 0 && len(msg.matchers) == 0 {
		return &Static{Message: protoadapt.MessageV2Of(msg.static)}, nil
	}

	return msg, nil
}

// joinMultilineStrings replaces the multiline strings enclosed in "`" symbol into
// a single line string, enclosed in double quotes, with escaped newlines, tabs,
// and double quotes.
func (b *Definer) joinMultilineStrings(def string) (string, error) {
	var sb strings.Builder

	type pos struct{ Line, Col int }
	curr, backtickStart := pos{Line: 1}, pos{}

	countPos := func(r rune) {
		if r != '\n' {
			curr.Col++
			return
		}

		curr.Line++
		curr.Col = 0
	}

	inMultiline := false
	for _, r := range def {
		countPos(r)

		switch {
		case r == '`':
			if !inMultiline {
				backtickStart = curr
			}
			inMultiline = !inMultiline
			_, _ = sb.WriteRune('"')
		case inMultiline:
			switch r {
			case '\n':
				_, _ = sb.WriteString(`\n`)
			case '\t':
				_, _ = sb.WriteString(`\t`)
			case '"':
				_, _ = sb.WriteString(`\"`)
			default:
				_, _ = sb.WriteRune(r)
			}
		default:
			_, _ = sb.WriteRune(r)
		}
	}

	if inMultiline {
		return "", errUnclosedMultilineString(backtickStart)
	}

	return sb.String(), nil
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
	var targets []*desc.MessageDescriptor //nolint:prealloc // we expect only one target
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

func buildMessage(target *desc.MessageDescriptor) (*dynamic.Message, error) {
	msg := dynamic.NewMessage(target)
	for _, field := range target.GetFields() {
		val, _ := proto.GetExtension(protoadapt.MessageV2Of(field.GetOptions()), groxypb.E_Value).(string)
		if err := setField(msg, field, val); err != nil {
			return nil, fmt.Errorf("set field %q: %w", field.GetName(), err)
		}
	}
	return msg, nil
}

func setField(msg *dynamic.Message, field *desc.FieldDescriptor, val string) error {
	v, err := buildValue(fieldDescriptorWrapper{FieldDescriptor: field}, val)
	if err != nil {
		return fmt.Errorf("parse value: %w", err)
	}

	if err = msg.TrySetField(field, v); err != nil {
		return fmt.Errorf("set value: %w", err)
	}

	return nil
}

func buildValue(field fieldDescriptor, s string) (any, error) {
	if s == "" {
		switch {
		case field.IsMap():
			return buildMap(field, "{}")
		case field.IsRepeated():
			return buildRepeated(field, "[]")
		case isMsg(field.GetType()): // repeated and map is just a repeated message
			return buildMessage(field.GetMessageType())
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
		return buildMap(field, s)
	case field.IsRepeated():
		return buildRepeated(field, s)
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

func buildMap(field fieldDescriptor, s string) (any, error) {
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

func buildRepeated(field fieldDescriptor, s string) (any, error) {
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

	result := make([]*dynamic.Message, 0, len(iface))
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

func floatparser(_ fieldDescriptor, s string) (any, error) {
	return strconv.ParseFloat(s, 64)
}

func intparser(_ fieldDescriptor, s string) (any, error) {
	return strconv.ParseInt(s, 10, 64)
}

func uintparser(_ fieldDescriptor, s string) (any, error) {
	return strconv.ParseUint(s, 10, 64)
}

var parsers = map[descriptorpb.FieldDescriptorProto_Type]func(fd fieldDescriptor, s string) (any, error){
	descriptorpb.FieldDescriptorProto_TYPE_STRING: func(_ fieldDescriptor, s string) (any, error) { return s, nil },
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

	descriptorpb.FieldDescriptorProto_TYPE_BYTES: func(_ fieldDescriptor, s string) (any, error) {
		return base64.StdEncoding.DecodeString(s)
	},

	descriptorpb.FieldDescriptorProto_TYPE_BOOL: func(_ fieldDescriptor, s string) (any, error) {
		return strconv.ParseBool(s)
	},

	descriptorpb.FieldDescriptorProto_TYPE_DOUBLE: floatparser,
	descriptorpb.FieldDescriptorProto_TYPE_FLOAT:  floatparser,

	descriptorpb.FieldDescriptorProto_TYPE_INT64:    intparser,
	descriptorpb.FieldDescriptorProto_TYPE_INT32:    intparser,
	descriptorpb.FieldDescriptorProto_TYPE_FIXED64:  intparser,
	descriptorpb.FieldDescriptorProto_TYPE_FIXED32:  intparser,
	descriptorpb.FieldDescriptorProto_TYPE_SFIXED32: intparser,
	descriptorpb.FieldDescriptorProto_TYPE_SFIXED64: intparser,
	descriptorpb.FieldDescriptorProto_TYPE_SINT32:   intparser,
	descriptorpb.FieldDescriptorProto_TYPE_SINT64:   intparser,

	descriptorpb.FieldDescriptorProto_TYPE_UINT32: uintparser,
	descriptorpb.FieldDescriptorProto_TYPE_UINT64: uintparser,
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

func isTemplated(tmpl *template.Template) bool {
	if tmpl == nil {
		return false
	}

	var hasNonTextNodes func(node parse.Node) bool
	hasNonTextNodes = func(node parse.Node) bool {
		switch n := node.(type) {
		case *parse.ListNode:
			for _, child := range n.Nodes {
				if hasNonTextNodes(child) {
					return true
				}
			}
			return false
		case *parse.TextNode:
			return false
		default:
			return true
		}
	}

	for _, t := range tmpl.Templates() {
		if t.Root != nil && hasNonTextNodes(t.Root) {
			return true
		}
	}

	return false
}
