package protodef

import (
	"encoding/json"
	"fmt"

	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

var globalDefiner = NewDefiner()

// BuildMessage parses the protobuf definition with groxy options and
// returns a proto.Message that can be used to respond requests or match requests to.
func BuildMessage(def string) (proto.Message, error) { return globalDefiner.BuildTarget(def) }

// Option is a functional option for the definer.
type Option func(*Definer)

// LoadOS sets definer to load the OS files, if they were requested in the definition.
func LoadOS(d *Definer) { d.loadFromOS = true }

// SetValues sets a set of arbitrary values to the message.
func SetValues(tmpl proto.Message, vals map[string]any) (proto.Message, error) {
	// FIXME: this is ugly, we need to set values explicitly for better performance
	tmpl = proto.Clone(tmpl)
	dmsg, err := dynamic.AsDynamicMessage(protoadapt.MessageV1Of(tmpl))
	if err != nil {
		return nil, fmt.Errorf("convert to dynamic message: %w", err)
	}

	bts, err := json.Marshal(vals)
	if err != nil {
		return nil, fmt.Errorf("marshal values: %w", err)
	}

	if err = dmsg.UnmarshalJSON(bts); err != nil {
		return nil, fmt.Errorf("unmarshal values: %w", err)
	}

	return protoadapt.MessageV2Of(dmsg), nil
}
