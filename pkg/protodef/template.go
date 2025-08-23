package protodef

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/expr-lang/expr"
	exprvm "github.com/expr-lang/expr/vm"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Template is an interface for generating protobuf messages based on templates.
type Template interface {
	DataMap(ctx context.Context, bts []byte) (map[string]any, error)
	Matches(ctx context.Context, bts []byte) (bool, error)
	Generate(ctx context.Context, data map[string]any) (proto.Message, error)
}

type templatedField struct {
	tmpl *template.Template
	desc *desc.FieldDescriptor
}

type matcherField struct {
	matcher *exprvm.Program
	desc    *desc.FieldDescriptor
}

type combined struct {
	desc     *desc.MessageDescriptor
	static   *dynamic.Message
	dynamic  []templatedField
	matchers []matcherField
}

// DataMap extracts all known fields from the provided byte sequence and returns them as a map.
func (t *combined) DataMap(_ context.Context, bts []byte) (map[string]any, error) {
	got := dynamic.NewMessage(t.desc)
	if err := got.Unmarshal(bts); err != nil {
		return nil, fmt.Errorf("unmarshal incoming message: %w", err)
	}
	out := make(map[string]any, len(got.GetKnownFields()))
	for _, field := range got.GetKnownFields() {
		out[field.GetName()] = got.GetField(field)
	}
	return out, nil
}

// Matches checks if the provided protobuf message matches the one in the template.
func (t *combined) Matches(ctx context.Context, bts []byte) (bool, error) {
	got, err := t.getStaticPart(bts)
	if err != nil {
		return false, fmt.Errorf("get static part of incoming message: %w", err)
	}

	if !dynamic.Equal(t.static, got) {
		return false, nil
	}

	got = dynamic.NewMessage(t.desc)
	if err = got.Unmarshal(bts); err != nil {
		return false, fmt.Errorf("unmarshal incoming message: %w", err)
	}

	for _, field := range t.matchers {
		val := got.GetField(field.desc)
		env := map[string]any{field.desc.GetName(): val, "ctx": ctx}

		out, err := expr.Run(field.matcher, env)
		if err != nil {
			return false, fmt.Errorf("evaluate matcher for field %s: %w", field.desc.GetName(), err)
		}

		matches, ok := out.(bool)
		if !ok {
			return false, fmt.Errorf("matcher for field %s did not return a boolean", field.desc.GetName())
		}

		if !matches {
			return false, nil
		}
	}

	return true, nil
}

func (t *combined) getStaticPart(bts []byte) (*dynamic.Message, error) {
	got := dynamic.NewMessage(t.desc)
	if err := got.Unmarshal(bts); err != nil {
		return nil, fmt.Errorf("unmarshal incoming message: %w", err)
	}

	// clear out unknown fields
	for _, tag := range got.GetUnknownFields() {
		got.ClearFieldByNumber(int(tag))
	}

	// clear out dynamic fields
	for _, field := range t.dynamic {
		got.ClearField(field.desc)
	}

	// clear out matcher fields
	for _, field := range t.matchers {
		got.ClearField(field.desc)
	}

	return got, nil
}

// Generate builds a new protobuf message out of a template and the provided data.
func (t *combined) Generate(ctx context.Context, data map[string]any) (proto.Message, error) {
	msg := dynamic.NewMessage(t.desc)
	if err := msg.MergeFrom(t.static); err != nil {
		return nil, fmt.Errorf("clone static fields into the new message: %w", err)
	}
	for _, field := range t.dynamic {
		sb := &strings.Builder{}

		input := map[string]any{"Context": ctx}
		for k, v := range data {
			input[k] = v
		}

		if err := field.tmpl.Execute(sb, input); err != nil {
			return nil, fmt.Errorf("execute template for field %s: %w", field.desc.GetName(), err)
		}

		if err := setField(msg, field.desc, sb.String()); err != nil {
			return nil, fmt.Errorf("set field %s: %w", field.desc.GetName(), err)
		}
	}
	return protoadapt.MessageV2Of(msg), nil
}

// Static is a Template that returns a static protobuf message without any modifications.
type Static struct {
	Desc    protoreflect.MessageDescriptor
	Message proto.Message
}

// DataMap returns a map of values, parsed from the provided byte sequence.
func (s Static) DataMap(_ context.Context, bts []byte) (map[string]any, error) {
	d, err := desc.WrapMessage(s.Desc)
	if err != nil {
		return nil, fmt.Errorf("wrap static message descriptor: %w", err)
	}

	got := dynamic.NewMessage(d)
	if err := got.Unmarshal(bts); err != nil {
		return nil, fmt.Errorf("unmarshal incoming message: %w", err)
	}

	out := make(map[string]any, len(got.GetKnownFields()))
	for _, field := range got.GetKnownFields() {
		out[field.GetName()] = got.GetField(field)
	}

	return out, nil
}

// Matches matches the byte sequence against the static message.
func (s Static) Matches(_ context.Context, got []byte) (bool, error) {
	want, err := proto.Marshal(s.Message)
	if err != nil {
		return false, fmt.Errorf("marshal static message: %w", err)
	}

	return bytes.Equal(got, want), nil
}

// Generate returns the static message without any modifications.
func (s Static) Generate(context.Context, map[string]any) (proto.Message, error) {
	return s.Message, nil
}
