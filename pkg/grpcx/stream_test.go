package grpcx

import (
	"testing"
	"github.com/Semior001/groxy/pkg/grpcx/grpctest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/assert"
	"errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"google.golang.org/protobuf/proto"
	"context"
)

func TestPipe(t *testing.T) {
	t.Run("man-in-the-middle provides message", func(t *testing.T) {
		srv := &grpctest.Server{
			BiDirectionalFunc: func(srv grpctest.ExampleService_BiDirectionalServer) error {
				expectedReqs := []*grpctest.StreamRequest{
					{Value: "pre-hello"},
					{Value: "hello"},
					{Value: "world"},
				}

				for idx, expectedReq := range expectedReqs {
					t.Logf("expecting request %d", idx)
					req, err := srv.Recv()
					require.NoError(t, err)
					if !assert.True(t, proto.Equal(req, expectedReq), "got: %v", req) {
						return errors.New("unexpected request")
					}
				}

				require.NoError(t, srv.Send(&grpctest.StreamResponse{Value: "hello back"}))
				return nil
			},
		}

		s := grpc.NewServer()
		grpctest.RegisterExampleServiceServer(s, srv)

		backend := grpctest.StartServer(t, s)
		t.Cleanup(s.GracefulStop)

		backendConn, err := grpc.Dial(backend, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)

		frontendS := grpc.NewServer(
			grpc.ForceServerCodec(RawBytesCodec{}),
			grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
				ctx := stream.Context()

				method, ok := grpc.Method(ctx)
				require.True(t, ok)

				desc := &grpc.StreamDesc{
					StreamName: method,
					Handler: func(any, grpc.ServerStream) error {
						t.Error("unexpected call to handler")
						t.FailNow()
						return nil
					},
					ServerStreams: true,
					ClientStreams: true,
				}

				cs, err := backendConn.NewStream(ctx, desc, method, grpc.ForceCodec(RawBytesCodec{}))
				require.NoError(t, err)
				defer cs.CloseSend()

				bts, err := proto.Marshal(&grpctest.StreamRequest{Value: "pre-hello"})
				require.NoError(t, err)
				require.NoError(t, cs.SendMsg(bts))

				err = Pipe(cs, stream)
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}),
		)

		frontend := grpctest.StartServer(t, frontendS)
		t.Cleanup(frontendS.GracefulStop)

		frontendConn, err := grpc.Dial(frontend, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)

		client := grpctest.NewExampleServiceClient(frontendConn)
		stream, err := client.BiDirectional(context.Background())
		require.NoError(t, err)
		defer stream.CloseSend()

		require.NoError(t, stream.Send(&grpctest.StreamRequest{Value: "hello"}))
		require.NoError(t, stream.Send(&grpctest.StreamRequest{Value: "world"}))
	})

	t.Run("no first message", func(t *testing.T) {
		srv := &grpctest.Server{
			BiDirectionalFunc: func(srv grpctest.ExampleService_BiDirectionalServer) error {
				expectedReqs := []*grpctest.StreamRequest{
					{Value: "hello"},
					{Value: "world"},
				}

				for idx, expectedReq := range expectedReqs {
					t.Logf("expecting request %d", idx)
					req, err := srv.Recv()
					require.NoError(t, err)
					if !assert.True(t, proto.Equal(req, expectedReq), "got: %v", req) {
						return errors.New("unexpected request")
					}
				}

				require.NoError(t, srv.Send(&grpctest.StreamResponse{Value: "hello back"}))
				return nil
			},
		}

		s := grpc.NewServer()
		grpctest.RegisterExampleServiceServer(s, srv)

		backend := grpctest.StartServer(t, s)
		t.Cleanup(s.GracefulStop)

		backendConn, err := grpc.Dial(backend, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)

		frontendS := grpc.NewServer(
			grpc.ForceServerCodec(RawBytesCodec{}),
			grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
				ctx := stream.Context()

				method, ok := grpc.Method(ctx)
				require.True(t, ok)

				desc := &grpc.StreamDesc{
					StreamName: method,
					Handler: func(any, grpc.ServerStream) error {
						t.Error("unexpected call to handler")
						t.FailNow()
						return nil
					},
					ServerStreams: true,
					ClientStreams: true,
				}

				cs, err := backendConn.NewStream(ctx, desc, method, grpc.ForceCodec(RawBytesCodec{}))
				require.NoError(t, err)
				defer cs.CloseSend()

				if err = Pipe(cs, stream); errors.Is(err, io.EOF) {

					return nil
				}
				return err
			}),
		)

		frontend := grpctest.StartServer(t, frontendS)
		t.Cleanup(frontendS.GracefulStop)

		frontendConn, err := grpc.Dial(frontend, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)

		client := grpctest.NewExampleServiceClient(frontendConn)
		stream, err := client.BiDirectional(context.Background())
		require.NoError(t, err)
		defer stream.CloseSend()

		require.NoError(t, stream.Send(&grpctest.StreamRequest{Value: "hello"}))
		require.NoError(t, stream.Send(&grpctest.StreamRequest{Value: "world"}))
	})
}
