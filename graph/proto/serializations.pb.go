// Code generated by protoc-gen-go. DO NOT EDIT.
// source: github.com/cayleygraph/cayley/graph/proto/serializations.proto

package proto

import (
	fmt "fmt"
	math "math"

	pquads "github.com/cayleygraph/quad/pquads"
	proto "github.com/golang/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type LogDelta struct {
	ID                   uint64       `protobuf:"varint,1,opt,name=ID,proto3" json:"ID,omitempty"`
	Quad                 *pquads.Quad `protobuf:"bytes,2,opt,name=Quad,proto3" json:"Quad,omitempty"`
	Action               int32        `protobuf:"varint,3,opt,name=Action,proto3" json:"Action,omitempty"`
	Timestamp            int64        `protobuf:"varint,4,opt,name=Timestamp,proto3" json:"Timestamp,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *LogDelta) Reset()         { *m = LogDelta{} }
func (m *LogDelta) String() string { return proto.CompactTextString(m) }
func (*LogDelta) ProtoMessage()    {}
func (*LogDelta) Descriptor() ([]byte, []int) {
	return fileDescriptor_7f543b9ed483bad1, []int{0}
}

func (m *LogDelta) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LogDelta.Unmarshal(m, b)
}
func (m *LogDelta) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LogDelta.Marshal(b, m, deterministic)
}
func (m *LogDelta) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LogDelta.Merge(m, src)
}
func (m *LogDelta) XXX_Size() int {
	return xxx_messageInfo_LogDelta.Size(m)
}
func (m *LogDelta) XXX_DiscardUnknown() {
	xxx_messageInfo_LogDelta.DiscardUnknown(m)
}

var xxx_messageInfo_LogDelta proto.InternalMessageInfo

func (m *LogDelta) GetID() uint64 {
	if m != nil {
		return m.ID
	}
	return 0
}

func (m *LogDelta) GetQuad() *pquads.Quad {
	if m != nil {
		return m.Quad
	}
	return nil
}

func (m *LogDelta) GetAction() int32 {
	if m != nil {
		return m.Action
	}
	return 0
}

func (m *LogDelta) GetTimestamp() int64 {
	if m != nil {
		return m.Timestamp
	}
	return 0
}

type HistoryEntry struct {
	History              []uint64 `protobuf:"varint,1,rep,packed,name=History,proto3" json:"History,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *HistoryEntry) Reset()         { *m = HistoryEntry{} }
func (m *HistoryEntry) String() string { return proto.CompactTextString(m) }
func (*HistoryEntry) ProtoMessage()    {}
func (*HistoryEntry) Descriptor() ([]byte, []int) {
	return fileDescriptor_7f543b9ed483bad1, []int{1}
}

func (m *HistoryEntry) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_HistoryEntry.Unmarshal(m, b)
}
func (m *HistoryEntry) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_HistoryEntry.Marshal(b, m, deterministic)
}
func (m *HistoryEntry) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HistoryEntry.Merge(m, src)
}
func (m *HistoryEntry) XXX_Size() int {
	return xxx_messageInfo_HistoryEntry.Size(m)
}
func (m *HistoryEntry) XXX_DiscardUnknown() {
	xxx_messageInfo_HistoryEntry.DiscardUnknown(m)
}

var xxx_messageInfo_HistoryEntry proto.InternalMessageInfo

func (m *HistoryEntry) GetHistory() []uint64 {
	if m != nil {
		return m.History
	}
	return nil
}

type NodeData struct {
	Name                 string        `protobuf:"bytes,1,opt,name=Name,proto3" json:"Name,omitempty"`
	Size                 int64         `protobuf:"varint,2,opt,name=Size,proto3" json:"Size,omitempty"`
	Value                *pquads.Value `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *NodeData) Reset()         { *m = NodeData{} }
func (m *NodeData) String() string { return proto.CompactTextString(m) }
func (*NodeData) ProtoMessage()    {}
func (*NodeData) Descriptor() ([]byte, []int) {
	return fileDescriptor_7f543b9ed483bad1, []int{2}
}

func (m *NodeData) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeData.Unmarshal(m, b)
}
func (m *NodeData) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeData.Marshal(b, m, deterministic)
}
func (m *NodeData) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeData.Merge(m, src)
}
func (m *NodeData) XXX_Size() int {
	return xxx_messageInfo_NodeData.Size(m)
}
func (m *NodeData) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeData.DiscardUnknown(m)
}

var xxx_messageInfo_NodeData proto.InternalMessageInfo

func (m *NodeData) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *NodeData) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *NodeData) GetValue() *pquads.Value {
	if m != nil {
		return m.Value
	}
	return nil
}

func init() {
	proto.RegisterType((*LogDelta)(nil), "proto.LogDelta")
	proto.RegisterType((*HistoryEntry)(nil), "proto.HistoryEntry")
	proto.RegisterType((*NodeData)(nil), "proto.NodeData")
}

func init() {
	proto.RegisterFile("github.com/cayleygraph/cayley/graph/proto/serializations.proto", fileDescriptor_7f543b9ed483bad1)
}

var fileDescriptor_7f543b9ed483bad1 = []byte{
	// 270 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x8f, 0xcf, 0x4b, 0xf3, 0x30,
	0x18, 0xc7, 0x49, 0x7f, 0xec, 0xdd, 0x9e, 0xed, 0xf5, 0x90, 0x83, 0x04, 0xf1, 0x10, 0xea, 0x25,
	0xa7, 0x16, 0xe6, 0x5d, 0x10, 0x2a, 0x38, 0x90, 0x81, 0x51, 0xf4, 0xfc, 0xac, 0x0d, 0x5d, 0xa0,
	0x5d, 0x6a, 0x9a, 0x0a, 0xdd, 0x5f, 0x2f, 0x4d, 0x3b, 0x3c, 0x79, 0x49, 0xbe, 0xdf, 0xcf, 0xc3,
	0x13, 0x3e, 0x81, 0x87, 0x4a, 0xbb, 0x63, 0x7f, 0x48, 0x0b, 0xd3, 0x64, 0x05, 0x0e, 0xb5, 0x1a,
	0x2a, 0x8b, 0xed, 0x71, 0xce, 0xd9, 0x54, 0x5a, 0x6b, 0x9c, 0xc9, 0x3a, 0x65, 0x35, 0xd6, 0xfa,
	0x8c, 0x4e, 0x9b, 0x53, 0x97, 0x7a, 0x48, 0x63, 0x7f, 0xdd, 0xa4, 0x7f, 0x3c, 0xf3, 0xd5, 0x63,
	0x99, 0xb5, 0xe3, 0xd9, 0xf9, 0x3c, 0xaf, 0x25, 0x16, 0x96, 0x2f, 0xa6, 0xca, 0x55, 0xed, 0x90,
	0x5e, 0x41, 0xb0, 0xcb, 0x19, 0xe1, 0x44, 0x44, 0x32, 0xd8, 0xe5, 0x94, 0x43, 0xf4, 0xda, 0x63,
	0xc9, 0x02, 0x4e, 0xc4, 0x7a, 0xbb, 0x49, 0xa7, 0xf5, 0x74, 0x64, 0xd2, 0x4f, 0xe8, 0x35, 0x2c,
	0x1e, 0x8b, 0xd1, 0x82, 0x85, 0x9c, 0x88, 0x58, 0xce, 0x8d, 0xde, 0xc2, 0xea, 0x5d, 0x37, 0xaa,
	0x73, 0xd8, 0xb4, 0x2c, 0xe2, 0x44, 0x84, 0xf2, 0x17, 0x24, 0x02, 0x36, 0xcf, 0xba, 0x73, 0xc6,
	0x0e, 0x4f, 0x27, 0x67, 0x07, 0xca, 0xe0, 0xdf, 0xdc, 0x19, 0xe1, 0xa1, 0x88, 0xe4, 0xa5, 0x26,
	0x9f, 0xb0, 0xdc, 0x9b, 0x52, 0xe5, 0xe8, 0x90, 0x52, 0x88, 0xf6, 0xd8, 0x28, 0xef, 0xb7, 0x92,
	0x3e, 0x8f, 0xec, 0x4d, 0x9f, 0x95, 0x37, 0x0c, 0xa5, 0xcf, 0xf4, 0x0e, 0xe2, 0x6f, 0xac, 0x7b,
	0xe5, 0x95, 0xd6, 0xdb, 0xff, 0x17, 0xed, 0x8f, 0x11, 0xca, 0x69, 0x76, 0x58, 0xf8, 0xdf, 0xdf,
	0xff, 0x04, 0x00, 0x00, 0xff, 0xff, 0xbd, 0xa9, 0xd4, 0xa7, 0x76, 0x01, 0x00, 0x00,
}
