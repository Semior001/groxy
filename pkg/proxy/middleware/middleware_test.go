package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/Semior001/groxy/pkg/proxy/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"github.com/Semior001/groxy/pkg/grpcx/grpctest"
	"google.golang.org/grpc/credentials/insecure"
)

func TestAppInfo(t *testing.T) {
	mw := AppInfo("app", "author", "version")
	var header metadata.MD
	ss := &mocks.ServerStreamMock{
		SetHeaderFunc: func(md metadata.MD) error {
			header = md
			return nil
		},
	}

	err := mw(func(_ any, _ grpc.ServerStream) error { return nil })(nil, ss)
	require.NoError(t, err)

	assert.Equal(t, metadata.Pairs(
		"app", "app",
		"author", "author",
		"version", "version",
	), header)
}

func TestRecoverer(t *testing.T) {
	bts := bytes.NewBuffer(nil)
	slog.SetDefault(slog.New(slog.NewTextHandler(bts, &slog.HandlerOptions{})))
	mw := Recoverer("{groxy} panic")(func(_ any, _ grpc.ServerStream) error { panic("test") })
	var err error
	require.NotPanics(t, func() {
		err = mw(nil, &mocks.ServerStreamMock{
			ContextFunc: context.Background,
		})
	})
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	assert.Equal(t, "{groxy} panic", st.Message())
}

func TestChain(t *testing.T) {
	var calls []string
	mw1 := func(next grpc.StreamHandler) grpc.StreamHandler {
		return func(srv any, stream grpc.ServerStream) error {
			calls = append(calls, "mw1")
			return next(srv, stream)
		}
	}
	mw2 := func(next grpc.StreamHandler) grpc.StreamHandler {
		return func(srv any, stream grpc.ServerStream) error {
			calls = append(calls, "mw2")
			return next(srv, stream)
		}
	}
	mw3 := func(next grpc.StreamHandler) grpc.StreamHandler {
		return func(srv any, stream grpc.ServerStream) error {
			calls = append(calls, "mw3")
			return next(srv, stream)
		}
	}
	h := Wrap(func(_ any, _ grpc.ServerStream) error { return nil }, mw1, mw2, mw3)
	require.NoError(t, h(nil, nil))
	assert.Equal(t, []string{"mw1", "mw2", "mw3"}, calls)
}

func TestHealth(t *testing.T) {
	prepare := func() (*health.Server, healthpb.HealthClient) {
		h := health.NewServer()
		h.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

		srv := grpc.NewServer(grpc.UnknownServiceHandler(Health(h)(func(_ any, _ grpc.ServerStream) error {
			return status.Error(codes.Internal, "must not be called")
		})))

		addr := grpctest.StartServer(t, srv)

		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)

		cl := healthpb.NewHealthClient(conn)

		return h, cl
	}

	t.Run("unary", func(t *testing.T) {
		h, cl := prepare()

		resp, err := cl.Check(context.Background(), &healthpb.HealthCheckRequest{})
		require.NoError(t, err)

		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)

		h.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

		resp, err = cl.Check(context.Background(), &healthpb.HealthCheckRequest{})
		require.NoError(t, err)

		assert.Equal(t, healthpb.HealthCheckResponse_NOT_SERVING, resp.Status)
	})

	t.Run("watch", func(t *testing.T) {
		h, cl := prepare()

		stream, err := cl.Watch(context.Background(), &healthpb.HealthCheckRequest{})
		require.NoError(t, err)
		defer stream.CloseSend()

		resp, err := stream.Recv()
		require.NoError(t, err)

		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)

		h.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

		resp, err = stream.Recv()
		require.NoError(t, err)

		assert.Equal(t, healthpb.HealthCheckResponse_NOT_SERVING, resp.Status)
	})

}
