// Package main is an application entrypoint.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/Semior001/groxy/pkg/discovery"
	"github.com/Semior001/groxy/pkg/discovery/fileprovider"
	"github.com/Semior001/groxy/pkg/proxy"
	"github.com/cappuccinotm/slogx"
	"github.com/cappuccinotm/slogx/slogm"
	"github.com/jessevdk/go-flags"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"golang.org/x/sync/errgroup"
)

var opts struct {
	Addr string `short:"a" long:"addr" env:"ADDR" default:":8080" description:"Address to listen on"`
	File struct {
		Name          string        `long:"name"           env:"NAME"           default:"groxy.yml" description:"Config file name"                  `
		CheckInterval time.Duration `long:"check-interval" env:"CHECK_INTERVAL" default:"3s"        description:"Check interval for the config file"`
		Delay         time.Duration `long:"delay"          env:"DELAY"          default:"500ms"     description:"Delay before applying the changes" `
	} `group:"file" namespace:"file" env-namespace:"FILE"`
	Signature  bool `long:"signature"     env:"SIGNATURE"        description:"Enable gRoxy signature headers"`
	Reflection bool `long:"reflection"    env:"REFLECTION"       description:"Enable gRPC reflection merger"`
	JSON       bool `long:"json"          env:"JSON"             description:"Enable JSON logging"`
	Debug      bool `long:"debug"         env:"DEBUG"            description:"Enable debug mode"`
}

var version = "unknown"

func getVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && version == "unknown" {
		return bi.Main.Version
	}
	return version
}

func main() {
	_, _ = fmt.Fprintf(os.Stderr, "groxy %s\n", getVersion())

	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}

	setupLog(opts.Debug, opts.JSON)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { // catch signal and invoke graceful termination
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		sig := <-stop
		slog.Warn("caught signal", slog.Any("signal", sig))
		cancel()
	}()

	if err := run(ctx); err != nil {
		slog.Error("failed to start groxy", slogx.Error(err))
	}
}

func run(ctx context.Context) error {
	dsvc := &discovery.Service{Providers: []discovery.Provider{
		&fileprovider.File{
			FileName:      opts.File.Name,
			CheckInterval: opts.File.CheckInterval,
			Delay:         opts.File.Delay,
		},
	}}

	proxyOpts := []proxy.Option{proxy.Version(getVersion())}
	if opts.Debug {
		proxyOpts = append(proxyOpts, proxy.Debug())
	}
	if opts.Reflection {
		slog.Info("gRPC reflection merger enabled")
		proxyOpts = append(proxyOpts, proxy.WithReflection())
	}
	if opts.Signature {
		proxyOpts = append(proxyOpts, proxy.WithSignature())
	}

	srv := proxy.NewServer(dsvc, proxyOpts...)

	ewg, ctx := errgroup.WithContext(ctx)
	ewg.Go(func() error {
		if err := dsvc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("discovery service: %w", err)
		}
		return nil
	})
	ewg.Go(func() error {
		if err := srv.Listen(opts.Addr); err != nil {
			return fmt.Errorf("proxy server: %w", err)
		}
		return nil
	})
	ewg.Go(func() error {
		<-ctx.Done()
		srv.Close()
		return nil
	})

	if err := ewg.Wait(); err != nil {
		return err
	}

	return nil
}

func setupLog(dbg, json bool) {
	defer slog.Info("prepared logger", slog.Bool("debug", dbg), slog.Bool("json", json))

	tintOpts := func(opts *slog.HandlerOptions, timeFormat string) *tint.Options {
		return &tint.Options{
			AddSource:   opts.AddSource,
			Level:       opts.Level,
			ReplaceAttr: opts.ReplaceAttr,
			TimeFormat:  timeFormat,
			NoColor:     !isatty.IsTerminal(os.Stderr.Fd()),
		}
	}

	timeFormat := time.DateTime
	handlerOpts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if dbg {
		timeFormat = time.RFC3339Nano
		handlerOpts.Level = slog.LevelDebug
		handlerOpts.AddSource = true
		handlerOpts.ReplaceAttr = func(_ []string, a slog.Attr) slog.Attr {
			if a.Key != slog.SourceKey {
				return a
			}
			// shorten source to just file:line and trim or extend to 15 characters
			src := a.Value.Any().(*slog.Source)
			file := src.File[strings.LastIndex(src.File, "/")+1:]

			s := fmt.Sprintf("%s:%d", file, src.Line)
			if json {
				return slog.String("s", s)
			}

			switch {
			case len(s) > 15:
				return slog.String("s", s[:15])
			case len(s) < 15:
				return slog.String("s", s+strings.Repeat(" ", 15-len(s)))
			default:
				return slog.String("s", s)
			}
		}
	}

	var handler slog.Handler
	if json {
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	} else {
		handler = tint.NewHandler(os.Stderr, tintOpts(handlerOpts, timeFormat))
	}

	handler = slogx.NewChain(handler,
		slogm.RequestID(),
		slogm.StacktraceOnError(),
		slogm.TrimAttrs(1024), // 1Kb
	)

	slog.SetDefault(slog.New(handler))
}
