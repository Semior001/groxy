package discovery

import (
	"regexp"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"context"
	"time"
	"errors"
)

func TestMatches_NeedsDeeperMatch(t *testing.T) {
	got := Matches{
		{},
		{Match: RequestMatcher{Message: &errdetails.RequestInfo{}}},
		{},
	}.NeedsDeeperMatch()
	assert.True(t, got)

	got = Matches{
		{},
		{},
		{},
	}.NeedsDeeperMatch()
	assert.False(t, got)
}

func TestService_MatchMetadata(t *testing.T) {
	svc := &Service{
		rules: []*Rule{
			{Name: "1", Match: RequestMatcher{
				URI:              regexp.MustCompile("test"),
				IncomingMetadata: metadata.New(map[string]string{"uri": "not-match"}),
			}},
			{Name: "2", Match: RequestMatcher{
				URI:              regexp.MustCompile("uri"),
				IncomingMetadata: metadata.New(map[string]string{"uri": "test"}),
			}},
			{Name: "3", Match: RequestMatcher{
				URI:              regexp.MustCompile("uri"),
				IncomingMetadata: metadata.New(map[string]string{"uri": "test"}),
			}},
		},
	}

	matches := svc.MatchMetadata("uri", metadata.New(map[string]string{"uri": "test"}))
	assert.Equal(t, Matches(svc.rules[1:]), matches)
}

func TestMatches_MatchMessage(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		r, ok := Matches{
			{Name: "1", Match: RequestMatcher{Message: &errdetails.RequestInfo{RequestId: "1"}}},
			{Name: "2", Match: RequestMatcher{Message: &errdetails.RequestInfo{RequestId: "2"}}},
			{Name: "3", Match: RequestMatcher{Message: &errdetails.RequestInfo{RequestId: "3"}}},
		}.MatchMessage(mustProtoMarshal(t, &errdetails.RequestInfo{RequestId: "2"}))
		require.True(t, ok)
		assert.Equal(t, "2", r.Name)
	})

	t.Run("match first non-empty", func(t *testing.T) {
		r, ok := Matches{
			{Name: "1", Match: RequestMatcher{Message: &errdetails.RequestInfo{RequestId: "1"}}},
			{Name: "2", Match: RequestMatcher{Message: &errdetails.RequestInfo{RequestId: "2"}}},
			{Name: "empty body", Match: RequestMatcher{}},
		}.MatchMessage(mustProtoMarshal(t, &errdetails.RequestInfo{RequestId: "3"}))
		require.True(t, ok)
		assert.Equal(t, "empty body", r.Name)
	})

	t.Run("no match", func(t *testing.T) {
		r, ok := Matches{
			{Name: "1", Match: RequestMatcher{Message: &errdetails.RequestInfo{RequestId: "1"}}},
			{Name: "2", Match: RequestMatcher{Message: &errdetails.RequestInfo{RequestId: "2"}}},
			{Name: "3", Match: RequestMatcher{Message: &errdetails.RequestInfo{RequestId: "3"}}},
		}.MatchMessage(mustProtoMarshal(t, &errdetails.RequestInfo{RequestId: "4"}))
		require.False(t, ok)
		assert.Empty(t, r)
	})
}

func TestService_Run(t *testing.T) {
	t.Run("merge multiple providers", func(t *testing.T) {
		p1 := &ProviderMock{
			NameFunc: func() string { return "p1" },
			EventsFunc: func(context.Context) <-chan string {
				res := make(chan string, 1)
				res <- "file:/file1"
				return res
			},
			StateFunc: func(context.Context) (*State, error) {
				return &State{Rules: []*Rule{
					{Name: "1", Match: RequestMatcher{}},
					{Name: "2", Match: RequestMatcher{IncomingMetadata: metadata.New(map[string]string{"uri": "test"})}},
				}}, nil
			},
		}
		p2 := &ProviderMock{
			NameFunc: func() string { return "p2" },
			EventsFunc: func(context.Context) <-chan string {
				return make(chan string, 1)
			},
			StateFunc: func(context.Context) (*State, error) {
				return &State{Rules: []*Rule{
					{Name: "3", Match: RequestMatcher{IncomingMetadata: metadata.New(map[string]string{
						"uri":  "test",
						"uri2": "test2",
					})}},
				}}, nil
			},
		}
		p3 := &ProviderMock{
			NameFunc: func() string { return "p3" },
			EventsFunc: func(context.Context) <-chan string {
				return make(chan string, 1)
			},
			StateFunc: func(context.Context) (*State, error) { return nil, errors.New("failed to get rules") },
		}

		svc := &Service{Providers: []Provider{p1, p2, p3}}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err := svc.Run(ctx)
		require.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
		assert.Equal(t, []*Rule{
			{Name: "3", Match: RequestMatcher{IncomingMetadata: metadata.New(map[string]string{
				"uri":  "test",
				"uri2": "test2",
			})}},
			{Name: "2", Match: RequestMatcher{IncomingMetadata: metadata.New(map[string]string{"uri": "test"})}},
			{Name: "1", Match: RequestMatcher{}},
		}, svc.rules)
	})

	t.Run("fail on error", func(t *testing.T) {
		failure := errors.New("failed to get rules")

		p1 := &ProviderMock{
			NameFunc: func() string { return "p1" },
			EventsFunc: func(context.Context) <-chan string {
				res := make(chan string, 1)
				res <- "file:/file1"
				return res
			},
			StateFunc: func(context.Context) (*State, error) {
				return &State{Rules: []*Rule{
					{Name: "1", Match: RequestMatcher{}},
					{Name: "2", Match: RequestMatcher{IncomingMetadata: metadata.New(map[string]string{"uri": "test"})}},
				}}, nil
			},
		}
		p2 := &ProviderMock{
			NameFunc: func() string { return "p2" },
			EventsFunc: func(context.Context) <-chan string {
				return make(chan string, 1)
			},
			StateFunc: func(context.Context) (*State, error) {
				return &State{Rules: []*Rule{
					{Name: "3", Match: RequestMatcher{IncomingMetadata: metadata.New(map[string]string{
						"uri":  "test",
						"uri2": "test2",
					})}},
				}}, nil
			},
		}
		p3 := &ProviderMock{
			NameFunc: func() string { return "p3" },
			EventsFunc: func(context.Context) <-chan string {
				return make(chan string, 1)
			},
			StateFunc: func(context.Context) (*State, error) { return nil, failure },
		}

		svc := &Service{Providers: []Provider{p1, p2, p3}, StopOnError: true}
		err := svc.Run(context.Background()) // run indefinitely
		assert.ErrorIs(t, err, failure)
	})
}

func mustProtoMarshal(t *testing.T, msg proto.Message) []byte {
	bts, err := proto.Marshal(msg)
	require.NoError(t, err)
	return bts
}
