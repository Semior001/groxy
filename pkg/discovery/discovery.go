// Package discovery provides the interface for matching gRPC requests to upstreams.
package discovery

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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
}

// Mock contains the details of how the handler should reply to the downstream.
type Mock struct {
	Header   metadata.MD
	Trailer  metadata.MD
	Messages []proto.Message
	Status   *status.Status
}

// String returns the string representation of the mock.
func (m Mock) String() string {
	return fmt.Sprintf("mock{header: %d; trailer: %d; messages: %d; status: %q}",
		len(m.Header), len(m.Trailer), len(m.Messages), m.Status)
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
func (r *Rule) String() string { return fmt.Sprintf("rule{name: %s}", r.Name) }

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
