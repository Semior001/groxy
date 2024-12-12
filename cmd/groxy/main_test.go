package main

import (
	"testing"
	"net"
	"github.com/stretchr/testify/require"
	"fmt"
	"math/rand"
	"os"
	"syscall"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"context"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"time"
	"google.golang.org/grpc/credentials/insecure"
	"strings"

	"github.com/Semior001/grpc-echo/echopb"
)

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
		require.NoError(tb, e)
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