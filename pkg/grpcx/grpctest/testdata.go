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
	"sync"
	"io"
	"fmt"
	"strconv"
)

// Server implements the ExampleServiceServer over wrapped functions.
type Server struct {
	BiDirectionalFunc func(ExampleService_BiDirectionalServer) error
	ServerStreamFunc  func(*StreamRequest, ExampleService_ServerStreamServer) error
	ClientStreamFunc  func(ExampleService_ClientStreamServer) error
	UnaryFunc         func(context.Context, *StreamRequest) (*StreamResponse, error)

	mu     sync.Mutex
	counts Counts
	UnimplementedExampleServiceServer
}

// Counts returns the number of calls to each method.
func (s *Server) Counts() Counts {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts
}

// BiDirectional calls the mocked method.
func (s *Server) BiDirectional(srv ExampleService_BiDirectionalServer) error {
	s.mu.Lock()
	s.counts.BiDirectional++
	s.mu.Unlock()

	if s.BiDirectionalFunc != nil {
		return s.BiDirectionalFunc(srv)
	}
	return status.Error(codes.Unimplemented, "didn't expect call on BiDirectional method")
}

// ServerStream calls the mocked method.
func (s *Server) ServerStream(req *StreamRequest, srv ExampleService_ServerStreamServer) error {
	s.mu.Lock()
	s.counts.ServerStream++
	s.mu.Unlock()

	if s.ServerStreamFunc != nil {
		return s.ServerStreamFunc(req, srv)
	}
	return status.Error(codes.Unimplemented, "didn't expect call on ServerStream method")
}

// ClientStream calls the mocked method.
func (s *Server) ClientStream(srv ExampleService_ClientStreamServer) error {
	s.mu.Lock()
	s.counts.ClientStream++
	s.mu.Unlock()

	if s.ClientStreamFunc != nil {
		return s.ClientStreamFunc(srv)
	}
	return status.Error(codes.Unimplemented, "didn't expect call on ClientStream method")
}

// Unary calls the mocked method.
func (s *Server) Unary(ctx context.Context, req *StreamRequest) (*StreamResponse, error) {
	s.mu.Lock()
	s.counts.Unary++
	s.mu.Unlock()

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

// Counts contains the number of calls to each method.
type Counts struct {
	BiDirectional int
	ServerStream  int
	ClientStream  int
	Unary         int
}

// Echo is a helper function to test bi-directional streaming.
// It responds with exactly the same message it received.
func Echo(srv ExampleService_BiDirectionalServer) error {
	for {
		req, err := srv.Recv()
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive message: %v", err)
		}

		if err = srv.Send(&StreamResponse{Value: req.Value}); err != nil {
			return status.Errorf(codes.Internal, "failed to send message: %v", err)
		}
	}
}

// PingPong is a helper function to test bi-directional streaming.
// It responds with "pong" to every "ping" message.
func PingPong(srv ExampleService_BiDirectionalServer) error {
	for {
		req, err := srv.Recv()
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive message: %v", err)
		}

		if req.Value != "ping" {
			return status.Errorf(codes.InvalidArgument, "expected 'ping', got %q", req.Value)
		}

		if err = srv.Send(&StreamResponse{Value: "pong"}); err != nil {
			return status.Errorf(codes.Internal, "failed to send message: %v", err)
		}
	}
}

// Flood is a helper function to test server-side streaming.
// It sends the number of messages that have been received from the client.
func Flood(req *StreamRequest, srv ExampleService_ServerStreamServer) error {
	n, err := strconv.Atoi(req.Value)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to parse value %q: %v", req.Value, err)
	}

	for i := 0; i < n; i++ {
		select {
		case <-srv.Context().Done():
			return nil
		default:
		}
		if err := srv.Send(&StreamResponse{Value: fmt.Sprintf("%d", i)}); err != nil {
			return status.Errorf(codes.Internal, "failed to send message: %v", err)
		}
	}

	return nil
}

// Sum is a helper function to test client-side streaming.
// It sums all the values received from the client and returns the result.
func Sum(srv ExampleService_ClientStreamServer) error {
	var sum int
	for {
		req, err := srv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return srv.SendAndClose(&StreamResponse{Value: fmt.Sprintf("%d", sum)})
			}
			return status.Errorf(codes.Internal, "failed to receive message: %v", err)
		}

		n, err := strconv.Atoi(req.Value)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "failed to parse value %q: %v", req.Value, err)
		}

		sum += n
	}
}
