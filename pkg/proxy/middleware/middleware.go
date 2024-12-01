package middleware

import (
	"log/slog"
	"net"

	"github.com/cappuccinotm/slogx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"github.com/Semior001/groxy/pkg/grpcx"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// Middleware is a function that intercepts the execution of a gRPC handler.
type Middleware func(grpc.StreamHandler) grpc.StreamHandler

// Wrap is a chain of middlewares.
func Wrap(base grpc.StreamHandler, mws ...Middleware) grpc.StreamHandler {
	for i := len(mws) - 1; i >= 0; i-- {
		base = mws[i](base)
	}
	return base
}

// Chain chains the middlewares.
func Chain(mws ...Middleware) Middleware {
	return func(next grpc.StreamHandler) grpc.StreamHandler {
		return Wrap(next, mws...)
	}
}

// AppInfo adds the app info to the header metadata.
func AppInfo(app, author, version string) Middleware {
	return func(next grpc.StreamHandler) grpc.StreamHandler {
		return func(srv any, stream grpc.ServerStream) error {
			md := metadata.Pairs(
				"app", app,
				"author", author,
				"version", version,
			)

			if err := stream.SetHeader(md); err != nil {
				slog.WarnContext(stream.Context(), "failed to send app info", slogx.Error(err))
			}

			return next(srv, stream)
		}
	}
}

// PassMetadata passes the incoming metadata into outgoing metadata.
func PassMetadata() Middleware {
	return func(next grpc.StreamHandler) grpc.StreamHandler {
		return func(srv any, stream grpc.ServerStream) error {
			ctx := stream.Context()
			inMD, _ := metadata.FromIncomingContext(ctx)
			outMD, _ := metadata.FromOutgoingContext(ctx)
			outMD = metadata.Join(outMD, inMD)
			ctx = metadata.NewOutgoingContext(ctx, outMD)
			return next(srv, grpcx.StreamWithContext(ctx, stream))
		}
	}
}

// Health serves the health check requests.
func Health(h *health.Server) Middleware {
	return func(next grpc.StreamHandler) grpc.StreamHandler {
		return func(srv any, stream grpc.ServerStream) error {
			ctx := stream.Context()
			mtd, ok := grpc.Method(ctx)
			if !ok {
				return next(srv, stream)
			}

			switch mtd {
			case "/grpc.health.v1.Health/Check":
				req := &healthpb.HealthCheckRequest{}
				if err := stream.RecvMsg(req); err != nil {
					return status.Error(codes.InvalidArgument, err.Error())
				}

				resp, err := h.Check(ctx, req)
				if err != nil {
					return err
				}

				return stream.SendMsg(resp)
			case "/grpc.health.v1.Health/Watch":
				// a dumb kludge to not write own WatchServer
				return healthpb.Health_ServiceDesc.Streams[0].Handler(h, stream)
			default:
				return next(srv, stream)
			}
		}
	}
}

// Recoverer is a middleware that recovers from panics, logs the panic and returns a gRPC error if possible.
func Recoverer(responseMessage string) Middleware {
	return func(next grpc.StreamHandler) grpc.StreamHandler {
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
						slog.Any("panic", rvr),
						slogx.Error(err))

					err = status.Error(codes.ResourceExhausted, responseMessage)
				}
			}()
			return next(srv, stream)
		}
	}
}

// Maybe is a middleware that conditionally applies the given middleware.
func Maybe(apply bool, mw Middleware) Middleware {
	if !apply {
		return func(next grpc.StreamHandler) grpc.StreamHandler { return next }
	}
	return mw
}
