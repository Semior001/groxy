package protodef

import (
	"testing"

	"github.com/Semior001/groxy/pkg/protodef/testdata"
	"github.com/jhump/protoreflect/desc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type fdMock struct {
	typ            descriptorpb.FieldDescriptorProto_Type
	isMap          bool
	isRepeated     bool
	keyTyp, valTyp descriptorpb.FieldDescriptorProto_Type
	msg            *desc.MessageDescriptor
	enum           *desc.EnumDescriptor
}

func (f fdMock) GetType() descriptorpb.FieldDescriptorProto_Type { return f.typ }
func (f fdMock) IsMap() bool                                     { return f.isMap }
func (f fdMock) IsRepeated() bool                                { return f.isRepeated }
func (f fdMock) GetMapKeyType() fieldDescriptor                  { return fdMock{typ: f.keyTyp} }
func (f fdMock) GetMapValueType() fieldDescriptor                { return fdMock{typ: f.valTyp} }
func (f fdMock) GetMessageType() *desc.MessageDescriptor         { return f.msg }
func (f fdMock) GetEnumType() *desc.EnumDescriptor               { return f.enum }

func Test_builder_parseValue(t *testing.T) {
	tests := []struct {
		name string
		fd   fdMock
		val  string
		want any
	}{
		{
			name: "string",
			fd:   fdMock{typ: descriptorpb.FieldDescriptorProto_TYPE_STRING},
			val:  "Hello, World!",
			want: "Hello, World!",
		},
		{
			name: "int32",
			fd:   fdMock{typ: descriptorpb.FieldDescriptorProto_TYPE_INT32},
			val:  "42",
			want: int32(42),
		},
		{
			name: "repeated int32",
			fd:   fdMock{typ: descriptorpb.FieldDescriptorProto_TYPE_INT32, isRepeated: true},
			val:  "[42, 314]",
			want: []int32{42, 314},
		},
		{
			name: "map string -> int32",
			fd: fdMock{isMap: true,
				keyTyp: descriptorpb.FieldDescriptorProto_TYPE_STRING,
				valTyp: descriptorpb.FieldDescriptorProto_TYPE_INT32,
			},
			val:  `{"key": 42}`,
			want: map[string]int32{"key": 42},
		},
		{
			name: "map int32 -> string",
			fd: fdMock{isMap: true,
				keyTyp: descriptorpb.FieldDescriptorProto_TYPE_INT32,
				valTyp: descriptorpb.FieldDescriptorProto_TYPE_STRING,
			},
			val:  `{42: "value"}`,
			want: map[int32]string{42: "value"},
		},
		{
			name: "map int32 -> string",
			fd: fdMock{isMap: true,
				keyTyp: descriptorpb.FieldDescriptorProto_TYPE_INT32,
				valTyp: descriptorpb.FieldDescriptorProto_TYPE_STRING,
			},
			val:  `{42: "value"}`,
			want: map[int32]string{42: "value"},
		},
		{
			name: "bytes",
			fd:   fdMock{typ: descriptorpb.FieldDescriptorProto_TYPE_BYTES},
			val:  "SGVsbG8sIFdvcmxkIQ==",
			want: []byte("Hello, World!"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (&Definer{}).buildValue(tt.fd, tt.val)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildMessage(t *testing.T) {
	t.Run("map, value in target", func(t *testing.T) {
		const def = `
				message Nested { string value = 6; }
				message StubResponse {
					option (groxypb.target) = true;
					map<string, Nested> map = 11 [(groxypb.value) = '{"key": {"value": "Hello, World!"}, "key2": {"value": "Hello, World2!"}}'];
				}`
		want := &testdata.Response{
			NestedMap: map[string]*testdata.Nested{
				"key":  {NestedValue: "Hello, World!"},
				"key2": {NestedValue: "Hello, World2!"},
			},
		}

		gotDyn, err := BuildMessage(def)
		require.NoError(t, err)
		got := &testdata.Response{}
		require.NoError(t, proto.Unmarshal(mustProtoMarshal(t, gotDyn), got))
		assert.Truef(t, proto.Equal(want, got),
			"expected: %v\nactual: %v", want.String(), got.String())
	})

	tests := []struct {
		name string
		def  string
		want proto.Message
	}{
		{
			name: "with nested message, value defined in target",
			def: `	message Nested { string value = 6; } 
					message StubResponse {
						option (groxypb.target) = true;
						Nested nested = 3 [(groxypb.value) = '{"value": "Hello, World!"}'];
					}`,
			want: &testdata.Response{Nested: &testdata.Nested{NestedValue: "Hello, World!"}},
		},
		{
			name: "with nested message, value defined in target AND in message type, target value has priority",
			def: `	message Nested { 
						string value = 6 [(groxypb.value) = 'Hello, World!'];
					} 
					message StubResponse {
						option (groxypb.target) = true;
						Nested nested = 3 [(groxypb.value) = '{"value": "Prioritized"}'];
					}`,
			want: &testdata.Response{Nested: &testdata.Nested{NestedValue: "Prioritized"}},
		},
		{
			name: "with nested message, value defined in message type",
			def: `	message Nested {
						string value = 6 [(groxypb.value) = 'Hello, World!'];
					}
					message StubResponse {	
						option (groxypb.target) = true;	
						Nested nested = 3;
					}`,
			want: &testdata.Response{Nested: &testdata.Nested{NestedValue: "Hello, World!"}},
		},
		{
			name: "with enum",
			def: `	enum SomeEnum { EMPTY = 0; NEEDED_VALUE = 2; }
					message StubResponse {
						option (groxypb.target) = true;
						SomeEnum some_enum = 9 [(groxypb.value) = 'NEEDED_VALUE'];
					}`,
			want: &testdata.Response{
				Enum: testdata.Enum_STUB_ENUM_SECOND,
			},
		},
		{
			name: "with enum in nested message, value in target",
			def: `	enum SomeEnum { EMPTY = 0; NEEDED_VALUE = 2; }
					message Nested {
						SomeEnum some_enum = 1;
						string value = 6;
					}
					message StubResponse {
						option (groxypb.target) = true;
						Nested nested = 3 [(groxypb.value) = '{"some_enum": "NEEDED_VALUE", "value": "Hello, World!"}'];
					}`,
			want: &testdata.Response{Nested: &testdata.Nested{
				Enum:        testdata.Enum_STUB_ENUM_SECOND,
				NestedValue: "Hello, World!",
			}},
		},
		{
			name: "repeated nested message, value in target",
			def: `	message Nested { string value = 6; }
					message StubResponse {
						option (groxypb.target) = true;
						repeated Nested nested = 10 [(groxypb.value) = '[{"value": "first"}, {"value": "second"}]'];
					}`,
			want: &testdata.Response{Nesteds: []*testdata.Nested{
				{NestedValue: "first"},
				{NestedValue: "second"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildMessage(tt.def)
			require.NoError(t, err)
			assert.Equal(t, mustProtoMarshal(t, tt.want), mustProtoMarshal(t, got))
		})
	}
}

func Test_parse(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fd, err := (&Definer{}).parseDefinition(testdata.String(t, "parse.gen.proto"))
		require.NoError(t, err)

		assertFileDescEqual(t, testdata.File_pkg_protodef_testdata_parse_gen_proto, fd)
	})
}

func assertFileDescEqual(t *testing.T, expected protoreflect.FileDescriptor, actual *desc.FileDescriptor) {
	expectedDesc, err := desc.WrapFile(expected)
	require.NoError(t, err)

	fdp := func(desc *desc.FileDescriptor) *descriptorpb.FileDescriptorProto {
		fdp := desc.AsFileDescriptorProto()
		fdp.Name = nil // clean source code file name
		return fdp
	}

	expectedFDP, gotFDP := fdp(expectedDesc), fdp(actual)
	assert.Truef(t, proto.Equal(expectedFDP, gotFDP),
		"expected: %v\nactual: %v", expectedFDP, gotFDP)
}

func mustProtoMarshal(t *testing.T, msg proto.Message) []byte {
	t.Helper()

	b, err := proto.Marshal(msg)
	require.NoError(t, err)
	return b
}
