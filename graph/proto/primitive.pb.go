// Copyright 2016 The Cayley Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0-devel
// 	protoc        v3.21.9
// source: github.com/cayleygraph/cayley/graph/proto/primitive.proto

package proto

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

type PrimitiveType int32

const (
	PrimitiveType_LINK      PrimitiveType = 0
	PrimitiveType_IRI       PrimitiveType = 1
	PrimitiveType_STRING    PrimitiveType = 2
	PrimitiveType_BNODE     PrimitiveType = 3
	PrimitiveType_TYPED_STR PrimitiveType = 4
	PrimitiveType_LANG_STR  PrimitiveType = 5
	PrimitiveType_INT       PrimitiveType = 6
	PrimitiveType_FLOAT     PrimitiveType = 7
	PrimitiveType_BOOL      PrimitiveType = 8
	PrimitiveType_TIMESTAMP PrimitiveType = 9
)

// Enum value maps for PrimitiveType.
var (
	PrimitiveType_name = map[int32]string{
		0: "LINK",
		1: "IRI",
		2: "STRING",
		3: "BNODE",
		4: "TYPED_STR",
		5: "LANG_STR",
		6: "INT",
		7: "FLOAT",
		8: "BOOL",
		9: "TIMESTAMP",
	}
	PrimitiveType_value = map[string]int32{
		"LINK":      0,
		"IRI":       1,
		"STRING":    2,
		"BNODE":     3,
		"TYPED_STR": 4,
		"LANG_STR":  5,
		"INT":       6,
		"FLOAT":     7,
		"BOOL":      8,
		"TIMESTAMP": 9,
	}
)

func (x PrimitiveType) Enum() *PrimitiveType {
	p := new(PrimitiveType)
	*p = x
	return p
}

func (x PrimitiveType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (PrimitiveType) Descriptor() protoreflect.EnumDescriptor {
	return file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_enumTypes[0].Descriptor()
}

func (PrimitiveType) Type() protoreflect.EnumType {
	return &file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_enumTypes[0]
}

func (x PrimitiveType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use PrimitiveType.Descriptor instead.
func (PrimitiveType) EnumDescriptor() ([]byte, []int) {
	return file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDescGZIP(), []int{0}
}

type Primitive struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ID        uint64 `protobuf:"varint,1,opt,name=ID,proto3" json:"ID,omitempty"`
	Subject   uint64 `protobuf:"varint,2,opt,name=Subject,proto3" json:"Subject,omitempty"`
	Predicate uint64 `protobuf:"varint,3,opt,name=Predicate,proto3" json:"Predicate,omitempty"`
	Object    uint64 `protobuf:"varint,4,opt,name=Object,proto3" json:"Object,omitempty"`
	Label     uint64 `protobuf:"varint,5,opt,name=Label,proto3" json:"Label,omitempty"`
	Replaces  uint64 `protobuf:"varint,6,opt,name=Replaces,proto3" json:"Replaces,omitempty"`
	Value     []byte `protobuf:"bytes,8,opt,name=Value,proto3" json:"Value,omitempty"`
	Deleted   bool   `protobuf:"varint,9,opt,name=Deleted,proto3" json:"Deleted,omitempty"`
}

func (x *Primitive) Reset() {
	*x = Primitive{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Primitive) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Primitive) ProtoMessage() {}

func (x *Primitive) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Primitive.ProtoReflect.Descriptor instead.
func (*Primitive) Descriptor() ([]byte, []int) {
	return file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDescGZIP(), []int{0}
}

func (x *Primitive) GetID() uint64 {
	if x != nil {
		return x.ID
	}
	return 0
}

func (x *Primitive) GetSubject() uint64 {
	if x != nil {
		return x.Subject
	}
	return 0
}

func (x *Primitive) GetPredicate() uint64 {
	if x != nil {
		return x.Predicate
	}
	return 0
}

func (x *Primitive) GetObject() uint64 {
	if x != nil {
		return x.Object
	}
	return 0
}

func (x *Primitive) GetLabel() uint64 {
	if x != nil {
		return x.Label
	}
	return 0
}

func (x *Primitive) GetReplaces() uint64 {
	if x != nil {
		return x.Replaces
	}
	return 0
}

func (x *Primitive) GetValue() []byte {
	if x != nil {
		return x.Value
	}
	return nil
}

func (x *Primitive) GetDeleted() bool {
	if x != nil {
		return x.Deleted
	}
	return false
}

var File_github_com_cayleygraph_cayley_graph_proto_primitive_proto protoreflect.FileDescriptor

var file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDesc = []byte{
	0x0a, 0x39, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x61, 0x79,
	0x6c, 0x65, 0x79, 0x67, 0x72, 0x61, 0x70, 0x68, 0x2f, 0x63, 0x61, 0x79, 0x6c, 0x65, 0x79, 0x2f,
	0x67, 0x72, 0x61, 0x70, 0x68, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x72, 0x69, 0x6d,
	0x69, 0x74, 0x69, 0x76, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0xcd, 0x01, 0x0a, 0x09, 0x50, 0x72, 0x69, 0x6d, 0x69, 0x74, 0x69, 0x76, 0x65,
	0x12, 0x0e, 0x0a, 0x02, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x02, 0x49, 0x44,
	0x12, 0x18, 0x0a, 0x07, 0x53, 0x75, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x07, 0x53, 0x75, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x50, 0x72,
	0x65, 0x64, 0x69, 0x63, 0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x50,
	0x72, 0x65, 0x64, 0x69, 0x63, 0x61, 0x74, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x4f, 0x62, 0x6a, 0x65,
	0x63, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06, 0x4f, 0x62, 0x6a, 0x65, 0x63, 0x74,
	0x12, 0x14, 0x0a, 0x05, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x18, 0x05, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x05, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x12, 0x1a, 0x0a, 0x08, 0x52, 0x65, 0x70, 0x6c, 0x61, 0x63,
	0x65, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x04, 0x52, 0x08, 0x52, 0x65, 0x70, 0x6c, 0x61, 0x63,
	0x65, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x05, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x44, 0x65, 0x6c, 0x65,
	0x74, 0x65, 0x64, 0x18, 0x09, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x44, 0x65, 0x6c, 0x65, 0x74,
	0x65, 0x64, 0x2a, 0x83, 0x01, 0x0a, 0x0d, 0x50, 0x72, 0x69, 0x6d, 0x69, 0x74, 0x69, 0x76, 0x65,
	0x54, 0x79, 0x70, 0x65, 0x12, 0x08, 0x0a, 0x04, 0x4c, 0x49, 0x4e, 0x4b, 0x10, 0x00, 0x12, 0x07,
	0x0a, 0x03, 0x49, 0x52, 0x49, 0x10, 0x01, 0x12, 0x0a, 0x0a, 0x06, 0x53, 0x54, 0x52, 0x49, 0x4e,
	0x47, 0x10, 0x02, 0x12, 0x09, 0x0a, 0x05, 0x42, 0x4e, 0x4f, 0x44, 0x45, 0x10, 0x03, 0x12, 0x0d,
	0x0a, 0x09, 0x54, 0x59, 0x50, 0x45, 0x44, 0x5f, 0x53, 0x54, 0x52, 0x10, 0x04, 0x12, 0x0c, 0x0a,
	0x08, 0x4c, 0x41, 0x4e, 0x47, 0x5f, 0x53, 0x54, 0x52, 0x10, 0x05, 0x12, 0x07, 0x0a, 0x03, 0x49,
	0x4e, 0x54, 0x10, 0x06, 0x12, 0x09, 0x0a, 0x05, 0x46, 0x4c, 0x4f, 0x41, 0x54, 0x10, 0x07, 0x12,
	0x08, 0x0a, 0x04, 0x42, 0x4f, 0x4f, 0x4c, 0x10, 0x08, 0x12, 0x0d, 0x0a, 0x09, 0x54, 0x49, 0x4d,
	0x45, 0x53, 0x54, 0x41, 0x4d, 0x50, 0x10, 0x09, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDescOnce sync.Once
	file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDescData = file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDesc
)

func file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDescGZIP() []byte {
	file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDescOnce.Do(func() {
		file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDescData = protoimpl.X.CompressGZIP(file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDescData)
	})
	return file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDescData
}

var file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_goTypes = []interface{}{
	(PrimitiveType)(0), // 0: proto.PrimitiveType
	(*Primitive)(nil),  // 1: proto.Primitive
}
var file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_init() }
func file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_init() {
	if File_github_com_cayleygraph_cayley_graph_proto_primitive_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Primitive); i {
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
			RawDescriptor: file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_goTypes,
		DependencyIndexes: file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_depIdxs,
		EnumInfos:         file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_enumTypes,
		MessageInfos:      file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_msgTypes,
	}.Build()
	File_github_com_cayleygraph_cayley_graph_proto_primitive_proto = out.File
	file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_rawDesc = nil
	file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_goTypes = nil
	file_github_com_cayleygraph_cayley_graph_proto_primitive_proto_depIdxs = nil
}
