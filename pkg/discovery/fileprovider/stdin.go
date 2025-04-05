package fileprovider

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/Semior001/groxy/pkg/discovery"
	"gopkg.in/yaml.v3"
)

// Stdin discovers routing rules from standard input.
type Stdin struct {
	// Reader is the source of configuration data.
	// Defaults to os.Stdin if not specified.
	Reader io.Reader
}

// Name returns the name of the provider.
func (s *Stdin) Name() string {
	return "stdin"
}

// Events sends a single event when the provider is created.
// Since stdin can only be read once, this provider will only emit one event.
func (s *Stdin) Events(ctx context.Context) <-chan string {
	res := make(chan string, 1)
	
	// Send an event immediately, as we'll read from stdin only once
	res <- s.Name()
	
	// We don't need to monitor stdin for changes
	go func() {
		<-ctx.Done()
		close(res)
	}()
	
	return res
}

// State parses stdin and returns the current state of the provider.
func (s *Stdin) State(ctx context.Context) (*discovery.State, error) {
	reader := s.Reader
	if reader == nil {
		reader = os.Stdin
	}

	var cfg Config
	if err := yaml.NewDecoder(reader).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode stdin: %w", err)
	}

	if cfg.Version != "1" {
		return nil, fmt.Errorf("unsupported version: %s", cfg.Version)
	}

	// Reuse the existing implementation to process the config
	slog.DebugContext(ctx, "parsed configuration from stdin")

	file := &File{} // Create a temporary File provider without file watching capabilities
	
	upstreams, err := file.upstreams(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("get upstreams: %w", err)
	}

	rules, err := file.rules(cfg, upstreams)
	if err != nil {
		return nil, fmt.Errorf("get rules: %w", err)
	}

	return &discovery.State{
		Name:      s.Name(),
		Rules:     rules,
		Upstreams: upstreams,
	}, nil
}