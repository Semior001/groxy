package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
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

func mustProtoMarshal(t *testing.T, msg proto.Message) []byte {
	bts, err := proto.Marshal(msg)
	require.NoError(t, err)
	return bts
}
