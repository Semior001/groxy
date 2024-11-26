//nolint:staticcheck // this file contains references to deprecated reflection API
package middleware

import (
	"testing"
	"google.golang.org/grpc"
	"github.com/Semior001/groxy/pkg/grpcx/grpctest"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials/insecure"
	"github.com/Semior001/groxy/pkg/discovery"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	rapi1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	rapi1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"context"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/proto"
	"github.com/Semior001/groxy/pkg/grpcx"
	"github.com/stretchr/testify/assert"
)

func TestReflector_MiddlewareUpstreamError(t *testing.T) {
	backend := grpc.NewServer(grpc.StreamInterceptor(func(
		any,
		grpc.ServerStream,
		*grpc.StreamServerInfo,
		grpc.StreamHandler,
	) error {
		return status.Error(codes.PermissionDenied, "access denied")
	}))
	reflection.Register(backend)
	backendAddr := grpctest.StartServer(t, backend)
	t.Cleanup(backend.GracefulStop)

	upstreamConn, err := grpc.Dial(backendAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, upstreamConn.Close()) })

	errHandler := func(any, grpc.ServerStream) error {
		return status.Error(codes.Internal, "should not be called")
	}

	mw := Reflector{
		UpstreamsFunc: func() []discovery.Upstream {
			return []discovery.Upstream{
				discovery.ClientConn{
					ConnName:        "backend",
					ServeReflection: true,
					ClientConn:      upstreamConn,
				},
			}
		},
	}.Middleware(errHandler)

	srv := grpc.NewServer(grpc.UnknownServiceHandler(func(srv any, stream grpc.ServerStream) error {
		return mw(srv, stream)
	}))
	srvAddr := grpctest.StartServer(t, srv)
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.Dial(srvAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	cl1 := rapi1.NewServerReflectionClient(conn)

	stream, err := cl1.ServerReflectionInfo(context.Background())
	require.NoError(t, err)

	err = stream.Send(&rapi1.ServerReflectionRequest{
		MessageRequest: &rapi1.ServerReflectionRequest_ListServices{},
	})
	require.NoError(t, err)

	recv, err := stream.Recv()
	require.Nil(t, recv)
	st := grpcx.StatusFromError(err)
	require.NotNil(t, st)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Equal(t, "{groxy} received from one of upstreams: access denied", st.Message())
}

func TestReflector_Middleware(t *testing.T) {
	s1 := grpc.NewServer()
	grpctest.RegisterExampleServiceServer(s1, &grpctest.Server{})
	reflection.Register(s1)

	s2 := grpc.NewServer()
	grpctest.RegisterOtherExampleServiceServer(s2, &grpctest.UnimplementedOtherExampleServiceServer{})
	reflection.Register(s2)

	backend1Addr := grpctest.StartServer(t, s1)
	t.Cleanup(s1.GracefulStop)
	backend2Addr := grpctest.StartServer(t, s2)
	t.Cleanup(s2.GracefulStop)

	conn1, err := grpc.Dial(backend1Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn1.Close()) })

	conn2, err := grpc.Dial(backend2Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn2.Close()) })

	errHandler := func(any, grpc.ServerStream) error {
		return status.Error(codes.Internal, "should not be called")
	}

	mw := Reflector{
		UpstreamsFunc: func() []discovery.Upstream {
			return []discovery.Upstream{
				discovery.ClientConn{
					ConnName:        "backend1",
					ServeReflection: true,
					ClientConn:      conn1,
				},
				discovery.ClientConn{
					ConnName:        "backend2",
					ServeReflection: true,
					ClientConn:      conn2,
				},
			}
		},
	}.Middleware(errHandler)

	srv := grpc.NewServer(grpc.UnknownServiceHandler(func(srv any, stream grpc.ServerStream) error {
		return mw(srv, stream)
	}))
	srvAddr := grpctest.StartServer(t, srv)
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.Dial(srvAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	cl1 := rapi1.NewServerReflectionClient(conn)
	cl1alpha := rapi1alpha.NewServerReflectionClient(conn)

	t.Run("reflection v1", func(t *testing.T) {
		stream, err := cl1.ServerReflectionInfo(context.Background())
		require.NoError(t, err)

		t.Run("list services", func(t *testing.T) {
			require.NoError(t, stream.Send(&rapi1.ServerReflectionRequest{
				MessageRequest: &rapi1.ServerReflectionRequest_ListServices{},
			}))

			resp, err := stream.Recv()
			require.NoError(t, err)

			require.True(t, proto.Equal(&rapi1.ServerReflectionResponse{
				OriginalRequest: &rapi1.ServerReflectionRequest{
					MessageRequest: &rapi1.ServerReflectionRequest_ListServices{},
				},
				MessageResponse: &rapi1.ServerReflectionResponse_ListServicesResponse{
					ListServicesResponse: &rapi1.ListServiceResponse{
						Service: []*rapi1.ServiceResponse{
							{Name: "groxy.testdata.ExampleService"},
							{Name: "groxy.testdata.OtherExampleService"},
							{Name: "grpc.reflection.v1.ServerReflection"},
							{Name: "grpc.reflection.v1alpha.ServerReflection"},
						},
					},
				},
			}, resp))
		})

		t.Run("file by symbol", func(t *testing.T) {
			require.NoError(t, stream.Send(&rapi1.ServerReflectionRequest{
				MessageRequest: &rapi1.ServerReflectionRequest_FileContainingSymbol{
					FileContainingSymbol: "groxy.testdata.ExampleService",
				},
			}))

			resp, err := stream.Recv()
			require.NoError(t, err)

			require.NotNil(t, resp.GetFileDescriptorResponse())
		})
	})

	t.Run("reflection v1alpha", func(t *testing.T) {
		stream, err := cl1alpha.ServerReflectionInfo(context.Background())
		require.NoError(t, err)

		t.Run("list services", func(t *testing.T) {
			require.NoError(t, stream.Send(&rapi1alpha.ServerReflectionRequest{
				MessageRequest: &rapi1alpha.ServerReflectionRequest_ListServices{},
			}))

			resp, err := stream.Recv()
			require.NoError(t, err)

			require.True(t, proto.Equal(&rapi1alpha.ServerReflectionResponse{
				OriginalRequest: &rapi1alpha.ServerReflectionRequest{
					MessageRequest: &rapi1alpha.ServerReflectionRequest_ListServices{},
				},
				MessageResponse: &rapi1alpha.ServerReflectionResponse_ListServicesResponse{
					ListServicesResponse: &rapi1alpha.ListServiceResponse{
						Service: []*rapi1alpha.ServiceResponse{
							{Name: "groxy.testdata.ExampleService"},
							{Name: "groxy.testdata.OtherExampleService"},
							{Name: "grpc.reflection.v1.ServerReflection"},
							{Name: "grpc.reflection.v1alpha.ServerReflection"},
						},
					},
				},
			}, resp))
		})

		t.Run("file by symbol", func(t *testing.T) {
			require.NoError(t, stream.Send(&rapi1alpha.ServerReflectionRequest{
				MessageRequest: &rapi1alpha.ServerReflectionRequest_FileContainingSymbol{
					FileContainingSymbol: "groxy.testdata.ExampleService",
				},
			}))

			resp, err := stream.Recv()
			require.NoError(t, err)

			require.NotNil(t, resp.GetFileDescriptorResponse())
		})
	})
}
