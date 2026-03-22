package main

import (
	"context"
	_ "embed"
	"fmt"
	"maps"
	"math/rand"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Semior001/groxy/_example"
	examplepb "github.com/Semior001/groxy/_example/gen"
	"github.com/Semior001/groxy/pkg/discovery/fileprovider"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gopkg.in/yaml.v3"

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

// exampleConfigPaths lists all example config files in the order matching
// the original mock.yaml rule ordering (important for the combined test).
var exampleConfigPaths = []string{
	"header-matching/config.yaml",
	"templating/config.yaml",
	"body-matching/config.yaml",
	"nested-messages/config.yaml",
	"error-responses/config.yaml",
	"upstream-forwarding/config.yaml",
	"uri-rewrite/config.yaml",
}

// readExampleConfig reads and returns a single example config from the embedded FS.
func readExampleConfig(t *testing.T, path string) string {
	t.Helper()
	data, err := _example.ExampleConfigs.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// combinedExampleConfig merges all example configs into one, preserving rule
// order. This produces a config similar to the original monolithic mock.yaml,
// combining rules and upstreams but not merging any not-matched defaults.
func combinedExampleConfig(t *testing.T) string {
	t.Helper()
	combined := fileprovider.Config{
		Version:   "1",
		Upstreams: map[string]fileprovider.Upstream{},
	}
	for _, path := range exampleConfigPaths {
		data, err := _example.ExampleConfigs.ReadFile(path)
		require.NoError(t, err)

		var cfg fileprovider.Config
		require.NoError(t, yaml.Unmarshal(data, &cfg))

		combined.Rules = append(combined.Rules, cfg.Rules...)
		maps.Copy(combined.Upstreams, cfg.Upstreams)
	}
	out, err := yaml.Marshal(combined)
	require.NoError(t, err)
	return string(out)
}

func TestMain_Examples(t *testing.T) {
	// Combined regression test: loads all example configs merged into one
	// server to approximate the original monolithic mock.yaml (rule ordering,
	// upstream wiring) but without any global not-matched defaults.

	echoAddr := startEchoServer(t)

	cfg := combinedExampleConfig(t)
	cfg = strings.ReplaceAll(cfg, "grpc-echo.semior.dev:443", echoAddr)
	cfg = strings.ReplaceAll(cfg, "tls: true", "tls: false")

	_, conn := setup(t, cfg)
	waitForServerUp(t, conn)

	protomsg := func(want proto.Message) func(*testing.T, proto.Message, error, metadata.MD, metadata.MD) {
		return func(t *testing.T, got proto.Message, err error, _, _ metadata.MD) {
			require.NoError(t, err)
			assert.True(t, proto.Equal(want, got),
				"expected and actual proto messages differ:\nexpected: %v\nactual:   %v",
				want, got)
		}
	}

	t.Run("stub-matcher", func(t *testing.T) {
		c := examplepb.NewExampleServiceClient(conn)
		started := time.Now()
		resp, err := c.Stub(t.Context(), &examplepb.StubRequest{Message: "matcher", Multiplier: 5})
		require.NoError(t, err)
		assert.True(t, proto.Equal(&examplepb.SomeOtherResponse{Message: "10"}, resp), "unexpected response: %v", resp)
		assert.GreaterOrEqual(t, time.Since(started), 2*time.Second)
	})

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
			method:  examplepb.ExampleService_Stub_FullMethodName,
			headers: map[string]string{"test": "true"},
			input:   &examplepb.StubRequest{Message: "needed value"},
			wantTyp: &examplepb.SomeOtherResponse{},
			want:    protomsg(&examplepb.SomeOtherResponse{Message: "needed value received", Code: 200}),
		},
		{
			name:    "stub-with-body",
			method:  examplepb.ExampleService_Stub_FullMethodName,
			input:   &examplepb.StubRequest{Message: "needed value"},
			wantTyp: &examplepb.SomeOtherResponse{},
			want:    protomsg(&examplepb.SomeOtherResponse{Message: "lol that works", Code: 400}),
		},
		{
			name:    "stub-random-uuid",
			method:  examplepb.ExampleService_Stub_FullMethodName,
			input:   &examplepb.StubRequest{Message: "random"},
			wantTyp: &examplepb.SomeOtherResponse{},
			want: func(t *testing.T, got proto.Message, err error, _, _ metadata.MD) {
				require.NoError(t, err)
				resp, ok := got.(*examplepb.SomeOtherResponse)
				require.True(t, ok)
				_, err = uuid.Parse(resp.Message)
				assert.NoError(t, err, "response message is not a valid UUID: %s", resp.Message)
			},
		},
		{
			name:    "stub-generic",
			method:  examplepb.ExampleService_Stub_FullMethodName,
			wantTyp: &examplepb.SomeOtherResponse{},
			want: protomsg(&examplepb.SomeOtherResponse{
				Dependency: &examplepb.Dependency{
					Value:    "some value",
					Flag:     true,
					RichText: "some text",
				},
			}),
		},
		{
			name:    "error",
			method:  examplepb.ExampleService_Error_FullMethodName,
			wantTyp: &examplepb.SomeOtherResponse{},
			want: func(t *testing.T, got proto.Message, err error, headers, trailers metadata.MD) {
				require.Empty(t, got)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, codes.InvalidArgument, st.Code())
				assert.Equal(t, "invalid request", st.Message())
				require.Empty(t, st.Details())
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

func TestMain_ExampleHeaderMatching(t *testing.T) {
	_, conn := setup(t, readExampleConfig(t, "header-matching/config.yaml"))
	waitForServerUp(t, conn)

	c := examplepb.NewExampleServiceClient(conn)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "test", "true")
	resp, err := c.Stub(ctx, &examplepb.StubRequest{Message: "needed value"})
	require.NoError(t, err)
	assert.True(t, proto.Equal(&examplepb.SomeOtherResponse{Message: "needed value received", Code: 200}, resp))
}

func TestMain_ExampleTemplating(t *testing.T) {
	_, conn := setup(t, readExampleConfig(t, "templating/config.yaml"))
	waitForServerUp(t, conn)

	c := examplepb.NewExampleServiceClient(conn)

	t.Run("uuid", func(t *testing.T) {
		resp, err := c.Stub(t.Context(), &examplepb.StubRequest{Message: "random"})
		require.NoError(t, err)
		_, err = uuid.Parse(resp.Message)
		assert.NoError(t, err, "response message is not a valid UUID: %s", resp.Message)
	})

	t.Run("expression-matcher", func(t *testing.T) {
		started := time.Now()
		resp, err := c.Stub(t.Context(), &examplepb.StubRequest{Message: "matcher", Multiplier: 5})
		require.NoError(t, err)
		assert.True(t, proto.Equal(&examplepb.SomeOtherResponse{Message: "10"}, resp))
		assert.GreaterOrEqual(t, time.Since(started), 2*time.Second)
	})
}

func TestMain_ExampleBodyMatching(t *testing.T) {
	_, conn := setup(t, readExampleConfig(t, "body-matching/config.yaml"))
	waitForServerUp(t, conn)

	c := examplepb.NewExampleServiceClient(conn)
	resp, err := c.Stub(t.Context(), &examplepb.StubRequest{Message: "needed value"})
	require.NoError(t, err)
	assert.True(t, proto.Equal(&examplepb.SomeOtherResponse{Message: "lol that works", Code: 400}, resp))
}

func TestMain_ExampleNestedMessages(t *testing.T) {
	_, conn := setup(t, readExampleConfig(t, "nested-messages/config.yaml"))
	waitForServerUp(t, conn)

	c := examplepb.NewExampleServiceClient(conn)
	resp, err := c.Stub(t.Context(), &examplepb.StubRequest{})
	require.NoError(t, err)
	assert.True(t, proto.Equal(&examplepb.SomeOtherResponse{
		Dependency: &examplepb.Dependency{
			Value:    "some value",
			Flag:     true,
			RichText: "some text",
		},
	}, resp))
}

func TestMain_ExampleErrorResponses(t *testing.T) {
	_, conn := setup(t, readExampleConfig(t, "error-responses/config.yaml"))
	waitForServerUp(t, conn)

	var headers, trailers metadata.MD
	err := conn.Invoke(
		context.Background(),
		examplepb.ExampleService_Error_FullMethodName,
		nil,
		&examplepb.SomeOtherResponse{},
		grpc.Header(&headers),
		grpc.Trailer(&trailers),
	)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Equal(t, "invalid request", st.Message())
	assert.Equal(t, []string{"123"}, headers.Get("X-Request-Id"))
	assert.Equal(t, []string{"groxy"}, trailers.Get("Powered-By"))
}

func TestMain_ExampleUpstreamForwarding(t *testing.T) {
	echoAddr := startEchoServer(t)

	cfg := readExampleConfig(t, "upstream-forwarding/config.yaml")
	cfg = strings.ReplaceAll(cfg, "grpc-echo.semior.dev:443", echoAddr)
	cfg = strings.ReplaceAll(cfg, "tls: true", "tls: false")

	_, conn := setup(t, cfg)
	waitForServerUp(t, conn)

	ctx := metadata.AppendToOutgoingContext(context.Background(), "X-Request-Id", "12345")
	resp := &echopb.EchoResponse{}
	err := conn.Invoke(ctx, echopb.EchoService_Echo_FullMethodName, &echopb.EchoRequest{Ping: "hello"}, resp)
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Body)
	assert.Equal(t, "12345", resp.Headers["x-request-id"])
}

func TestMain_ExampleURIRewrite(t *testing.T) {
	echoAddr := startEchoServer(t)

	cfg := readExampleConfig(t, "uri-rewrite/config.yaml")
	cfg = strings.ReplaceAll(cfg, "grpc-echo.semior.dev:443", echoAddr)
	cfg = strings.ReplaceAll(cfg, "tls: true", "tls: false")

	_, conn := setup(t, cfg)
	waitForServerUp(t, conn)

	c := examplepb.NewExampleServiceClient(conn)
	resp, err := c.SomeEchoMethod(t.Context(), &examplepb.EchoRequest{Ping: "hello"})
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Body)
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
	for range 100 {
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
