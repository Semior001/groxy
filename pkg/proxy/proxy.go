// Package proxy provides server and codec for proxying gRPC requests.
package proxy

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/Semior001/groxy/pkg/proxy/discovery"
	"github.com/Semior001/groxy/pkg/proxy/middleware"
	"github.com/cappuccinotm/slogx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Server is a gRPC server.
type Server struct {
	version string

	serverOpts       []grpc.ServerOption
	defaultResponder func(stream grpc.ServerStream, firstRecv []byte) error
	matcher          *discovery.Service

	l    net.Listener
	grpc *grpc.Server
}

// NewServer creates a new server.
func NewServer(m *discovery.Service, opts ...Option) *Server {
	s := &Server{
		matcher: m,
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
		grpc.UnknownServiceHandler(middleware.Chain(s.handle,
			middleware.Recoverer,
			middleware.AppInfo(s.version, "Semior001", "groxy"),
			middleware.Log,
		)),
		grpc.ForceServerCodec(RawBytesCodec{}),
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

	req := discovery.Request{URI: mtd, IncomingMetadata: md}

	matches, ok := s.matcher.Match(req)
	if !ok {
		if err := stream.RecvMsg(&req.FirstRecv); err != nil {
			slog.WarnContext(ctx, "failed to read the first RECV", slogx.Error(err))
			return s.defaultResponder(stream, nil)
		}

		if matches, ok = s.matcher.Match(req); !ok {
			return s.defaultResponder(stream, req.FirstRecv)
		}
	}

	return s.mock(stream, matches.Mock)
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
