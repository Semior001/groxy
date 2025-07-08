package discovery

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/metadata"
)

func TestRule_String(t *testing.T) {
	got := (&Rule{
		Name: "name",
		Match: RequestMatcher{
			IncomingMetadata: map[string]*regexp.Regexp{
				"key":  regexp.MustCompile("value"),
				"key2": regexp.MustCompile("value2"),
			},
			Message: &errdetails.RequestInfo{
				RequestId:   "request-id",
				ServingData: "serving-data",
			},
		},
	}).String()

	// for some reason, the proto message string sometimes generates two spaces between fields
	got = strings.ReplaceAll(got, "  ", " ")

	assert.Equal(t, "(name; 2 metadata; with body: {request_id:\"request-id\" serving_data:\"serving-data\"})", got)
}

func TestRequestMatcher_Matches(t *testing.T) {
	rm := RequestMatcher{
		URI:              regexp.MustCompile(`^/v1/example/.*$`),
		IncomingMetadata: map[string]*regexp.Regexp{"key": regexp.MustCompile("^value.*$")},
	}
	assert.True(t, rm.Matches("/v1/example/123", metadata.New(map[string]string{"key": "value"})))
	assert.True(t, rm.Matches("/v1/example/123", metadata.New(map[string]string{"key": "value123"})))
	assert.False(t, rm.Matches("/v1/example/123", metadata.New(map[string]string{"key": "v"})))
	assert.False(t, rm.Matches("/v1/example/123", metadata.New(map[string]string{"another": "value"})))
	assert.False(t, rm.Matches("/v2/example/123", metadata.New(map[string]string{"key": "value"})))
}
