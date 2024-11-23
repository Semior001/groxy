// Package grpctest contains the gRPC server to test streaming helpers.
package grpctest

import (
	"testing"
	"google.golang.org/grpc"
	"github.com/stretchr/testify/require"
	"net"
	"errors"
	"context"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
)

// Server implements the ExampleServiceServer over wrapped functions.
type Server struct {
	BiDirectionalFunc func(ExampleService_BiDirectionalServer) error
	ServerStreamFunc  func(*StreamRequest, ExampleService_ServerStreamServer) error
	ClientStreamFunc  func(ExampleService_ClientStreamServer) error
	UnaryFunc         func(context.Context, *StreamRequest) (*StreamResponse, error)

	UnimplementedExampleServiceServer
}

// BiDirectional calls the mocked method.
func (s *Server) BiDirectional(srv ExampleService_BiDirectionalServer) error {
	if s.BiDirectionalFunc != nil {
		return s.BiDirectionalFunc(srv)
	}
	return status.Error(codes.Unimplemented, "didn't expect call on BiDirectional method")
}

// ServerStream calls the mocked method.
func (s *Server) ServerStream(req *StreamRequest, srv ExampleService_ServerStreamServer) error {
	if s.ServerStreamFunc != nil {
		return s.ServerStreamFunc(req, srv)
	}
	return status.Error(codes.Unimplemented, "didn't expect call on ServerStream method")
}

// ClientStream calls the mocked method.
func (s *Server) ClientStream(srv ExampleService_ClientStreamServer) error {
	if s.ClientStreamFunc != nil {
		return s.ClientStreamFunc(srv)
	}
	return status.Error(codes.Unimplemented, "didn't expect call on ClientStream method")
}

// Unary calls the mocked method.
func (s *Server) Unary(ctx context.Context, req *StreamRequest) (*StreamResponse, error) {
	if s.UnaryFunc != nil {
		return s.UnaryFunc(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "didn't expect call on Unary method")
}

// StartServer starts a new TestServiceServer.
func StartServer(t *testing.T, srv *grpc.Server) (addr string) {
	t.Helper()

	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "failed to start listener")

	go func() {
		if err := srv.Serve(l); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("failed to serve: %v", err)
		}
	}()

	addr = l.Addr().String()
	t.Logf("started test server on %s", addr)
	return addr
}
