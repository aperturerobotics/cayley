// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v3.21.9
// source: github.com/cayleygraph/cayley/quad/pquads/pio/test/test.proto

package test

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// TestMsg is a test message used for unit tests.
type TestMsg struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Value int32 `protobuf:"varint,1,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *TestMsg) Reset() {
	*x = TestMsg{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TestMsg) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestMsg) ProtoMessage() {}

func (x *TestMsg) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestMsg.ProtoReflect.Descriptor instead.
func (*TestMsg) Descriptor() ([]byte, []int) {
	return file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDescGZIP(), []int{0}
}

func (x *TestMsg) GetValue() int32 {
	if x != nil {
		return x.Value
	}
	return 0
}

var File_github_com_cayleygraph_quad_pquads_pio_test_test_proto protoreflect.FileDescriptor

var file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDesc = []byte{
	0x0a, 0x36, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x61, 0x79,
	0x6c, 0x65, 0x79, 0x67, 0x72, 0x61, 0x70, 0x68, 0x2f, 0x71, 0x75, 0x61, 0x64, 0x2f, 0x70, 0x71,
	0x75, 0x61, 0x64, 0x73, 0x2f, 0x70, 0x69, 0x6f, 0x2f, 0x74, 0x65, 0x73, 0x74, 0x2f, 0x74, 0x65,
	0x73, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x08, 0x70, 0x69, 0x6f, 0x2e, 0x74, 0x65,
	0x73, 0x74, 0x22, 0x1f, 0x0a, 0x07, 0x54, 0x65, 0x73, 0x74, 0x4d, 0x73, 0x67, 0x12, 0x14, 0x0a,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x42, 0x2d, 0x5a, 0x2b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x63, 0x61, 0x79, 0x6c, 0x65, 0x79, 0x67, 0x72, 0x61, 0x70, 0x68, 0x2f, 0x71, 0x75,
	0x61, 0x64, 0x2f, 0x70, 0x71, 0x75, 0x61, 0x64, 0x73, 0x2f, 0x70, 0x69, 0x6f, 0x2f, 0x74, 0x65,
	0x73, 0x74, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDescOnce sync.Once
	file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDescData = file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDesc
)

func file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDescGZIP() []byte {
	file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDescOnce.Do(func() {
		file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDescData = protoimpl.X.CompressGZIP(file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDescData)
	})
	return file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDescData
}

var file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_goTypes = []interface{}{
	(*TestMsg)(nil), // 0: pio.test.TestMsg
}
var file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_init() }
func file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_init() {
	if File_github_com_cayleygraph_quad_pquads_pio_test_test_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TestMsg); i {
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
			RawDescriptor: file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_goTypes,
		DependencyIndexes: file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_depIdxs,
		MessageInfos:      file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_msgTypes,
	}.Build()
	File_github_com_cayleygraph_quad_pquads_pio_test_test_proto = out.File
	file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_rawDesc = nil
	file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_goTypes = nil
	file_github_com_cayleygraph_quad_pquads_pio_test_test_proto_depIdxs = nil
}
