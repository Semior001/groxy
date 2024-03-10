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
	"golang.org/x/sync/errgroup"
)

var opts struct {
	Addr string `short:"a" long:"addr" env:"ADDR" default:":8080" description:"Address to listen on"`
	File struct {
		Name          string        `long:"name"           env:"NAME"           default:"groxy.yml" description:"Config file name"                  `
		CheckInterval time.Duration `long:"check-interval" env:"CHECK_INTERVAL" default:"3s"        description:"Check interval for the config file"`
		Delay         time.Duration `long:"delay"          env:"DELAY"          default:"500ms"     description:"Delay before applying the changes" `
	} `group:"file" namespace:"file" env-namespace:"FILE"`
	Debug bool `long:"debug" env:"DEBUG" description:"Enable debug mode"`
}

var version = "unknown"

func getVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if ok {
		return bi.Main.Version
	}
	return version
}

func main() {
	_, _ = fmt.Fprintf(os.Stderr, "groxy %s\n", getVersion())

	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}

	setupLog(opts.Debug)

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

func setupLog(debug bool) {
	defer slog.Info("prepared logger", slog.Bool("debug", debug))
	handlerOpts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.Handler(slog.NewJSONHandler(os.Stderr, handlerOpts))

	if debug {
		handlerOpts.Level = slog.LevelDebug
		handlerOpts.AddSource = true
		handlerOpts.ReplaceAttr = func(_ []string, a slog.Attr) slog.Attr {
			// shorten source to just file:line
			if a.Key == slog.SourceKey {
				src := a.Value.Any().(*slog.Source)
				file := src.File[strings.LastIndex(src.File, "/")+1:]
				return slog.String("s", fmt.Sprintf("%s:%d", file, src.Line))
			}
			return a
		}
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	}

	handler = slogx.NewChain(handler,
		slogm.RequestID(),
		slogm.StacktraceOnError(),
		slogm.TrimAttrs(1024), // 1Kb
	)

	slog.SetDefault(slog.New(handler))
}

func run(ctx context.Context) error {
	dsvc := &discovery.Service{Providers: []discovery.Provider{
		&fileprovider.File{
			FileName:      opts.File.Name,
			CheckInterval: opts.File.CheckInterval,
			Delay:         opts.File.Delay,
		},
	}}
	srv := proxy.NewServer(dsvc, proxy.Version(getVersion()))

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
