package protodef

import (
	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"
	"text/template"
)

// BuildTarget parses the protobuf definition with groxy options and
// returns a proto.Message that can be used to respond requests or match requests to.
func BuildTarget(def string, data any, opts ...Option) (proto.Message, error) {
	return NewDefiner(opts...).BuildTarget(def, data)
}

// Option is a functional option for the definer.
type Option func(*Definer)

// LoadOS sets definer to load the OS files, if they were requested in the definition.
func LoadOS(d *Definer) { d.loadFromOS = true }

// WithFuncs sets the definer to use the provided functions for templating.
// Note: function with the name that has been already defined will be overwritten.
func WithFuncs(funcs template.FuncMap) Option {
	return func(d *Definer) {
		d.templateFuncs = lo.Assign(d.templateFuncs, funcs)
	}
}
