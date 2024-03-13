package protodef

import (
	"testing"

	"github.com/Semior001/groxy/pkg/protodef/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestSetValues(t *testing.T) {
	type want struct {
		msg proto.Message
		err error
	}
	tests := []struct {
		name string
		tmpl proto.Message
		vals map[string]any
		want want
	}{
		{
			name: "simple message",
			tmpl: &testdata.Response{},
			vals: map[string]any{"value": "test"},
			want: want{msg: &testdata.Response{Value: "test"}},
		},
		{
			name: "nested message",
			tmpl: &testdata.Response{},
			vals: map[string]any{"nested": map[string]any{"nested_value": "test"}},
			want: want{msg: &testdata.Response{Nested: &testdata.Nested{NestedValue: "test"}}},
		},
		{
			name: "nested message with repeated field",
			tmpl: &testdata.Response{},
			vals: map[string]any{"nesteds": []any{
				map[string]any{"nested_value": "test1"},
				map[string]any{"nested_value": "test2"},
			}},
			want: want{msg: &testdata.Response{Nesteds: []*testdata.Nested{
				{NestedValue: "test1"},
				{NestedValue: "test2"},
			}}},
		},
		{
			name: "enum",
			tmpl: &testdata.Response{},
			vals: map[string]any{"enum": "STUB_ENUM_SECOND"},
			want: want{msg: &testdata.Response{Enum: testdata.Enum_STUB_ENUM_SECOND}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetValues(tt.tmpl, tt.vals)
			require.ErrorIs(t, err, tt.want.err)
			assert.Equal(t, mustProtoMarshal(t, tt.want.msg), mustProtoMarshal(t, got))
		})
	}
}
