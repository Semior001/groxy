package discovery

import (
	"context"
	"log/slog"
	"sync"

	"github.com/cappuccinotm/slogx"
	"github.com/samber/lo"
)

// Service provides routing rules for the Service.
type Service struct {
	Providers []Provider

	rules []Rule
	mu    sync.RWMutex
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

			rules := s.mergeRules(ctx)
			s.mu.Lock()
			s.rules = rules
			s.mu.Unlock()
		}
	}
}

func (s *Service) mergeRules(ctx context.Context) []Rule {
	rules := make([]Rule, 0)
	for _, p := range s.Providers {
		rs, err := p.Rules(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get rules",
				slog.String("provider", p.Name()),
				slogx.Error(err))
			continue
		}
		rules = append(rules, rs...)
	}
	return rules
}

// Match matches the given gRPC request to an upstream connection.
func (s *Service) Match(req Request) (Match, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.rules {
		if r.Match.Matches(&req) {
			return Match{Mock: r.Mock}, true
		}
	}

	return Match{}, false
}
