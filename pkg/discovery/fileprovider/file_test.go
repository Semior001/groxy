package fileprovider

import (
	"context"
	_ "embed"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/Semior001/groxy/pkg/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestFile_Events(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2000*time.Millisecond)
	defer cancel()

	tmp, err := os.CreateTemp(os.TempDir(), "groxy-test-events")
	require.NoError(t, err)
	_ = tmp.Close()
	defer os.Remove(tmp.Name())

	f := File{
		FileName:      tmp.Name(),
		CheckInterval: 100 * time.Millisecond,
		Delay:         500 * time.Millisecond,
	}

	go func() { //nolint:testifylint // we can't use require.NoError here
		time.Sleep(600 * time.Millisecond)
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))
		time.Sleep(600 * time.Millisecond)
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))
		time.Sleep(600 * time.Millisecond)
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))

		// all those event will be ignored, submitted too fast
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))
	}()

	ch := f.Events(ctx)
	events := 0
	for range ch {
		events++
	}
	// expecting events from creation + 3 writes
	assert.Equal(t, 4, events)
}

//go:embed testdata/config.yaml
var f string

func TestFile_Rules(t *testing.T) {
	tmp, err := os.CreateTemp(os.TempDir(), "groxy-test-rules")
	require.NoError(t, err)
	_ = tmp.Close()
	defer os.Remove(tmp.Name())

	require.NoError(t, os.WriteFile(tmp.Name(), []byte(f), 0o600))

	f := File{
		FileName:      tmp.Name(),
		CheckInterval: 100 * time.Millisecond,
		Delay:         200 * time.Millisecond,
	}

	state, err := f.State(context.Background())
	require.NoError(t, err)

	require.Len(t, state.Rules, 6)
	assert.NotNil(t, state.Rules[0].Match.Message)
	assert.NotNil(t, state.Rules[0].Mock.Body)
	assert.NotNil(t, state.Rules[1].Match.Message)
	assert.NotNil(t, state.Rules[1].Mock.Body)
	assert.NotNil(t, state.Rules[2].Mock.Body)
	assert.NotNil(t, state.Rules[4].Forward)

	state.Rules[0].Match.Message = nil
	state.Rules[0].Mock.Body = nil
	state.Rules[1].Match.Message = nil
	state.Rules[1].Mock.Body = nil
	state.Rules[2].Mock.Body = nil
	state.Rules[4].Forward = nil

	assert.Equal(t, []*discovery.Rule{
		{
			Name: "com.github.Semior001.groxy.example.mock.ExampleService/Stub",
			Match: discovery.RequestMatcher{
				URI:              regexp.MustCompile("com.github.Semior001.groxy.example.mock.ExampleService/Stub"),
				IncomingMetadata: map[string][]string{"test": {"true"}},
			},
			Mock: &discovery.Mock{},
		},
		{
			Name: "com.github.Semior001.groxy.example.mock.ExampleService/Stub",
			Match: discovery.RequestMatcher{
				URI:              regexp.MustCompile("com.github.Semior001.groxy.example.mock.ExampleService/Stub"),
				IncomingMetadata: metadata.New(nil),
			},
			Mock: &discovery.Mock{},
		},
		{
			Name: "com.github.Semior001.groxy.example.mock.ExampleService/Stub",
			Match: discovery.RequestMatcher{
				URI:              regexp.MustCompile("com.github.Semior001.groxy.example.mock.ExampleService/Stub"),
				IncomingMetadata: metadata.New(nil),
			},
			Mock: &discovery.Mock{},
		},
		{
			Name: "com.github.Semior001.groxy.example.mock.ExampleService/Error",
			Match: discovery.RequestMatcher{
				URI:              regexp.MustCompile("com.github.Semior001.groxy.example.mock.ExampleService/Error"),
				IncomingMetadata: metadata.New(nil),
			},
			Mock: &discovery.Mock{
				Status:  status.New(codes.InvalidArgument, "invalid request"),
				Header:  metadata.New(map[string]string{"X-Request-Id": "123"}),
				Trailer: metadata.New(map[string]string{"Powered-By": "groxy"}),
			},
		},
		{
			Name: "com.github.Semior001.groxy.example.mock.Upstream/Get",
			Match: discovery.RequestMatcher{
				URI:              regexp.MustCompile("com.github.Semior001.groxy.example.mock.Upstream/Get"),
				IncomingMetadata: metadata.New(nil),
			},
		},
		{
			Name:  "not matched",
			Match: discovery.RequestMatcher{URI: regexp.MustCompile(".*")},
			Mock: &discovery.Mock{
				Status: status.New(codes.NotFound, "some custom not found"),
			},
		},
	}, state.Rules)
}

func TestFile_Upstreams(t *testing.T) {
	tmp, err := os.CreateTemp(os.TempDir(), "groxy-test-rules")
	require.NoError(t, err)
	_ = tmp.Close()
	defer os.Remove(tmp.Name())

	require.NoError(t, os.WriteFile(tmp.Name(), []byte(f), 0o600))

	f := File{
		FileName:      tmp.Name(),
		CheckInterval: 100 * time.Millisecond,
		Delay:         200 * time.Millisecond,
	}

	state, err := f.State(context.Background())
	require.NoError(t, err)

	require.Len(t, state.Upstreams, 2)
	assert.Equal(t, "example-1", state.Upstreams[0].Name())
	assert.Equal(t, "localhost:50051", state.Upstreams[0].Target())
	assert.True(t, state.Upstreams[0].Reflection())
	// TODO: check somehow TLS

	assert.Equal(t, "example-2", state.Upstreams[1].Name())
	assert.Equal(t, "localhost:50052", state.Upstreams[1].Target())
	assert.False(t, state.Upstreams[1].Reflection())
}
