// Package discovery provides the interface for matching gRPC requests to upstreams.
package discovery

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/grpc"
)

// Provider provides routing rules for the Service.
type Provider interface {
	// Name returns the name of the provider.
	Name() string

	// Events returns the events of the routing rules.
	// It returns the name of the provider to update the routing rules.
	Events(ctx context.Context) <-chan string

	// Rules returns the routing rules.
	Rules(ctx context.Context) ([]*Rule, error)

	// Upstreams returns the upstreams.
	Upstreams(ctx context.Context) ([]Upstream, error)
}

// Mock contains the details of how the handler should reply to the downstream.
type Mock struct {
	Header  metadata.MD
	Trailer metadata.MD
	Body    proto.Message
	Status  *status.Status
}

// Rule is a routing rule for the Service.
type Rule struct {
	// Name is an optional name of the rule.
	Name string

	// Match defines the request matcher.
	// Any request that matches the matcher will be handled by the rule.
	Match RequestMatcher

	// Mock defines the details of how the handler should reply to the downstream.
	Mock *Mock
}

// String returns the name of the rule.
func (r *Rule) String() string {
	sb := &strings.Builder{}
	_, _ = sb.WriteString("(")
	_, _ = sb.WriteString(r.Name)
	_, _ = sb.WriteString("; ")
	_, _ = sb.WriteString(strconv.Itoa(len(r.Match.IncomingMetadata)))
	_, _ = sb.WriteString(" metadata")
	if r.Match.Message != nil {
		_, _ = sb.WriteString("; with body: {")
		_, _ = sb.WriteString(protoadapt.MessageV1Of(r.Match.Message).String())
		_, _ = sb.WriteString("}")
	}
	_, _ = sb.WriteString(")")
	return sb.String()
}

// RequestMatcher defines parameters to match the request to the rule.
type RequestMatcher struct {
	// URI defines the fully-qualified method name, e.g.
	// "/package.Service/Method".
	URI *regexp.Regexp

	// IncomingMetadata contains the metadata of the incoming request.
	IncomingMetadata metadata.MD

	// Message contains the expected first RECV message of the request.
	Message proto.Message
}

// Matches returns true if the request metadata is matched to the rule.
func (r RequestMatcher) Matches(uri string, md metadata.MD) bool {
	if r.URI != nil && !r.URI.MatchString(uri) {
		return false
	}

	for k, v := range r.IncomingMetadata {
		if !slices.Equal(v, md.Get(k)) {
			return false
		}
	}

	return true
}

// Upstream specifies a gRPC client connection.
type Upstream interface {
	Name() string
	Reflection() bool

	Target() string
	Close() error
	grpc.ClientConnInterface
}

// NamedClosableClientConn is a named closable client connection.
type NamedClosableClientConn struct {
	ConnName        string
	ServeReflection bool
	*grpc.ClientConn
}

// Name returns the name of the connection.
func (n NamedClosableClientConn) Name() string { return n.ConnName }

// Reflection returns true if the connection serves reflection.
func (n NamedClosableClientConn) Reflection() bool { return n.ServeReflection }
