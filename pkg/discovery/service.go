package discovery

import (
	"context"
	"log/slog"
	"sort"
	"sync"

	"github.com/cappuccinotm/slogx"
	"github.com/samber/lo"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"bytes"
)

//go:generate moq -out mock_provider.go -fmt goimports . Provider

// Service provides routing rules for the Service.
type Service struct {
	Providers []Provider

	upstreams []Upstream
	rules     []*Rule
	mu        sync.RWMutex
}

// Run starts a blocking loop that updates the routing rules
// on the signals, received from providers.
func (s *Service) Run(ctx context.Context) (err error) {
	slog.InfoContext(ctx, "starting discovery service")
	defer slog.WarnContext(ctx, "discovery service stopped", slogx.Error(err))

	chs := make([]<-chan string, 0, len(s.Providers))
	for _, p := range s.Providers {
		chs = append(chs, p.Events(ctx))
	}

	ch := lo.FanIn(0, chs...)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-ch:
			slog.DebugContext(ctx, "new event update received", slog.String("event", ev))

			rules, upstreams := s.mergeStates(ctx)
			s.mu.Lock()
			s.rules = rules
			s.closeUpstreams(ctx)
			s.upstreams = upstreams
			s.mu.Unlock()

			slog.InfoContext(ctx, "updated routing rules",
				slog.Int("rules", len(rules)),
				slog.Int("upstreams", len(upstreams)))
		}
	}
}

func (s *Service) mergeStates(ctx context.Context) ([]*Rule, []Upstream) {
	var rules []*Rule
	var upstreams []Upstream
	for _, p := range s.Providers {
		st, err := p.State(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get state of the provider",
				slog.String("provider", p.Name()),
				slogx.Error(err))
			continue
		}
		rules = append(rules, st.Rules...)
		upstreams = append(upstreams, st.Upstreams...)
	}

	// sort rules by the following order:
	// 1. rules with more metadata to match
	// 2. rules with request bodies to match
	// 3. rest of the rules
	sort.Slice(rules, func(i, j int) bool {
		ri, rj := rules[i].Match, rules[j].Match
		if len(ri.IncomingMetadata) != len(rj.IncomingMetadata) {
			return len(ri.IncomingMetadata) > len(rj.IncomingMetadata)
		}
		return ri.Message != nil && rj.Message == nil
	})

	return rules, upstreams
}

// MatchMetadata matches the given gRPC request to an upstream connection.
func (s *Service) MatchMetadata(uri string, md metadata.MD) Matches {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches Matches
	for _, r := range s.rules {
		if r.Match.Matches(uri, md) {
			matches = append(matches, r)
		}
	}

	return matches
}

// Upstreams returns the list of upstream connections.
func (s *Service) Upstreams() []Upstream {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.upstreams
}

func (s *Service) closeUpstreams(ctx context.Context) {
	for _, u := range s.upstreams {
		slog.DebugContext(ctx, "closing upstream connection", slog.String("upstream", u.Name()))

		if err := u.Close(); err != nil {
			slog.WarnContext(ctx, "failed to close upstream connection",
				slog.String("upstream", u.Name()),
				slogx.Error(err))
		}
	}
}

// Matches is a set of matches.
type Matches []*Rule

// NeedsDeeperMatch returns true if first RECV message
// is needed to match the request.
func (m Matches) NeedsDeeperMatch() bool {
	for _, rule := range m {
		if rule.Match.Message != nil {
			return true
		}
	}
	return false
}

// MatchMessage matches the given gRPC request to a rule.
// It returns the first match and true if the request is matched.
func (m Matches) MatchMessage(bts []byte) (*Rule, bool) {
	for _, rule := range m {
		// matches are sorted by presence of the message,
		// if any previous rule hasn't matched, then we consider
		// a rule first non-messaged rule
		if rule.Match.Message == nil {
			return rule, true
		}

		// we consider messages equal if their wire-encoded bytes are equal
		expectedBts, err := proto.Marshal(rule.Match.Message)
		if err != nil {
			continue
		}

		if bytes.Equal(bts, expectedBts) {
			return rule, true
		}
	}

	return nil, false
}
