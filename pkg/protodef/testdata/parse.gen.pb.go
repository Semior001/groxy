// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v4.24.4
// source: pkg/protodef/testdata/parse.gen.proto

package testdata

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Enum int32

const (
	Enum_STUB_ENUM_UNSPECIFIED Enum = 0
	Enum_STUB_ENUM_FIRST       Enum = 1
	Enum_STUB_ENUM_SECOND      Enum = 2
)

// Enum value maps for Enum.
var (
	Enum_name = map[int32]string{
		0: "STUB_ENUM_UNSPECIFIED",
		1: "STUB_ENUM_FIRST",
		2: "STUB_ENUM_SECOND",
	}
	Enum_value = map[string]int32{
		"STUB_ENUM_UNSPECIFIED": 0,
		"STUB_ENUM_FIRST":       1,
		"STUB_ENUM_SECOND":      2,
	}
)

func (x Enum) Enum() *Enum {
	p := new(Enum)
	*p = x
	return p
}

func (x Enum) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Enum) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_protodef_testdata_parse_gen_proto_enumTypes[0].Descriptor()
}

func (Enum) Type() protoreflect.EnumType {
	return &file_pkg_protodef_testdata_parse_gen_proto_enumTypes[0]
}

func (x Enum) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Enum.Descriptor instead.
func (Enum) EnumDescriptor() ([]byte, []int) {
	return file_pkg_protodef_testdata_parse_gen_proto_rawDescGZIP(), []int{0}
}

type Request struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Value string `protobuf:"bytes,1,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *Request) Reset() {
	*x = Request{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_protodef_testdata_parse_gen_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Request) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Request) ProtoMessage() {}

func (x *Request) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_protodef_testdata_parse_gen_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Request.ProtoReflect.Descriptor instead.
func (*Request) Descriptor() ([]byte, []int) {
	return file_pkg_protodef_testdata_parse_gen_proto_rawDescGZIP(), []int{0}
}

func (x *Request) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

type Nested struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Enum        Enum   `protobuf:"varint,1,opt,name=enum,proto3,enum=groxy.runtime_generated.Enum" json:"enum,omitempty"`
	NestedValue string `protobuf:"bytes,6,opt,name=nested_value,json=nestedValue,proto3" json:"nested_value,omitempty"`
}

func (x *Nested) Reset() {
	*x = Nested{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_protodef_testdata_parse_gen_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Nested) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Nested) ProtoMessage() {}

func (x *Nested) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_protodef_testdata_parse_gen_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Nested.ProtoReflect.Descriptor instead.
func (*Nested) Descriptor() ([]byte, []int) {
	return file_pkg_protodef_testdata_parse_gen_proto_rawDescGZIP(), []int{1}
}

func (x *Nested) GetEnum() Enum {
	if x != nil {
		return x.Enum
	}
	return Enum_STUB_ENUM_UNSPECIFIED
}

func (x *Nested) GetNestedValue() string {
	if x != nil {
		return x.NestedValue
	}
	return ""
}

type Response struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Nested    *Nested            `protobuf:"bytes,3,opt,name=nested,proto3" json:"nested,omitempty"`
	Enum      Enum               `protobuf:"varint,9,opt,name=enum,proto3,enum=groxy.runtime_generated.Enum" json:"enum,omitempty"`
	Nesteds   []*Nested          `protobuf:"bytes,10,rep,name=nesteds,proto3" json:"nesteds,omitempty"`
	NestedMap map[string]*Nested `protobuf:"bytes,11,rep,name=nested_map,json=nestedMap,proto3" json:"nested_map,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Value     string             `protobuf:"bytes,12,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *Response) Reset() {
	*x = Response{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_protodef_testdata_parse_gen_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Response) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Response) ProtoMessage() {}

func (x *Response) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_protodef_testdata_parse_gen_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Response.ProtoReflect.Descriptor instead.
func (*Response) Descriptor() ([]byte, []int) {
	return file_pkg_protodef_testdata_parse_gen_proto_rawDescGZIP(), []int{2}
}

func (x *Response) GetNested() *Nested {
	if x != nil {
		return x.Nested
	}
	return nil
}

func (x *Response) GetEnum() Enum {
	if x != nil {
		return x.Enum
	}
	return Enum_STUB_ENUM_UNSPECIFIED
}

func (x *Response) GetNesteds() []*Nested {
	if x != nil {
		return x.Nesteds
	}
	return nil
}

func (x *Response) GetNestedMap() map[string]*Nested {
	if x != nil {
		return x.NestedMap
	}
	return nil
}

func (x *Response) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

var File_pkg_protodef_testdata_parse_gen_proto protoreflect.FileDescriptor

var file_pkg_protodef_testdata_parse_gen_proto_rawDesc = []byte{
	0x0a, 0x25, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x65, 0x66, 0x2f, 0x74,
	0x65, 0x73, 0x74, 0x64, 0x61, 0x74, 0x61, 0x2f, 0x70, 0x61, 0x72, 0x73, 0x65, 0x2e, 0x67, 0x65,
	0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x17, 0x67, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x72,
	0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64,
	0x22, 0x1f, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x22, 0x5e, 0x0a, 0x06, 0x4e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x12, 0x31, 0x0a, 0x04, 0x65,
	0x6e, 0x75, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x1d, 0x2e, 0x67, 0x72, 0x6f, 0x78,
	0x79, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61,
	0x74, 0x65, 0x64, 0x2e, 0x45, 0x6e, 0x75, 0x6d, 0x52, 0x04, 0x65, 0x6e, 0x75, 0x6d, 0x12, 0x21,
	0x0a, 0x0c, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x06,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x22, 0x8f, 0x03, 0x0a, 0x08, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x37,
	0x0a, 0x06, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f,
	0x2e, 0x67, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x67,
	0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x4e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x52,
	0x06, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x12, 0x31, 0x0a, 0x04, 0x65, 0x6e, 0x75, 0x6d, 0x18,
	0x09, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x1d, 0x2e, 0x67, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x72, 0x75,
	0x6e, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e,
	0x45, 0x6e, 0x75, 0x6d, 0x52, 0x04, 0x65, 0x6e, 0x75, 0x6d, 0x12, 0x39, 0x0a, 0x07, 0x6e, 0x65,
	0x73, 0x74, 0x65, 0x64, 0x73, 0x18, 0x0a, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x67, 0x72,
	0x6f, 0x78, 0x79, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x67, 0x65, 0x6e, 0x65,
	0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x4e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x52, 0x07, 0x6e, 0x65,
	0x73, 0x74, 0x65, 0x64, 0x73, 0x12, 0x4f, 0x0a, 0x0a, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f,
	0x6d, 0x61, 0x70, 0x18, 0x0b, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x30, 0x2e, 0x67, 0x72, 0x6f, 0x78,
	0x79, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61,
	0x74, 0x65, 0x64, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2e, 0x4e, 0x65, 0x73,
	0x74, 0x65, 0x64, 0x4d, 0x61, 0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x09, 0x6e, 0x65, 0x73,
	0x74, 0x65, 0x64, 0x4d, 0x61, 0x70, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x0c, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x1a, 0x5d, 0x0a, 0x0e,
	0x4e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x4d, 0x61, 0x70, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10,
	0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79,
	0x12, 0x35, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1f, 0x2e, 0x67, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x5f,
	0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x4e, 0x65, 0x73, 0x74, 0x65, 0x64,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x4a, 0x04, 0x08, 0x04, 0x10,
	0x05, 0x4a, 0x04, 0x08, 0x05, 0x10, 0x06, 0x4a, 0x04, 0x08, 0x07, 0x10, 0x08, 0x4a, 0x04, 0x08,
	0x08, 0x10, 0x09, 0x2a, 0x4c, 0x0a, 0x04, 0x45, 0x6e, 0x75, 0x6d, 0x12, 0x19, 0x0a, 0x15, 0x53,
	0x54, 0x55, 0x42, 0x5f, 0x45, 0x4e, 0x55, 0x4d, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49,
	0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x13, 0x0a, 0x0f, 0x53, 0x54, 0x55, 0x42, 0x5f, 0x45,
	0x4e, 0x55, 0x4d, 0x5f, 0x46, 0x49, 0x52, 0x53, 0x54, 0x10, 0x01, 0x12, 0x14, 0x0a, 0x10, 0x53,
	0x54, 0x55, 0x42, 0x5f, 0x45, 0x4e, 0x55, 0x4d, 0x5f, 0x53, 0x45, 0x43, 0x4f, 0x4e, 0x44, 0x10,
	0x02, 0x32, 0x62, 0x0a, 0x0b, 0x54, 0x65, 0x73, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x12, 0x53, 0x0a, 0x0a, 0x54, 0x65, 0x73, 0x74, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x12, 0x20,
	0x2e, 0x67, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x67,
	0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x21, 0x2e, 0x67, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65,
	0x5f, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x00, 0x32, 0x63, 0x0a, 0x0c, 0x4f, 0x74, 0x68, 0x65, 0x72, 0x53, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x53, 0x0a, 0x0b, 0x4f, 0x74, 0x68, 0x65, 0x72, 0x4d, 0x65,
	0x74, 0x68, 0x6f, 0x64, 0x12, 0x1f, 0x2e, 0x67, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x72, 0x75, 0x6e,
	0x74, 0x69, 0x6d, 0x65, 0x5f, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x4e,
	0x65, 0x73, 0x74, 0x65, 0x64, 0x1a, 0x21, 0x2e, 0x67, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x72, 0x75,
	0x6e, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x2e,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x3b, 0x5a, 0x39, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x53, 0x65, 0x6d, 0x69, 0x6f, 0x72, 0x30,
	0x30, 0x31, 0x2f, 0x67, 0x72, 0x6f, 0x78, 0x79, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x64, 0x65, 0x66, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x64, 0x61, 0x74, 0x61, 0x3b, 0x74,
	0x65, 0x73, 0x74, 0x64, 0x61, 0x74, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_protodef_testdata_parse_gen_proto_rawDescOnce sync.Once
	file_pkg_protodef_testdata_parse_gen_proto_rawDescData = file_pkg_protodef_testdata_parse_gen_proto_rawDesc
)

func file_pkg_protodef_testdata_parse_gen_proto_rawDescGZIP() []byte {
	file_pkg_protodef_testdata_parse_gen_proto_rawDescOnce.Do(func() {
		file_pkg_protodef_testdata_parse_gen_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_protodef_testdata_parse_gen_proto_rawDescData)
	})
	return file_pkg_protodef_testdata_parse_gen_proto_rawDescData
}

var file_pkg_protodef_testdata_parse_gen_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pkg_protodef_testdata_parse_gen_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_pkg_protodef_testdata_parse_gen_proto_goTypes = []interface{}{
	(Enum)(0),        // 0: groxy.runtime_generated.Enum
	(*Request)(nil),  // 1: groxy.runtime_generated.Request
	(*Nested)(nil),   // 2: groxy.runtime_generated.Nested
	(*Response)(nil), // 3: groxy.runtime_generated.Response
	nil,              // 4: groxy.runtime_generated.Response.NestedMapEntry
}
var file_pkg_protodef_testdata_parse_gen_proto_depIdxs = []int32{
	0, // 0: groxy.runtime_generated.Nested.enum:type_name -> groxy.runtime_generated.Enum
	2, // 1: groxy.runtime_generated.Response.nested:type_name -> groxy.runtime_generated.Nested
	0, // 2: groxy.runtime_generated.Response.enum:type_name -> groxy.runtime_generated.Enum
	2, // 3: groxy.runtime_generated.Response.nesteds:type_name -> groxy.runtime_generated.Nested
	4, // 4: groxy.runtime_generated.Response.nested_map:type_name -> groxy.runtime_generated.Response.NestedMapEntry
	2, // 5: groxy.runtime_generated.Response.NestedMapEntry.value:type_name -> groxy.runtime_generated.Nested
	1, // 6: groxy.runtime_generated.TestService.TestMethod:input_type -> groxy.runtime_generated.Request
	2, // 7: groxy.runtime_generated.OtherService.OtherMethod:input_type -> groxy.runtime_generated.Nested
	3, // 8: groxy.runtime_generated.TestService.TestMethod:output_type -> groxy.runtime_generated.Response
	3, // 9: groxy.runtime_generated.OtherService.OtherMethod:output_type -> groxy.runtime_generated.Response
	8, // [8:10] is the sub-list for method output_type
	6, // [6:8] is the sub-list for method input_type
	6, // [6:6] is the sub-list for extension type_name
	6, // [6:6] is the sub-list for extension extendee
	0, // [0:6] is the sub-list for field type_name
}

func init() { file_pkg_protodef_testdata_parse_gen_proto_init() }
func file_pkg_protodef_testdata_parse_gen_proto_init() {
	if File_pkg_protodef_testdata_parse_gen_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_protodef_testdata_parse_gen_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Request); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_protodef_testdata_parse_gen_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Nested); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_protodef_testdata_parse_gen_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Response); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_protodef_testdata_parse_gen_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_pkg_protodef_testdata_parse_gen_proto_goTypes,
		DependencyIndexes: file_pkg_protodef_testdata_parse_gen_proto_depIdxs,
		EnumInfos:         file_pkg_protodef_testdata_parse_gen_proto_enumTypes,
		MessageInfos:      file_pkg_protodef_testdata_parse_gen_proto_msgTypes,
	}.Build()
	File_pkg_protodef_testdata_parse_gen_proto = out.File
	file_pkg_protodef_testdata_parse_gen_proto_rawDesc = nil
	file_pkg_protodef_testdata_parse_gen_proto_goTypes = nil
	file_pkg_protodef_testdata_parse_gen_proto_depIdxs = nil
}
