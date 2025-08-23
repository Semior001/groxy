package protodef

import (
	"context"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestTemplatingFunctions(t *testing.T) {
	testTemplate := func(def string, data map[string]any, validator func(t *testing.T, msgStr string)) {
		tmpl, err := BuildMessage(def)
		require.NoError(t, err)

		msg, err := tmpl.Generate(context.Background(), data)
		require.NoError(t, err)

		msgBytes, err := proto.Marshal(msg)
		require.NoError(t, err)

		validator(t, string(msgBytes))
	}

	t.Run("uuidv4 function", func(t *testing.T) {
		uuid.SetRand(rand.New(rand.NewSource(42)))

		def := `message TestResponse {
			option (groxypb.target) = true;
			string value = 1 [(groxypb.value) = "{{uuidv4}}"];
		}`

		testTemplate(def, nil, func(t *testing.T, msgStr string) {
			assert.Regexp(t, `[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`, msgStr)
		})
	})

	t.Run("mul function with data", func(t *testing.T) {
		def := `message TestResponse {
			option (groxypb.target) = true;
			string value = 1 [(groxypb.value) = "{{mul .multiplier 3}}"];
		}`
		data := map[string]any{"multiplier": 5}

		testTemplate(def, data, func(t *testing.T, msgStr string) {
			assert.Contains(t, msgStr, "15")
		})
	})

	t.Run("env function with environment variable", func(t *testing.T) {
		require.NoError(t, os.Setenv("TEST_TEMPLATE_VAR", "test-value"))
		defer os.Unsetenv("TEST_TEMPLATE_VAR")

		def := `message TestResponse {
			option (groxypb.target) = true;
			string value = 1 [(groxypb.value) = "{{env \"TEST_TEMPLATE_VAR\"}}"];
		}`

		testTemplate(def, nil, func(t *testing.T, msgStr string) {
			assert.Contains(t, msgStr, "test-value")
		})
	})

	t.Run("sprig upper function", func(t *testing.T) {
		def := `message TestResponse {
			option (groxypb.target) = true;
			string value = 1 [(groxypb.value) = "{{upper \"hello world\"}}"];
		}`

		testTemplate(def, nil, func(t *testing.T, msgStr string) {
			assert.Contains(t, msgStr, "HELLO WORLD")
		})
	})

	t.Run("combination of functions", func(t *testing.T) {
		def := `message TestResponse {
			option (groxypb.target) = true;
			string value = 1 [(groxypb.value) = "Result: {{upper (printf \"num-%d\" (mul .factor 2))}}"];
		}`
		data := map[string]any{"factor": 21}

		testTemplate(def, data, func(t *testing.T, msgStr string) {
			assert.Contains(t, msgStr, "Result: NUM-42")
		})
	})
}

func TestTemplateDataExtraction(t *testing.T) {
	def := `message TestRequest {
		option (groxypb.target) = true;
		string message = 1;
		int32 multiplier = 2;
		bool flag = 3;
	}`

	tmpl, err := BuildMessage(def)
	require.NoError(t, err)

	reqDef := `message TestRequest {
		option (groxypb.target) = true;
		string message = 1 [(groxypb.value) = "hello"];
		int32 multiplier = 2 [(groxypb.value) = "42"];
		bool flag = 3 [(groxypb.value) = "true"];
	}`

	reqTmpl, err := BuildMessage(reqDef)
	require.NoError(t, err)

	reqMsg, err := reqTmpl.Generate(context.Background(), nil)
	require.NoError(t, err)

	reqBytes, err := proto.Marshal(reqMsg)
	require.NoError(t, err)

	extractedData, err := tmpl.DataMap(context.Background(), reqBytes)
	require.NoError(t, err)

	assert.Equal(t, "hello", extractedData["message"])
	assert.Equal(t, int32(42), extractedData["multiplier"])
	assert.Equal(t, true, extractedData["flag"])
}

func TestTemplateMatching(t *testing.T) {
	tests := []struct {
		name     string
		template string
		testMsg  string
		data     map[string]any
		matches  bool
	}{
		{
			name: "static field matching",
			template: `message TestRequest {
				option (groxypb.target) = true;
				string message = 1 [(groxypb.value) = "exact-match"];
			}`,
			testMsg: `message TestRequest {
				option (groxypb.target) = true;
				string message = 1 [(groxypb.value) = "exact-match"];
			}`,
			matches: true,
		},
		{
			name: "static field not matching",
			template: `message TestRequest {
				option (groxypb.target) = true;
				string message = 1 [(groxypb.value) = "exact-match"];
			}`,
			testMsg: `message TestRequest {
				option (groxypb.target) = true;
				string message = 1 [(groxypb.value) = "different-value"];
			}`,
			matches: false,
		},
		{
			name: "matcher field with condition",
			template: `message TestRequest {
				option (groxypb.target) = true;
				int32 value = 1 [(groxypb.matcher) = "value > 10"];
			}`,
			testMsg: `message TestRequest {
				option (groxypb.target) = true;
				int32 value = 1 [(groxypb.value) = "15"];
			}`,
			matches: true,
		},
		{
			name: "matcher field with failing condition",
			template: `message TestRequest {
				option (groxypb.target) = true;
				int32 value = 1 [(groxypb.matcher) = "value > 10"];
			}`,
			testMsg: `message TestRequest {
				option (groxypb.target) = true;
				int32 value = 1 [(groxypb.value) = "5"];
			}`,
			matches: false,
		},
		{
			name: "combined static and matcher fields",
			template: `message TestRequest {
				option (groxypb.target) = true;
				string type = 1 [(groxypb.value) = "test"];
				int32 value = 2 [(groxypb.matcher) = "value >= 0"];
			}`,
			testMsg: `message TestRequest {
				option (groxypb.target) = true;
				string type = 1 [(groxypb.value) = "test"];
				int32 value = 2 [(groxypb.value) = "42"];
			}`,
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build template
			tmpl, err := BuildMessage(tt.template)
			require.NoError(t, err)

			// Build test message
			testTmpl, err := BuildMessage(tt.testMsg)
			require.NoError(t, err)

			testMsg, err := testTmpl.Generate(context.Background(), tt.data)
			require.NoError(t, err)

			testBytes, err := proto.Marshal(testMsg)
			require.NoError(t, err)

			// Test matching
			matches, err := tmpl.Matches(context.Background(), testBytes)
			require.NoError(t, err)

			assert.Equal(t, tt.matches, matches)
		})
	}
}

func TestComplexTemplatingScenarios(t *testing.T) {
	uuid.SetRand(rand.New(rand.NewSource(123)))

	tests := []struct {
		name     string
		def      string
		data     map[string]any
		validate func(t *testing.T, msg proto.Message)
	}{
		{
			name: "nested message with templating",
			def: `message Nested {
				string value = 1;
			}
			message TestResponse {
				option (groxypb.target) = true;
				Nested nested = 1 [(groxypb.value) = "{\"value\": \"{{.inputValue}}\"}"];
			}`,
			data: map[string]any{"inputValue": "templated-value"},
			validate: func(t *testing.T, msg proto.Message) {
				msgBytes, err := proto.Marshal(msg)
				require.NoError(t, err)
				assert.Contains(t, string(msgBytes), "templated-value")
			},
		},
		{
			name: "repeated field with templating",
			def: `message TestResponse {
				option (groxypb.target) = true;
				repeated string items = 1 [(groxypb.value) = "[\"{{.prefix}}-1\", \"{{.prefix}}-2\"]"];
			}`,
			data: map[string]any{"prefix": "item"},
			validate: func(t *testing.T, msg proto.Message) {
				msgBytes, err := proto.Marshal(msg)
				require.NoError(t, err)
				msgStr := string(msgBytes)
				assert.Contains(t, msgStr, "item-1")
				assert.Contains(t, msgStr, "item-2")
			},
		},
		{
			name: "multiple templated fields",
			def: `message TestResponse {
				option (groxypb.target) = true;
				string id = 1 [(groxypb.value) = "{{uuidv4}}"];
				string name = 2 [(groxypb.value) = "{{.userName}}"];
				int32 score = 3 [(groxypb.value) = "{{mul .baseScore 2}}"];
			}`,
			data: map[string]any{"userName": "testuser", "baseScore": 50},
			validate: func(t *testing.T, msg proto.Message) {
				msgBytes, err := proto.Marshal(msg)
				require.NoError(t, err)
				msgStr := string(msgBytes)
				assert.Contains(t, msgStr, "testuser")
				// Score should be 100 (50 * 2)
				assert.Contains(t, msgStr, "d") // 100 in protobuf encoding
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := BuildMessage(tt.def)
			require.NoError(t, err)

			msg, err := tmpl.Generate(context.Background(), tt.data)
			require.NoError(t, err)

			tt.validate(t, msg)
		})
	}
}

func TestTemplateErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		def     string
		data    map[string]any
		wantErr string
	}{
		{
			name: "invalid template syntax",
			def: `message TestResponse {
				option (groxypb.target) = true;
				string value = 1 [(groxypb.value) = "{{invalid template}}"];
			}`,
			wantErr: "parse template",
		},
		{
			name: "invalid matcher expression",
			def: `message TestRequest {
				option (groxypb.target) = true;
				int32 value = 1 [(groxypb.matcher) = "invalid expression"];
			}`,
			wantErr: "compile matcher",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := BuildMessage(tt.def)
			if tt.wantErr != "" && strings.Contains(tt.wantErr, "parse template") ||
				strings.Contains(tt.wantErr, "compile matcher") {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)

			_, err = tmpl.Generate(context.Background(), tt.data)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
