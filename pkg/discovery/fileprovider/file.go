// Package fileprovider provides a file-based discovery provider.
package fileprovider

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"time"

	"github.com/Semior001/groxy/pkg/discovery"
	"github.com/Semior001/groxy/pkg/protodef"
	"github.com/cappuccinotm/slogx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials"
	"crypto/tls"
	"sort"
	"github.com/Semior001/groxy/pkg/grpcx"
)

// File discovers the changes in routing rules from a file.
type File struct {
	FileName      string
	CheckInterval time.Duration
	Delay         time.Duration
}

// Name returns the name of the provider.
func (d *File) Name() string {
	return fmt.Sprintf("file:%s", d.FileName)
}

// Events checks whether the file has been changed.
func (d *File) Events(ctx context.Context) <-chan string {
	res := make(chan string)

	trySubmit := func(ch chan string) bool {
		select {
		case ch <- d.Name():
			return true
		default:
			return false
		}
	}

	go func() {
		ticker := time.NewTicker(d.CheckInterval)
		defer close(res)
		defer ticker.Stop()

		var lastModif, modif time.Time
		var ok bool

		if modif, ok = d.getModifTime(ctx); ok { // parse for the first time
			res <- d.Name()
			lastModif = modif
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if modif, ok = d.getModifTime(ctx); !ok {
					continue
				}

				// don't react on modification right away
				if modif == lastModif || modif.Sub(lastModif) < d.Delay {
					continue
				}

				slog.DebugContext(ctx, "file changed",
					slog.String("file", d.FileName),
					slog.String("last_modified", lastModif.Format(time.RFC3339Nano)),
					slog.String("current_modified", modif.Format(time.RFC3339Nano)))

				if trySubmit(res) {
					lastModif = modif
				}
			}
		}
	}()

	return res
}

// State parses the file and returns the current state of the provider.
func (d *File) State(ctx context.Context) (*discovery.State, error) {
	f, err := os.Open(d.FileName)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode file: %w", err)
	}

	if cfg.Version != "1" {
		return nil, fmt.Errorf("unsupported version: %s", cfg.Version)
	}

	upstreams, err := d.upstreams(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("get upstreams: %w", err)
	}

	rules, err := d.rules(cfg, upstreams)
	if err != nil {
		return nil, fmt.Errorf("get rules: %w", err)
	}

	return &discovery.State{
		Name:      d.Name(),
		Rules:     rules,
		Upstreams: upstreams,
	}, nil
}

// Rules parses the file and returns the routing rules from it.
func (d *File) rules(cfg Config, upstreams []discovery.Upstream) ([]*discovery.Rule, error) {
	rules := make([]*discovery.Rule, 0, len(cfg.Rules)+1)
	for idx, r := range cfg.Rules {
		rule, err := parseRule(r, upstreams)
		if err != nil {
			return nil, fmt.Errorf("parse rule #%d: %w", idx, err)
		}

		rules = append(rules, &rule)
	}

	if cfg.NotMatched != nil {
		mock, err := parseRespond(cfg.NotMatched)
		if err != nil {
			return nil, fmt.Errorf("parse respond: %w", err)
		}
		rule := discovery.Rule{
			Name:  "not matched",
			Match: discovery.RequestMatcher{URI: regexp.MustCompile(".*")},
			Mock:  mock,
		}
		rules = append(rules, &rule)
	}

	return rules, nil
}

func (d *File) upstreams(ctx context.Context, cfg Config) ([]discovery.Upstream, error) {
	res := make([]discovery.Upstream, 0, len(cfg.Upstreams))
	for name, u := range cfg.Upstreams {
		cred := insecure.NewCredentials()
		if u.TLS {
			cred = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
		}

		if u.Addr == "" {
			return nil, fmt.Errorf("empty address in upstream %q", name)
		}

		slog.DebugContext(ctx, "dialing upstream",
			slog.String("upstream", name),
			slog.String("address", u.Addr),
			slog.Bool("tls", u.TLS))

		cc, err := grpc.DialContext(ctx, u.Addr,
			grpc.WithTransportCredentials(cred),
			grpc.WithStreamInterceptor(grpcx.ClientLogInterceptor(slog.Default())),
		)
		if err != nil {
			return nil, fmt.Errorf("dial upstream %q: %w", name, err)
		}

		res = append(res, discovery.ClientConn{
			ConnName:        name,
			ServeReflection: u.ServeReflection,
			ClientConn:      cc,
		})
	}

	sort.Slice(res, func(i, j int) bool { return res[i].Name() < res[j].Name() })

	return res, nil
}

func (d *File) getModifTime(ctx context.Context) (modif time.Time, ok bool) {
	fi, err := os.Stat(d.FileName)
	if err != nil {
		slog.WarnContext(ctx, "failed to read file",
			slog.String("file", d.FileName),
			slogx.Error(err))
		return time.Time{}, false
	}

	if fi.IsDir() {
		slog.WarnContext(ctx, "expected file, but found a directory",
			slog.String("file", d.FileName))
		return time.Time{}, false
	}

	return fi.ModTime(), true
}

func parseRule(r Rule, upstreams []discovery.Upstream) (result discovery.Rule, err error) {
	if r.Match.URI == "" {
		return discovery.Rule{}, fmt.Errorf("empty URI in rule")
	}

	result.Name = r.Match.URI
	if result.Match.URI, err = regexp.Compile(r.Match.URI); err != nil {
		return discovery.Rule{}, fmt.Errorf("compile URI regexp: %w", err)
	}

	result.Match.IncomingMetadata = metadata.New(r.Match.Header)

	if r.Match.Body != nil {
		if result.Match.Message, err = protodef.BuildMessage(*r.Match.Body); err != nil {
			return discovery.Rule{}, fmt.Errorf("build request matcher message: %w", err)
		}
	}

	if r.Forward != nil {
		result.Forward = &discovery.Forward{Header: metadata.New(r.Forward.Header)}
		for _, up := range upstreams {
			if up.Name() == r.Forward.Upstream {
				result.Forward.Upstream = up
				break
			}
		}
		if result.Forward.Upstream == nil {
			return discovery.Rule{}, fmt.Errorf("upstream %q not found", r.Forward.Upstream)
		}
	}

	if result.Mock, err = parseRespond(r.Respond); err != nil {
		return discovery.Rule{}, fmt.Errorf("parse respond: %w", err)
	}

	switch {
	case result.Mock != nil && result.Forward != nil:
		return discovery.Rule{}, fmt.Errorf("can't set both mock and forward in rule")
	case result.Mock == nil && result.Forward == nil:
		return discovery.Rule{}, fmt.Errorf("empty rule")
	}

	return result, nil
}

func parseRespond(r *Respond) (result *discovery.Mock, err error) {
	if r == nil {
		return nil, nil
	}

	result = &discovery.Mock{}

	if r.Metadata != nil {
		result.Header = metadata.New(r.Metadata.Header)
		result.Trailer = metadata.New(r.Metadata.Trailer)
	}

	switch {
	case r.Status != nil && r.Body != nil:
		return nil, fmt.Errorf("can't set both status and body in rule")
	case r.Status != nil:
		var code codes.Code
		if err = code.UnmarshalJSON([]byte(fmt.Sprintf("%q", r.Status.Code))); err != nil {
			return nil, fmt.Errorf("unmarshal status code: %w", err)
		}
		result.Status = status.New(code, r.Status.Message)
	case r.Body != nil:
		if result.Body, err = protodef.BuildMessage(*r.Body); err != nil {
			return nil, fmt.Errorf("build respond message: %w", err)
		}
	default:
		return nil, fmt.Errorf("empty response in rule")
	}

	return result, nil
}
