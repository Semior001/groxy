package proxy

import (
	"testing"
	"github.com/Semior001/groxy/pkg/proxy/mocks"
	"google.golang.org/grpc/metadata"
	"github.com/Semior001/groxy/pkg/discovery"
	"regexp"
	"github.com/Semior001/groxy/pkg/protodef/testdata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"math/rand"
	"github.com/stretchr/testify/require"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"context"
)

func TestServer_handle(t *testing.T) {
	matcher := &mocks.MatcherMock{
		MatchMetadataFunc: func(string, metadata.MD) discovery.Matches {
			return discovery.Matches{
				{
					Name: "groxy.runtime_generated.TestService/TestMethod (mock with status)",
					Match: discovery.RequestMatcher{
						URI: regexp.MustCompile("groxy.runtime_generated.TestService/TestMethod"),
						Message: &testdata.Request{
							Value: "error",
						},
					},
					Mock: &discovery.Mock{
						Status: status.New(codes.Internal, "test"),
					},
				},
				{
					Name: "groxy.runtime_generated.TestService/TestMethod (mock with metadata)",
					Match: discovery.RequestMatcher{
						URI: regexp.MustCompile("groxy.runtime_generated.TestService/TestMethod"),
						Message: &testdata.Request{
							Value: "metadata",
						},
					},
					Mock: &discovery.Mock{
						Header: metadata.Pairs("test", "test"),
						Status: status.New(codes.InvalidArgument, "test metadata"),
					},
				},
				{
					Name: "groxy.runtime_generated.TestService/TestMethod (mock with response)",
					Match: discovery.RequestMatcher{
						URI: regexp.MustCompile("groxy.runtime_generated.TestService/TestMethod"),
					},
					Mock: &discovery.Mock{
						Body: &testdata.Response{
							Value: "test",
						},
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

	cl := testdata.NewTestServiceClient(cc)

	t.Run("mock with response", func(t *testing.T) {
		resp, err := cl.TestMethod(context.Background(), &testdata.Request{Value: "test"})
		require.NoError(t, err)

		require.Equal(t, "test", resp.Value)
	})

	t.Run("mock with status", func(t *testing.T) {
		_, err := cl.TestMethod(context.Background(), &testdata.Request{Value: "error"})
		st, ok := status.FromError(err)
		require.True(t, ok)

		require.Equal(t, codes.Internal, st.Code())
		require.Equal(t, "test", st.Message())
	})

	t.Run("mock with metadata", func(t *testing.T) {
		md := &metadata.MD{}
		_, err := cl.TestMethod(context.Background(), &testdata.Request{Value: "metadata"},
			grpc.Header(md))
		st, ok := status.FromError(err)
		require.True(t, ok)

		require.Equal(t, codes.InvalidArgument, st.Code())
		require.Equal(t, "test metadata", st.Message())

		require.Equal(t, "test", (*md)["test"][0])
	})
}
