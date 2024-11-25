// Package proxy provides server and codec for proxying gRPC requests.
package proxy

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/Semior001/groxy/pkg/discovery"
	"github.com/Semior001/groxy/pkg/proxy/middleware"
	"github.com/cappuccinotm/slogx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"github.com/Semior001/groxy/pkg/grpcx"
)

//go:generate moq -out mocks/mocks.go --skip-ensure -pkg mocks . Matcher ServerStream

// ServerStream is a gRPC server stream.
type ServerStream grpc.ServerStream

// Matcher matches the request URI and incoming metadata to the
// registered rules.
type Matcher interface {
	MatchMetadata(string, metadata.MD) discovery.Matches
	Upstreams() []discovery.Upstream
}

// Server is a gRPC server.
type Server struct {
	version string

	serverOpts       []grpc.ServerOption
	defaultResponder func(stream grpc.ServerStream, firstRecv []byte) error
	matcher          Matcher

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
		defaultResponder: func(_ grpc.ServerStream, _ []byte) error {
			return status.Error(codes.Internal, "{groxy} didn't match request to any rule")
		},
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

	s.grpc = grpc.NewServer(append(s.serverOpts,
		grpc.UnknownServiceHandler(middleware.Wrap(s.handle,
			middleware.Recoverer(),
			middleware.Maybe(s.signature, middleware.AppInfo("groxy", "Semior001", s.version)),
			middleware.Log(s.debug, "/grpc.reflection."),
			middleware.PassMetadata(),
			middleware.Maybe(s.reflection, middleware.Chain(
				middleware.Reflector{
					Logger:        slog.Default().With(slog.String("subsystem", "reflection")),
					UpstreamsFunc: s.matcher.Upstreams,
				}.Middleware,
			)),
		)),
		grpc.ForceServerCodec(grpcx.RawBytesCodec{}),
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

func (s *Server) handle(_ any, stream grpc.ServerStream) error {
	ctx := stream.Context()

	mtd, ok := grpc.Method(ctx)
	if !ok {
		slog.WarnContext(ctx, "failed to get method from context")
		return s.defaultResponder(stream, nil)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	matches := s.matcher.MatchMetadata(mtd, md)
	if len(matches) == 0 {
		return s.defaultResponder(stream, nil)
	}

	slog.DebugContext(ctx, "found matches", slog.Any("matches", matches))

	var firstRecv []byte

	match := matches[0]
	if matches.NeedsDeeperMatch() {
		if err := stream.RecvMsg(&firstRecv); err != nil {
			slog.WarnContext(ctx, "failed to read the first RECV", slogx.Error(err))
			return s.defaultResponder(stream, nil)
		}

		if match, ok = matches.MatchMessage(firstRecv); !ok {
			return s.defaultResponder(stream, firstRecv)
		}
	}

	slog.DebugContext(ctx, "matched", slog.Any("match", match))

	if match.Forward != nil {
		return s.forward(stream, match.Forward, firstRecv)
	}

	return s.mock(stream, match.Mock)
}

func (s *Server) forward(stream grpc.ServerStream, forward discovery.Upstream, recv []byte) error {
	ctx := stream.Context()
	mtd, _ := grpc.Method(ctx)

	upstream, err := forward.NewStream(ctx, &grpc.StreamDesc{ClientStreams: true, ServerStreams: true}, mtd,
		grpc.ForceCodec(grpcx.RawBytesCodec{}))
	if err != nil {
		return status.Errorf(codes.Internal, "{groxy} failed to create upstream: %v", err)
	}

	var msgs []grpcx.Message
	if recv != nil {
		msgs = []grpcx.Message{{Value: recv, Direction: grpcx.ClientToServer}}
	}

	defer func() {
		if err := upstream.CloseSend(); err != nil {
			slog.WarnContext(ctx, "failed to close the upstream",
				slog.String("upstream_name", forward.Name()),
				slogx.Error(err))
		}
	}()

	switch err = grpcx.Pipe(upstream, stream, msgs...); {
	case errors.Is(err, io.EOF):
		// try to get the error from the upstream
		if err := upstream.RecvMsg(nil); err != nil {
			if st := grpcx.StatusFromError(err); st != nil {
				return st.Err()
			}

			if errors.Is(err, io.EOF) {
				return nil // if there is just EOF then probably everything is fine
			}

			return status.Errorf(codes.Internal, "{groxy} failed to read the EOF from the upstream: %v", err)
		}

		return status.Error(codes.Internal, "{groxy} unexpected EOF from the upstream")
	case err != nil:
		slog.WarnContext(ctx, "failed to pipe",
			slog.String("upstream_name", forward.Name()),
			slogx.Error(err))
		return status.Errorf(codes.Internal, "{groxy} failed to pipe messages to the upstream")
	default:
		return nil
	}
}

func (s *Server) mock(stream grpc.ServerStream, reply *discovery.Mock) error {
	ctx := stream.Context()

	if len(reply.Header) > 0 {
		if err := stream.SetHeader(reply.Header); err != nil {
			slog.WarnContext(ctx, "failed to set header to the client", slogx.Error(err))
		}
	}

	if len(reply.Trailer) > 0 {
		stream.SetTrailer(reply.Trailer)
	}

	switch {
	case reply.Body != nil:
		if err := stream.SendMsg(reply.Body); err != nil {
			return status.Errorf(codes.Internal, "{groxy} failed to send message: %v", err)
		}
	case reply.Status != nil:
		return reply.Status.Err()
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
