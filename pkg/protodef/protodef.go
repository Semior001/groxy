package protodef

import "google.golang.org/protobuf/proto"

var globalDefiner = NewDefiner()

// SetDefiner sets the default definer for the package.
func SetDefiner(d *Definer) { globalDefiner = d }

// BuildMessage parses the protobuf definition with groxy options and
// returns a proto.Message that can be used to respond requests or match requests to.
func BuildMessage(def string) (proto.Message, error) { return globalDefiner.BuildTarget(def) }

// ParseTarget parses the protobuf definition with groxy options and
// returns a zero-valued proto.Message that can be used to parse messages
func ParseTarget(def string) (proto.Message, error) { return globalDefiner.ParseTarget(def) }

// Option is a functional option for the definer.
type Option func(*Definer)

// LoadOS sets definer to load the OS files, if they were requested in the definition.
func LoadOS(d *Definer) { d.loadFromOS = true }
