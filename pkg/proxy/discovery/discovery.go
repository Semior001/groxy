// Package discovery provides the interface for matching gRPC requests to upstreams.
package discovery

import (
	"context"
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
	Rules(ctx context.Context) ([]Rule, error)
}

// Rule is a routing rule for the Service.
type Rule struct {
	// Match defines the request matcher.
	// Any request that matches the matcher will be handled by the rule.
	Match RequestMatcher

	// Mock defines the details of how the handler should reply to the downstream.
	Mock *Mock
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

// Matches returns true if the request matches the rule.
func (r RequestMatcher) Matches(req *Request) bool {
	if r.URI != nil && !r.URI.MatchString(req.URI) {
		return false
	}

	for k, v := range r.IncomingMetadata {
		if !slices.Equal(v, req.IncomingMetadata.Get(k)) {
			return false
		}
	}

	if r.Message != nil {
		if req.FirstRecv == nil {
			return false
		}

		msg := r.Message.ProtoReflect().New()

		if err := proto.Unmarshal(req.FirstRecv, msg.Interface()); err != nil {
			return false
		}

		if !proto.Equal(msg.Interface(), r.Message) {
			return false
		}
	}

	return true
}

// Request defines parameters of the gRPC request to match.
// It can read first RECV message to match the request, if the method without
// the body didn't match to any rule.
// The request should never be modified, it should be returned as a new one.
type Request struct {
	// URI defines the fully-qualified method name, e.g.
	// "/package.Service/Method".
	URI string

	// IncomingMetadata contains the metadata of the incoming request.
	IncomingMetadata metadata.MD

	// FirstRecv contains the first RECV message of the request.
	FirstRecv []byte
}

// Match contains the result of the matching.
type Match struct {
	Mock *Mock
}

// Mock contains the details of how the handler should reply to the downstream.
type Mock struct {
	Header  metadata.MD
	Trailer metadata.MD
	Body    proto.Message
	Status  *status.Status
}

// Payload contains the details of the payload.
type Payload struct {
	Body proto.Message
}
