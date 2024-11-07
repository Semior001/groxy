package protodef

import (
	"errors"
	"fmt"
	"slices"
)

var errNoTarget = errors.New("no target message found")

type errMultipleTarget []string

func (e errMultipleTarget) Error() string {
	return fmt.Sprintf("multiple target messages found: %v", []string(e))
}

func (e errMultipleTarget) Is(target error) bool {
	var errMultipleTarget errMultipleTarget
	ok := errors.As(target, &errMultipleTarget)
	return ok && slices.Equal(e, errMultipleTarget)
}

type errSyntax struct {
	Line int
	Col  int
	Err  string
}

func (e errSyntax) Error() string {
	return fmt.Sprintf("(%d:%d) %s", e.Line, e.Col, e.Err)
}

type errUnclosedMultilineString struct {
	Line int
	Col  int
}

func (e errUnclosedMultilineString) Error() string {
	return fmt.Sprintf("(%d:%d) unclosed multiline string", e.Line, e.Col)
}
