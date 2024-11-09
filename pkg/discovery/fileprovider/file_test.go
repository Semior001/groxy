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
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	tmp, err := os.CreateTemp(os.TempDir(), "groxy-test-events")
	require.NoError(t, err)
	_ = tmp.Close()
	defer os.Remove(tmp.Name())

	f := File{
		FileName:      tmp.Name(),
		CheckInterval: 100 * time.Millisecond,
		Delay:         200 * time.Millisecond,
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))
		time.Sleep(300 * time.Millisecond)
		assert.NoError(t, os.WriteFile(tmp.Name(), []byte("something"), 0o600))
		time.Sleep(300 * time.Millisecond)
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

	assert.NoError(t, os.WriteFile(tmp.Name(), []byte(f), 0o600))

	f := File{
		FileName:      tmp.Name(),
		CheckInterval: 100 * time.Millisecond,
		Delay:         200 * time.Millisecond,
	}

	rules, err := f.Rules(context.Background())
	require.NoError(t, err)

	require.Len(t, rules, 5)
	assert.NotNil(t, rules[0].Match.Message)
	assert.NotNil(t, rules[0].Mock.Messages)
	assert.NotNil(t, rules[1].Match.Message)
	assert.NotNil(t, rules[1].Mock.Messages)
	assert.NotNil(t, rules[2].Mock.Messages)

	rules[0].Match.Message = nil
	rules[0].Mock.Messages = nil
	rules[1].Match.Message = nil
	rules[1].Mock.Messages = nil
	rules[2].Mock.Messages = nil

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
			Name:  "not matched",
			Match: discovery.RequestMatcher{URI: regexp.MustCompile(".*")},
			Mock: &discovery.Mock{
				Status: status.New(codes.NotFound, "some custom not found"),
			},
		},
	}, rules)
}
