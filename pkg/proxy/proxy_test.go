package proxy

import (
	"testing"
	"github.com/Semior001/groxy/pkg/proxy/mocks"
	"google.golang.org/grpc/metadata"
	"github.com/Semior001/groxy/pkg/discovery"
	"regexp"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"math/rand"
	"github.com/stretchr/testify/require"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"context"
	"github.com/Semior001/groxy/pkg/grpcx/grpctest"
)

func TestServer_handle(t *testing.T) {
	backendSrv := grpc.NewServer()
	grpctest.RegisterExampleServiceServer(backendSrv, &grpctest.Server{
		BiDirectionalFunc: grpctest.PingPong,
		ServerStreamFunc:  grpctest.Flood,
		ClientStreamFunc:  grpctest.Sum,
		UnaryFunc: func(_ context.Context, req *grpctest.StreamRequest) (*grpctest.StreamResponse, error) {
			if req.Value == "forward" {
				return &grpctest.StreamResponse{Value: "forwarded to the backend"}, nil
			}
			return nil, status.Error(codes.Unimplemented, "unexpected call")
		},
	})
	backendConn, err := grpc.Dial(grpctest.StartServer(t, backendSrv),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	matcher := &mocks.MatcherMock{
		UpstreamsFunc: func() []discovery.Upstream {
			return []discovery.Upstream{discovery.ClientConn{
				ConnName:        "backend",
				ServeReflection: true,
				ClientConn:      backendConn,
			}}
		},
		MatchMetadataFunc: func(string, metadata.MD) discovery.Matches {
			return discovery.Matches{
				{
					Name: "groxy.testdata.ExampleService/Unary (mock with status)",
					Match: discovery.RequestMatcher{
						URI:     regexp.MustCompile("groxy.testdata.ExampleService/Unary"),
						Message: &grpctest.StreamRequest{Value: "error"},
					},
					Mock: &discovery.Mock{
						Status: status.New(codes.Internal, "test"),
					},
				},
				{
					Name: "groxy.testdata.ExampleService/Unary (mock with metadata)",
					Match: discovery.RequestMatcher{
						URI:     regexp.MustCompile("groxy.testdata.ExampleService/Unary"),
						Message: &grpctest.StreamRequest{Value: "metadata"},
					},
					Mock: &discovery.Mock{
						Header: metadata.Pairs("test", "test"),
						Status: status.New(codes.InvalidArgument, "test metadata"),
					},
				},
				{
					Name: "groxy.testdata.ExampleService/Unary (forward to backend)",
					Match: discovery.RequestMatcher{
						URI:     regexp.MustCompile("groxy.testdata.ExampleService/Unary"),
						Message: &grpctest.StreamRequest{Value: "forward"},
					},
					Forward: discovery.ClientConn{
						ConnName:        "backend",
						ServeReflection: true,
						ClientConn:      backendConn,
					},
				},
				{
					Name: "groxy.testdata.ExampleService/Unary (mock with response)",
					Match: discovery.RequestMatcher{
						URI:     regexp.MustCompile("groxy.testdata.ExampleService/Unary"),
						Message: &grpctest.StreamRequest{Value: "response"},
					},
					Mock: &discovery.Mock{
						Body: &grpctest.StreamResponse{Value: "test"},
					},
				},
				{
					Name: "groxy.testdata.ExampleService/BiDirectional (forward to backend)",
					Match: discovery.RequestMatcher{
						URI: regexp.MustCompile("groxy.testdata.ExampleService/BiDirectional"),
					},
					Forward: discovery.ClientConn{
						ConnName:        "backend",
						ServeReflection: true,
						ClientConn:      backendConn,
					},
				},
				{
					Name: "groxy.testdata.ExampleService/ClientStream (forward to backend)",
					Match: discovery.RequestMatcher{
						URI: regexp.MustCompile("groxy.testdata.ExampleService/ClientStream"),
					},
					Forward: discovery.ClientConn{
						ConnName:        "backend",
						ServeReflection: true,
						ClientConn:      backendConn,
					},
				},
				{
					Name: "groxy.testdata.ExampleService/ServerStream (forward to backend)",
					Match: discovery.RequestMatcher{
						URI: regexp.MustCompile("groxy.testdata.ExampleService/ServerStream"),
					},
					Forward: discovery.ClientConn{
						ConnName:        "backend",
						ServeReflection: true,
						ClientConn:      backendConn,
					},
				},
			}
		},
	}
	srv := NewServer(matcher, Version("test"), Debug())
	port := rand.Intn(1000) + 10000

	go func() {
		require.NoError(t, srv.Listen(fmt.Sprintf("localhost:%d", port)))
	}()
	defer srv.Close()

	cc, err := grpc.Dial(fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	cl := grpctest.NewExampleServiceClient(cc)

	t.Run("mock with response", func(t *testing.T) {
		resp, err := cl.Unary(context.Background(), &grpctest.StreamRequest{Value: "response"})
		require.NoError(t, err)

		require.Equal(t, "test", resp.Value)
	})

	t.Run("mock with status", func(t *testing.T) {
		_, err := cl.Unary(context.Background(), &grpctest.StreamRequest{Value: "error"})
		st, ok := status.FromError(err)
		require.True(t, ok)

		require.Equal(t, codes.Internal, st.Code())
		require.Equal(t, "test", st.Message())
	})

	t.Run("mock with metadata", func(t *testing.T) {
		md := &metadata.MD{}
		_, err := cl.Unary(context.Background(), &grpctest.StreamRequest{Value: "metadata"},
			grpc.Header(md))
		st, ok := status.FromError(err)
		require.True(t, ok)

		require.Equal(t, codes.InvalidArgument, st.Code())
		require.Equal(t, "test metadata", st.Message())

		require.Equal(t, "test", (*md)["test"][0])
	})

	t.Run("forward to the backend", func(t *testing.T) {
		t.Run("unary", func(t *testing.T) {
			resp, err := cl.Unary(context.Background(), &grpctest.StreamRequest{Value: "forward"})
			require.NoError(t, err)

			require.Equal(t, "forwarded to the backend", resp.Value)
		})

		t.Run("bidirectional", func(t *testing.T) {
			stream, err := cl.BiDirectional(context.Background())
			require.NoError(t, err)

			for i := 0; i < 5; i++ {
				require.NoError(t, stream.Send(&grpctest.StreamRequest{Value: "ping"}))
				resp, err := stream.Recv()
				require.NoError(t, err)

				require.Equal(t, "pong", resp.Value)
			}

			require.NoError(t, stream.CloseSend())
		})

		t.Run("client stream", func(t *testing.T) {
			//cl := grpctest.NewExampleServiceClient(backendConn)

			stream, err := cl.ClientStream(context.Background())
			require.NoError(t, err)

			for i := 0; i < 5; i++ {
				require.NoError(t, stream.Send(&grpctest.StreamRequest{Value: fmt.Sprint(i)}))
			}

			resp, err := stream.CloseAndRecv()
			require.NoError(t, err)

			require.Equal(t, "10", resp.Value)
		})

		t.Run("server stream", func(t *testing.T) {
			stream, err := cl.ServerStream(context.Background(), &grpctest.StreamRequest{Value: "5"})
			require.NoError(t, err)

			for i := 0; i < 5; i++ {
				resp, err := stream.Recv()
				require.NoError(t, err)

				require.Equal(t, fmt.Sprint(i), resp.Value)
			}
		})
	})
}
