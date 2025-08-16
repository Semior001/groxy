package protodef

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

// Template is an interface for generating protobuf messages based on templates.
type Template interface {
	Generate(ctx context.Context, data any) (proto.Message, error)
}

type templatedField struct {
	tmpl *template.Template
	desc *desc.FieldDescriptor
}

type combined struct {
	desc    *desc.MessageDescriptor
	static  *dynamic.Message
	dynamic []templatedField
}

// Generate builds a new protobuf message out of a template and the provided data.
func (t *combined) Generate(ctx context.Context, data any) (proto.Message, error) {
	msg := dynamic.NewMessage(t.desc)
	if err := msg.MergeFrom(t.static); err != nil {
		return nil, fmt.Errorf("clone static fields into the new message: %w", err)
	}
	for _, field := range t.dynamic {
		sb := &strings.Builder{}

		input := struct {
			Ctx  context.Context
			Data any
		}{
			Ctx:  ctx,
			Data: data,
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
type Static struct{ proto.Message }

// Generate returns the static message without any modifications.
func (s Static) Generate(context.Context, any) (proto.Message, error) {
	return s.Message, nil
}
