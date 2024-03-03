package groxypb

import _ "embed"

//go:embed annotations.proto
var annotation string

// Annotation returns the content of the annotations.proto file.
func Annotation() string { return annotation }
