// Package middleware contains middlewares for gRPC unknown handlers.
package middleware

import (
	"log/slog"
	"net"

	"github.com/cappuccinotm/slogx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// Middleware is a function that intercepts the execution of a gRPC handler.
type Middleware func(grpc.StreamHandler) grpc.StreamHandler

// Chain is a chain of middlewares.
func Chain(base grpc.StreamHandler, mws ...Middleware) grpc.StreamHandler {
	for i := len(mws) - 1; i >= 0; i-- {
		base = mws[i](base)
	}
	return base
}

// AppInfo adds the app info to the header metadata.
func AppInfo(app, author, version string) Middleware {
	return func(next grpc.StreamHandler) grpc.StreamHandler {
		return func(srv any, stream grpc.ServerStream) error {
			ctx := stream.Context()

			md := metadata.Pairs(
				"app", app,
				"author", author,
				"version", version,
			)

			if err := stream.SetHeader(md); err != nil {
				slog.WarnContext(ctx, "failed to send app info", slogx.Error(err))
			}

			return next(srv, stream)
		}
	}
}

// Recoverer is a middleware that recovers from panics, logs the panic and returns a gRPC error if possible.
func Recoverer(next grpc.StreamHandler) grpc.StreamHandler {
	return func(srv any, stream grpc.ServerStream) (err error) {
		defer func() {
			if rvr := recover(); rvr != nil {
				ctx := stream.Context()

				mtd, ok := grpc.Method(ctx)
				if !ok {
					mtd = "unknown"
				}

				pi, ok := peer.FromContext(ctx)
				if !ok {
					pi = &peer.Peer{Addr: &net.IPAddr{IP: net.IPv4zero}}
				}

				slog.ErrorContext(ctx, "stream panic",
					slog.String("method", mtd),
					slog.String("remote", pi.Addr.String()),
					slog.Any("panic", rvr))
			}
		}()
		return next(srv, stream)
	}
}
