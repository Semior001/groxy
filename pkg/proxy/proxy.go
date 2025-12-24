// Package proxy provides server and codec for proxying gRPC requests.
package proxy

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"context"

	"github.com/Semior001/groxy/pkg/discovery"
	"github.com/Semior001/groxy/pkg/grpcx"
	"github.com/Semior001/groxy/pkg/proxy/middleware"
	"github.com/cappuccinotm/slogx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

//go:generate moq -out mocks/mocks.go --skip-ensure -pkg mocks . Matcher ServerStream

// ServerStream is a gRPC server stream.
type ServerStream grpc.ServerStream

// Matcher matches the request URI and incoming metadata to the
// registered rules.
type Matcher interface {
	MatchMetadata(string, metadata.MD) discovery.Matches // returns matches based on the method and metadata.
	Upstreams() []discovery.Upstream                     // returns all upstreams registered in the matcher
}

// Server is a gRPC server.
type Server struct {
	version string

	serverOpts []grpc.ServerOption
	matcher    Matcher

	signature  bool
	reflection bool
	debug      bool
	l          net.Listener
	grpc       *grpc.Server
}

// NewServer creates a new server.
func NewServer(m Matcher, opts ...Option) *Server {
	s := &Server{
		matcher:   m,
		signature: false,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Listen starts the server on the given address.
// Blocking call.
func (s *Server) Listen(addr string) (err error) {
	slog.Info("starting gRPC server", slog.Any("addr", addr))
	defer slog.Warn("gRPC server stopped", slogx.Error(err))

	healthHandler := health.NewServer()
	healthHandler.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	noMatchHandler := func(any, grpc.ServerStream) error {
		return status.Error(codes.Internal, "{groxy} didn't match request to any rule")
	}

	s.grpc = grpc.NewServer(append(s.serverOpts,
		grpc.ForceServerCodec(grpcx.RawBytesCodec{}),
		grpc.UnknownServiceHandler(middleware.Wrap(noMatchHandler,
			middleware.Recoverer("{groxy} panic"),
			middleware.Maybe(s.signature, middleware.AppInfo("groxy", "Semior001", s.version)),
			middleware.Log(s.debug, "/grpc.reflection."),
			middleware.PassMetadata(),
			middleware.Health(healthHandler),
			middleware.Maybe(s.reflection, middleware.Chain(
				middleware.Reflector{
					Logger:        slog.Default().With(slog.String("subsystem", "reflection")),
					UpstreamsFunc: s.matcher.Upstreams,
				}.Middleware,
			)),
			s.matchMiddleware,
			s.mockMiddleware, s.forwardMiddleware,
		)),
	)...)

	if s.l, err = net.Listen("tcp", addr); err != nil {
		return fmt.Errorf("register listener: %w", err)
	}

	if err = s.grpc.Serve(s.l); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

// Close stops the server.
func (s *Server) Close() { s.grpc.GracefulStop() }

type contextKey string

var (
	ctxMatch     = contextKey("match")
	ctxFirstRecv = contextKey("first_recv")
)

func (s *Server) matchMiddleware(next grpc.StreamHandler) grpc.StreamHandler {
	return func(srv any, stream grpc.ServerStream) error {
		ctx := stream.Context()

		mtd, ok := grpc.Method(ctx)
		if !ok {
			slog.WarnContext(ctx, "failed to get method from context")
			return status.Error(codes.Internal, "{groxy} failed to get method from the context")
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		matches := s.matcher.MatchMetadata(mtd, md)
		if len(matches) == 0 {
			return next(srv, stream)
		}

		slog.DebugContext(ctx, "found matches", slog.Any("matches", matches))

		match := matches[0]
		if !matches.NeedsDeeperMatch() {
			slog.DebugContext(ctx, "matched", slog.Any("match", match))
			ctx = context.WithValue(ctx, ctxMatch, match)
			return next(srv, grpcx.StreamWithContext(ctx, stream))
		}

		var firstRecv []byte
		if err := stream.RecvMsg(&firstRecv); err != nil {
			slog.WarnContext(ctx, "failed to read the first RECV", slogx.Error(err))
			return status.Errorf(codes.Internal, "{groxy} failed to read the first RECV: %v", err)
		}

		ctx = context.WithValue(ctx, ctxFirstRecv, firstRecv)
		if match, ok = matches.MatchMessage(ctx, firstRecv); !ok {
			return next(srv, stream)
		}

		slog.DebugContext(ctx, "matched", slog.Any("match", match))
		ctx = context.WithValue(ctx, ctxMatch, match)
		return next(srv, grpcx.StreamWithContext(ctx, stream))
	}
}

func (s *Server) mockMiddleware(next grpc.StreamHandler) grpc.StreamHandler {
	return func(srv any, stream grpc.ServerStream) error {
		ctx := stream.Context()

		match, ok := ctx.Value(ctxMatch).(*discovery.Rule)
		if !ok {
			return next(srv, stream)
		}

		if match.Mock == nil {
			return next(srv, stream)
		}

		if match.Mock.Wait > 0 {
			slog.DebugContext(ctx, "waiting before responding", slog.Any("wait", match.Mock.Wait))
			select {
			case <-ctx.Done():
				slog.WarnContext(ctx, "context done while waiting",
					slog.Any("wait", match.Mock.Wait),
					slogx.Error(ctx.Err()))
				return status.Error(codes.Canceled, "{groxy} context done while waiting")
			case <-time.After(match.Mock.Wait):
			}
		}

		if len(match.Mock.Header) > 0 {
			if err := stream.SetHeader(match.Mock.Header); err != nil {
				slog.WarnContext(ctx, "failed to set header to the client", slogx.Error(err))
			}
		}

		if len(match.Mock.Trailer) > 0 {
			stream.SetTrailer(match.Mock.Trailer)
		}

		switch {
		case match.Mock.Body != nil:
			var data map[string]any

			firstRecv := ctx.Value(ctxFirstRecv)
			if firstRecv != nil && match.Match.Message != nil {
				dm, err := match.Match.Message.DataMap(ctx, firstRecv.([]byte))
				if err != nil {
					slog.WarnContext(ctx, "failed to extract data from the first message", slogx.Error(err))
					return status.Errorf(codes.Internal, "{groxy} failed to extract data from the first message: %v", err)
				}
				data = dm
			}

			msg, err := match.Mock.Body.Generate(ctx, data)
			if err != nil {
				slog.WarnContext(ctx, "failed to generate mock body", slogx.Error(err))
				return status.Errorf(codes.Internal, "{groxy} failed to generate mock body: %v", err)
			}

			if err = stream.SendMsg(msg); err != nil {
				return status.Errorf(codes.Internal, "{groxy} failed to send message: %v", err)
			}
		case match.Mock.Status != nil:
			return match.Mock.Status.Err()
		default:
			return status.Error(codes.Internal, "{groxy} empty mock")
		}

		// dump the rest of the stream
		for {
			if err := stream.RecvMsg(nil); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}

				return status.Errorf(codes.Internal, "{groxy} failed to read the rest of the stream: %v", err)
			}
		}
	}
}

func (s *Server) forwardMiddleware(next grpc.StreamHandler) grpc.StreamHandler {
	return func(_ any, stream grpc.ServerStream) error {
		ctx := stream.Context()

		match, ok := ctx.Value(ctxMatch).(*discovery.Rule)
		if !ok || match.Forward == nil {
			return next(nil, stream)
		}

		ctx = plantHeader(ctx, match.Forward.Header)

		mtd, _ := grpc.Method(ctx)
		desc := &grpc.StreamDesc{ClientStreams: true, ServerStreams: true}

		upstreamHeader, upstreamTrailer := metadata.New(nil), metadata.New(nil)

		if match.Forward.Rewrite != "" {
			mtd = match.Match.URI.ReplaceAllString(mtd, match.Forward.Rewrite)
		}

		upstream, err := match.Forward.Upstream.NewStream(ctx, desc, mtd,
			grpc.ForceCodec(grpcx.RawBytesCodec{}),
			grpc.Header(&upstreamHeader),
			grpc.Trailer(&upstreamTrailer))
		if err != nil {
			return status.Errorf(codes.Internal, "{groxy} failed to create upstream: %v", err)
		}

		if firstRecv, _ := ctx.Value(ctxFirstRecv).([]byte); firstRecv != nil {
			if err = upstream.SendMsg(firstRecv); err != nil {
				return status.Errorf(codes.Internal,
					"{groxy} failed to send the first message to the upstream: %v", err)
			}
		}

		defer func() {
			stream.SetTrailer(metadata.Join(upstreamHeader, upstreamTrailer))

			if err = upstream.CloseSend(); err != nil {
				slog.WarnContext(ctx, "failed to close the upstream",
					slog.String("upstream_name", match.Forward.Upstream.Name()),
					slogx.Error(err))
			}
		}()

		if err = grpcx.Pipe(upstream, stream); err != nil {
			if errors.Is(err, io.EOF) {
				return eofStatus(upstream)
			}
			if st := grpcx.StatusFromError(err); st != nil {
				return st.Err()
			}
			slog.WarnContext(ctx, "failed to pipe",
				slog.String("upstream_name", match.Forward.Upstream.Name()),
				slogx.Error(err))
			return status.Errorf(codes.Internal, "{groxy} failed to pipe messages to the upstream")
		}

		return nil
	}
}

func eofStatus(upstream grpc.ClientStream) (err error) {
	if err = upstream.RecvMsg(nil); err == nil {
		return status.Error(codes.Internal, "{groxy} unexpected EOF from the upstream")
	}
	if st := grpcx.StatusFromError(err); st != nil {
		return st.Err()
	}
	if !errors.Is(err, io.EOF) {
		return status.Errorf(codes.Internal, "{groxy} failed to read the EOF from the upstream: %v", err)
	}
	return nil // if there is just EOF then probably everything is fine
}

func plantHeader(ctx context.Context, header metadata.MD) context.Context {
	outMD, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		outMD = metadata.New(nil)
	}

	for k, v := range header {
		if _, ok = outMD[k]; !ok {
			outMD[k] = v
		}
	}

	return metadata.NewOutgoingContext(ctx, outMD)
}
