package main

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Semior001/groxy/_example"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/Semior001/grpc-echo/echopb"
	"github.com/stretchr/testify/assert"
)

func TestMain_Examples(t *testing.T) {
	// This test basically must be a copy of _example/Taskfile.yml execution.
	// It ensures that the example config works as intended.
	// If you change the example config, please update this test as well.

	_, conn := setup(t, _example.Examples)
	waitForServerUp(t, conn)

	protomsg := func(want proto.Message) func(*testing.T, proto.Message, error, metadata.MD, metadata.MD) {
		return func(t *testing.T, got proto.Message, err error, _, _ metadata.MD) {
			require.NoError(t, err)
			assert.True(t, proto.Equal(want, got),
				"expected and actual proto messages differ:\nexpected: %v\nactual:   %v",
				want, got)
		}
	}

	tests := []struct {
		name    string
		method  string
		headers map[string]string
		input   proto.Message
		wantTyp proto.Message
		want    func(
			t *testing.T,
			resp proto.Message,
			err error,
			headers, trailers metadata.MD,
		)
	}{
		{
			name:    "stub-with-header",
			method:  _example.ExampleService_Stub_FullMethodName,
			headers: map[string]string{"test": "true"},
			input:   &_example.StubRequest{Message: "needed value"},
			wantTyp: &_example.SomeOtherResponse{},
			want:    protomsg(&_example.SomeOtherResponse{Message: "needed value received", Code: 200}),
		},
		{
			name:    "stub-with-body",
			method:  _example.ExampleService_Stub_FullMethodName,
			input:   &_example.StubRequest{Message: "needed value"},
			wantTyp: &_example.SomeOtherResponse{},
			want:    protomsg(&_example.SomeOtherResponse{Message: "lol that works", Code: 400}),
		},
		{
			name:    "stub-random-uuid",
			method:  _example.ExampleService_Stub_FullMethodName,
			input:   &_example.StubRequest{Message: "random"},
			wantTyp: &_example.SomeOtherResponse{},
			want: func(t *testing.T, got proto.Message, err error, _, _ metadata.MD) {
				require.NoError(t, err)
				resp, ok := got.(*_example.SomeOtherResponse)
				require.True(t, ok)
				_, err = uuid.Parse(resp.Message)
				assert.NoError(t, err, "response message is not a valid UUID: %s", resp.Message)
			},
		},
		{
			name:    "stub-matcher",
			method:  _example.ExampleService_Stub_FullMethodName,
			input:   &_example.StubRequest{Message: "matcher", Multiplier: 5},
			wantTyp: &_example.SomeOtherResponse{},
			want:    protomsg(&_example.SomeOtherResponse{Message: "10"}),
		},
		{
			name:    "stub-generic",
			method:  _example.ExampleService_Stub_FullMethodName,
			wantTyp: &_example.SomeOtherResponse{},
			want: protomsg(&_example.SomeOtherResponse{
				Dependency: &_example.Dependency{
					Value:    "some value",
					Flag:     true,
					RichText: "some text",
				},
			}),
		},
		{
			name:    "error",
			method:  _example.ExampleService_Error_FullMethodName,
			wantTyp: &_example.SomeOtherResponse{},
			want: func(t *testing.T, got proto.Message, err error, headers, trailers metadata.MD) {
				require.Empty(t, got)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, codes.InvalidArgument, st.Code())
				assert.Equal(t, "invalid request", st.Message())
				require.Len(t, st.Details(), 0)
				assert.Equal(t, []string{"123"}, headers.Get("X-Request-Id"))
				assert.Equal(t, []string{"groxy"}, trailers.Get("Powered-By"))
			},
		},
		{
			name:    "upstream",
			method:  echopb.EchoService_Echo_FullMethodName,
			headers: map[string]string{"X-Request-Id": "12345"},
			input:   &echopb.EchoRequest{Ping: "hello"},
			wantTyp: &echopb.EchoResponse{},
			want: func(t *testing.T, resp proto.Message, err error, _, _ metadata.MD) {
				require.NoError(t, err)
				echoResp, ok := resp.(*echopb.EchoResponse)
				require.True(t, ok)
				assert.Equalf(t, "hello", echoResp.Body,
					"unexpected echo response: %s", echoResp.Body)
				assert.Equal(t, "12345", echoResp.Headers["x-request-id"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.wantTyp
			ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(tt.headers))

			var headers, trailers metadata.MD
			err := conn.Invoke(ctx, tt.method, tt.input, resp, grpc.Header(&headers), grpc.Trailer(&trailers))
			tt.want(t, resp, err, headers, trailers)
		})
	}
}

func TestMain_ReverseProxy(t *testing.T) {
	echoAddr := startEchoServer(t)
	_, conn := setup(t, fmt.Sprintf(`
version: 1
upstreams: { benchmark: { address: "%s" } }
rules: [{ match: { uri: "(.*)" }, forward: { upstream: "benchmark" } }]`, echoAddr))
	waitForServerUp(t, conn)

	client := echopb.NewEchoServiceClient(conn)

	resp, err := client.Echo(context.Background(), &echopb.EchoRequest{Ping: "hello"})
	require.NoError(t, err)

	t.Logf("%+v", resp)
}

func TestMain_StdinConfig(t *testing.T) {
	// Save original stdin and restore after test
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create a pipe to use as stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdin = r

	// Write the config to the pipe
	config := `
version: 1
rules:
  - match: { uri: "/grpc_echo.v1.EchoService/Echo" }
    respond:
      body: |
        message Response {
          option (groxypb.target) = true;
          string body = 2 [(groxypb.value) = "stdin config worked"];
        }
`
	_, err = w.WriteString(config)
	require.NoError(t, err)
	w.Close()

	port := 40000 + int(rand.Int31n(10000))
	os.Args = []string{"test", "--stdin", "--addr=:" + fmt.Sprint(port)}

	// Start the server
	done := make(chan struct{})
	go func() {
		<-done
		e := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		assert.NoError(t, e)
	}()

	started, finished := make(chan struct{}), make(chan struct{})
	go func() {
		t.Logf("running server on port %d with stdin config", port)
		close(started)
		main()
		close(finished)
	}()

	t.Cleanup(func() {
		close(done)
		<-finished
	})

	<-started
	time.Sleep(time.Millisecond * 50) // do not start right away

	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUserAgent("groxy-test-ua"))
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := conn.Close(); err != nil &&
			!strings.Contains(err.Error(), "grpc: the client connection is closing") {
			t.Errorf("failed to close connection: %v", err)
		}
	})

	waitForServerUp(t, conn)

	// Test the server
	client := echopb.NewEchoServiceClient(conn)
	resp, err := client.Echo(context.Background(), &echopb.EchoRequest{Ping: "hello"})
	require.NoError(t, err)
	assert.Equal(t, "stdin config worked", resp.Body)
}

//nolint:unparam // port is ok to be unused, will be needed in further tests
func setup(tb testing.TB, config string, flags ...string) (port int, conn *grpc.ClientConn) {
	tb.Helper()

	f, err := os.CreateTemp("", "groxy-test")
	require.NoError(tb, err)
	tb.Cleanup(func() { require.NoError(tb, os.Remove(f.Name())) })

	tb.Logf("writing config to %s", f.Name())

	_, err = f.WriteString(config)
	require.NoError(tb, err)
	require.NoError(tb, f.Close())

	port = 40000 + int(rand.Int31n(10000))
	os.Args = append([]string{"test", "--file.name=" + f.Name(), "--addr=:" + fmt.Sprint(port)}, flags...)

	done := make(chan struct{})
	go func() {
		<-done
		e := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		assert.NoError(tb, e)
	}()

	started, finished := make(chan struct{}), make(chan struct{})
	go func() {
		tb.Logf("running server on port %d", port)
		close(started)
		main()
		close(finished)
	}()

	tb.Cleanup(func() {
		close(done)
		<-finished
	})

	<-started
	time.Sleep(time.Millisecond * 50) // do not start right away
	conn, err = grpc.NewClient(fmt.Sprintf("localhost:%d", port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUserAgent("groxy-test-ua"))
	require.NoError(tb, err)

	tb.Cleanup(func() {
		if err := conn.Close(); err != nil &&
			!strings.Contains(err.Error(), "grpc: the client connection is closing") {
			tb.Errorf("failed to close connection: %v", err)
		}
	})

	tb.Logf("server started on port %d", port)
	return port, conn
}

func waitForServerUp(tb testing.TB, conn *grpc.ClientConn) {
	tb.Helper()

	healthClient := healthpb.NewHealthClient(conn)
	for i := 0; i < 100; i++ {
		time.Sleep(time.Millisecond * 100)
		st, err := healthClient.Check(context.Background(), &healthpb.HealthCheckRequest{})
		if err == nil && st.Status == healthpb.HealthCheckResponse_SERVING {
			tb.Logf("server is up")
			return
		}
	}
	tb.Fatal("server is not up")
}

func startEchoServer(tb testing.TB) (addr string) {
	tb.Helper()
	ctx := context.Background()

	provider, err := testcontainers.ProviderDocker.GetProvider()
	if err != nil {
		tb.Skipf("docker not available, skipping test: %s", err)
	}
	if err = provider.Health(ctx); err != nil {
		tb.Skipf("docker not healthy, skipping test: %s", err)
	}

	echoReq := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "semior/grpc-echo:latest",
			ExposedPorts: []string{"8080/tcp"},
			WaitingFor:   wait.ForLog("listening gRPC"),
		},
		Started: true,
	}
	echo, err := testcontainers.GenericContainer(ctx, echoReq)
	testcontainers.CleanupContainer(tb, echo)
	require.NoError(tb, err)

	echoIP, err := echo.Host(ctx)
	require.NoError(tb, err)

	echoPort, err := echo.MappedPort(ctx, "8080")
	require.NoError(tb, err)

	addr = net.JoinHostPort(echoIP, echoPort.Port())
	tb.Logf("started echo server at %s", addr)
	return addr
}
