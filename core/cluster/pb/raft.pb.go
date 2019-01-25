// Code generated by protoc-gen-go. DO NOT EDIT.
// source: raft.proto

package cluster

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	math "math"
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

// Log entries are replicated to all members of the Raft cluster
// and form the heart of the replicated state machine.
type Log struct {
	// Index holds the index of the log entry.
	Index uint64 `protobuf:"varint,1,opt,name=Index,proto3" json:"Index,omitempty"`
	// Term holds the election term of the log entry.
	Term uint64 `protobuf:"varint,2,opt,name=Term,proto3" json:"Term,omitempty"`
	// Type holds the type of the log entry.
	Type uint32 `protobuf:"varint,3,opt,name=Type,proto3" json:"Type,omitempty"`
	// Data holds the log entry's type-specific data.
	Data []byte `protobuf:"bytes,4,opt,name=Data,proto3" json:"Data,omitempty"`
	// peer is not exported since it is not transmitted, only used
	// internally to construct the Data field.
	Peer                 string   `protobuf:"bytes,5,opt,name=peer,proto3" json:"peer,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Log) Reset()         { *m = Log{} }
func (m *Log) String() string { return proto.CompactTextString(m) }
func (*Log) ProtoMessage()    {}
func (*Log) Descriptor() ([]byte, []int) {
	return fileDescriptor_b042552c306ae59b, []int{0}
}

func (m *Log) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Log.Unmarshal(m, b)
}
func (m *Log) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Log.Marshal(b, m, deterministic)
}
func (m *Log) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Log.Merge(m, src)
}
func (m *Log) XXX_Size() int {
	return xxx_messageInfo_Log.Size(m)
}
func (m *Log) XXX_DiscardUnknown() {
	xxx_messageInfo_Log.DiscardUnknown(m)
}

var xxx_messageInfo_Log proto.InternalMessageInfo

func (m *Log) GetIndex() uint64 {
	if m != nil {
		return m.Index
	}
	return 0
}

func (m *Log) GetTerm() uint64 {
	if m != nil {
		return m.Term
	}
	return 0
}

func (m *Log) GetType() uint32 {
	if m != nil {
		return m.Type
	}
	return 0
}

func (m *Log) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *Log) GetPeer() string {
	if m != nil {
		return m.Peer
	}
	return ""
}

// AppendEntriesRequest is the command used to append entries to the
// replicated log.
type AppendEntriesRequest struct {
	// Provide the current term and leader
	Term   uint64 `protobuf:"varint,1,opt,name=Term,proto3" json:"Term,omitempty"`
	Leader []byte `protobuf:"bytes,2,opt,name=Leader,proto3" json:"Leader,omitempty"`
	// Provide the previous entries for integrity checking
	PrevLogEntry uint64 `protobuf:"varint,3,opt,name=PrevLogEntry,proto3" json:"PrevLogEntry,omitempty"`
	PrevLogTerm  uint64 `protobuf:"varint,4,opt,name=PrevLogTerm,proto3" json:"PrevLogTerm,omitempty"`
	// New entries to commit
	Entries []*Log `protobuf:"bytes,5,rep,name=Entries,proto3" json:"Entries,omitempty"`
	// Commit index on the leader
	LeaderCommitIndex    uint64   `protobuf:"varint,6,opt,name=LeaderCommitIndex,proto3" json:"LeaderCommitIndex,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AppendEntriesRequest) Reset()         { *m = AppendEntriesRequest{} }
func (m *AppendEntriesRequest) String() string { return proto.CompactTextString(m) }
func (*AppendEntriesRequest) ProtoMessage()    {}
func (*AppendEntriesRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_b042552c306ae59b, []int{1}
}

func (m *AppendEntriesRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AppendEntriesRequest.Unmarshal(m, b)
}
func (m *AppendEntriesRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AppendEntriesRequest.Marshal(b, m, deterministic)
}
func (m *AppendEntriesRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AppendEntriesRequest.Merge(m, src)
}
func (m *AppendEntriesRequest) XXX_Size() int {
	return xxx_messageInfo_AppendEntriesRequest.Size(m)
}
func (m *AppendEntriesRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_AppendEntriesRequest.DiscardUnknown(m)
}

var xxx_messageInfo_AppendEntriesRequest proto.InternalMessageInfo

func (m *AppendEntriesRequest) GetTerm() uint64 {
	if m != nil {
		return m.Term
	}
	return 0
}

func (m *AppendEntriesRequest) GetLeader() []byte {
	if m != nil {
		return m.Leader
	}
	return nil
}

func (m *AppendEntriesRequest) GetPrevLogEntry() uint64 {
	if m != nil {
		return m.PrevLogEntry
	}
	return 0
}

func (m *AppendEntriesRequest) GetPrevLogTerm() uint64 {
	if m != nil {
		return m.PrevLogTerm
	}
	return 0
}

func (m *AppendEntriesRequest) GetEntries() []*Log {
	if m != nil {
		return m.Entries
	}
	return nil
}

func (m *AppendEntriesRequest) GetLeaderCommitIndex() uint64 {
	if m != nil {
		return m.LeaderCommitIndex
	}
	return 0
}

// AppendEntriesResponse is the response returned from an
// AppendEntriesRequest.
type AppendEntriesResponse struct {
	// Newer term if leader is out of date
	Term uint64 `protobuf:"varint,1,opt,name=Term,proto3" json:"Term,omitempty"`
	// Last Log is a hint to help accelerate rebuilding slow nodes
	LastLog uint64 `protobuf:"varint,2,opt,name=LastLog,proto3" json:"LastLog,omitempty"`
	// We may not succeed if we have a conflicting entry
	Success bool `protobuf:"varint,3,opt,name=Success,proto3" json:"Success,omitempty"`
	// There are scenarios where this request didn't succeed
	// but there's no need to wait/back-off the next attempt.
	NoRetryBackoff       bool     `protobuf:"varint,4,opt,name=NoRetryBackoff,proto3" json:"NoRetryBackoff,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AppendEntriesResponse) Reset()         { *m = AppendEntriesResponse{} }
func (m *AppendEntriesResponse) String() string { return proto.CompactTextString(m) }
func (*AppendEntriesResponse) ProtoMessage()    {}
func (*AppendEntriesResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_b042552c306ae59b, []int{2}
}

func (m *AppendEntriesResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AppendEntriesResponse.Unmarshal(m, b)
}
func (m *AppendEntriesResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AppendEntriesResponse.Marshal(b, m, deterministic)
}
func (m *AppendEntriesResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AppendEntriesResponse.Merge(m, src)
}
func (m *AppendEntriesResponse) XXX_Size() int {
	return xxx_messageInfo_AppendEntriesResponse.Size(m)
}
func (m *AppendEntriesResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_AppendEntriesResponse.DiscardUnknown(m)
}

var xxx_messageInfo_AppendEntriesResponse proto.InternalMessageInfo

func (m *AppendEntriesResponse) GetTerm() uint64 {
	if m != nil {
		return m.Term
	}
	return 0
}

func (m *AppendEntriesResponse) GetLastLog() uint64 {
	if m != nil {
		return m.LastLog
	}
	return 0
}

func (m *AppendEntriesResponse) GetSuccess() bool {
	if m != nil {
		return m.Success
	}
	return false
}

func (m *AppendEntriesResponse) GetNoRetryBackoff() bool {
	if m != nil {
		return m.NoRetryBackoff
	}
	return false
}

// RequestVoteRequest is the command used by a candidate to ask a Raft peer
// for a vote in an election.
type RequestVoteRequest struct {
	// Provide the term and our id
	Term      uint64 `protobuf:"varint,1,opt,name=Term,proto3" json:"Term,omitempty"`
	Candidate []byte `protobuf:"bytes,2,opt,name=Candidate,proto3" json:"Candidate,omitempty"`
	// Used to ensure safety
	LastLogIndex         uint64   `protobuf:"varint,3,opt,name=LastLogIndex,proto3" json:"LastLogIndex,omitempty"`
	LastLogTerm          uint64   `protobuf:"varint,4,opt,name=LastLogTerm,proto3" json:"LastLogTerm,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RequestVoteRequest) Reset()         { *m = RequestVoteRequest{} }
func (m *RequestVoteRequest) String() string { return proto.CompactTextString(m) }
func (*RequestVoteRequest) ProtoMessage()    {}
func (*RequestVoteRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_b042552c306ae59b, []int{3}
}

func (m *RequestVoteRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RequestVoteRequest.Unmarshal(m, b)
}
func (m *RequestVoteRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RequestVoteRequest.Marshal(b, m, deterministic)
}
func (m *RequestVoteRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RequestVoteRequest.Merge(m, src)
}
func (m *RequestVoteRequest) XXX_Size() int {
	return xxx_messageInfo_RequestVoteRequest.Size(m)
}
func (m *RequestVoteRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_RequestVoteRequest.DiscardUnknown(m)
}

var xxx_messageInfo_RequestVoteRequest proto.InternalMessageInfo

func (m *RequestVoteRequest) GetTerm() uint64 {
	if m != nil {
		return m.Term
	}
	return 0
}

func (m *RequestVoteRequest) GetCandidate() []byte {
	if m != nil {
		return m.Candidate
	}
	return nil
}

func (m *RequestVoteRequest) GetLastLogIndex() uint64 {
	if m != nil {
		return m.LastLogIndex
	}
	return 0
}

func (m *RequestVoteRequest) GetLastLogTerm() uint64 {
	if m != nil {
		return m.LastLogTerm
	}
	return 0
}

// RequestVoteResponse is the response returned from a RequestVoteRequest.
type RequestVoteResponse struct {
	// Newer term if leader is out of date
	Term uint64 `protobuf:"varint,1,opt,name=Term,proto3" json:"Term,omitempty"`
	// Return the peers, so that a node can shutdown on removal
	Peers []byte `protobuf:"bytes,2,opt,name=Peers,proto3" json:"Peers,omitempty"`
	// Is the vote granted
	Granted              bool     `protobuf:"varint,3,opt,name=Granted,proto3" json:"Granted,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RequestVoteResponse) Reset()         { *m = RequestVoteResponse{} }
func (m *RequestVoteResponse) String() string { return proto.CompactTextString(m) }
func (*RequestVoteResponse) ProtoMessage()    {}
func (*RequestVoteResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_b042552c306ae59b, []int{4}
}

func (m *RequestVoteResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RequestVoteResponse.Unmarshal(m, b)
}
func (m *RequestVoteResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RequestVoteResponse.Marshal(b, m, deterministic)
}
func (m *RequestVoteResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RequestVoteResponse.Merge(m, src)
}
func (m *RequestVoteResponse) XXX_Size() int {
	return xxx_messageInfo_RequestVoteResponse.Size(m)
}
func (m *RequestVoteResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_RequestVoteResponse.DiscardUnknown(m)
}

var xxx_messageInfo_RequestVoteResponse proto.InternalMessageInfo

func (m *RequestVoteResponse) GetTerm() uint64 {
	if m != nil {
		return m.Term
	}
	return 0
}

func (m *RequestVoteResponse) GetPeers() []byte {
	if m != nil {
		return m.Peers
	}
	return nil
}

func (m *RequestVoteResponse) GetGranted() bool {
	if m != nil {
		return m.Granted
	}
	return false
}

// InstallSnapshotRequest is the command sent to a Raft peer to bootstrap its
// log (and state machine) from a snapshot on another peer.
type InstallSnapshotRequest struct {
	Term   uint64 `protobuf:"varint,1,opt,name=Term,proto3" json:"Term,omitempty"`
	Leader []byte `protobuf:"bytes,2,opt,name=Leader,proto3" json:"Leader,omitempty"`
	// These are the last index/term included in the snapshot
	LastLogIndex uint64 `protobuf:"varint,3,opt,name=LastLogIndex,proto3" json:"LastLogIndex,omitempty"`
	LastLogTerm  uint64 `protobuf:"varint,4,opt,name=LastLogTerm,proto3" json:"LastLogTerm,omitempty"`
	// Peer Set in the snapshot
	Peers []byte `protobuf:"bytes,5,opt,name=Peers,proto3" json:"Peers,omitempty"`
	// Size of the snapshot
	Size                 int64    `protobuf:"varint,6,opt,name=Size,proto3" json:"Size,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *InstallSnapshotRequest) Reset()         { *m = InstallSnapshotRequest{} }
func (m *InstallSnapshotRequest) String() string { return proto.CompactTextString(m) }
func (*InstallSnapshotRequest) ProtoMessage()    {}
func (*InstallSnapshotRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_b042552c306ae59b, []int{5}
}

func (m *InstallSnapshotRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_InstallSnapshotRequest.Unmarshal(m, b)
}
func (m *InstallSnapshotRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_InstallSnapshotRequest.Marshal(b, m, deterministic)
}
func (m *InstallSnapshotRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_InstallSnapshotRequest.Merge(m, src)
}
func (m *InstallSnapshotRequest) XXX_Size() int {
	return xxx_messageInfo_InstallSnapshotRequest.Size(m)
}
func (m *InstallSnapshotRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_InstallSnapshotRequest.DiscardUnknown(m)
}

var xxx_messageInfo_InstallSnapshotRequest proto.InternalMessageInfo

func (m *InstallSnapshotRequest) GetTerm() uint64 {
	if m != nil {
		return m.Term
	}
	return 0
}

func (m *InstallSnapshotRequest) GetLeader() []byte {
	if m != nil {
		return m.Leader
	}
	return nil
}

func (m *InstallSnapshotRequest) GetLastLogIndex() uint64 {
	if m != nil {
		return m.LastLogIndex
	}
	return 0
}

func (m *InstallSnapshotRequest) GetLastLogTerm() uint64 {
	if m != nil {
		return m.LastLogTerm
	}
	return 0
}

func (m *InstallSnapshotRequest) GetPeers() []byte {
	if m != nil {
		return m.Peers
	}
	return nil
}

func (m *InstallSnapshotRequest) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

// InstallSnapshotResponse is the response returned from an
// InstallSnapshotRequest.
type InstallSnapshotResponse struct {
	Term                 uint64   `protobuf:"varint,1,opt,name=Term,proto3" json:"Term,omitempty"`
	Success              bool     `protobuf:"varint,2,opt,name=Success,proto3" json:"Success,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *InstallSnapshotResponse) Reset()         { *m = InstallSnapshotResponse{} }
func (m *InstallSnapshotResponse) String() string { return proto.CompactTextString(m) }
func (*InstallSnapshotResponse) ProtoMessage()    {}
func (*InstallSnapshotResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_b042552c306ae59b, []int{6}
}

func (m *InstallSnapshotResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_InstallSnapshotResponse.Unmarshal(m, b)
}
func (m *InstallSnapshotResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_InstallSnapshotResponse.Marshal(b, m, deterministic)
}
func (m *InstallSnapshotResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_InstallSnapshotResponse.Merge(m, src)
}
func (m *InstallSnapshotResponse) XXX_Size() int {
	return xxx_messageInfo_InstallSnapshotResponse.Size(m)
}
func (m *InstallSnapshotResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_InstallSnapshotResponse.DiscardUnknown(m)
}

var xxx_messageInfo_InstallSnapshotResponse proto.InternalMessageInfo

func (m *InstallSnapshotResponse) GetTerm() uint64 {
	if m != nil {
		return m.Term
	}
	return 0
}

func (m *InstallSnapshotResponse) GetSuccess() bool {
	if m != nil {
		return m.Success
	}
	return false
}

func init() {
	proto.RegisterType((*Log)(nil), "cluster.Log")
	proto.RegisterType((*AppendEntriesRequest)(nil), "cluster.AppendEntriesRequest")
	proto.RegisterType((*AppendEntriesResponse)(nil), "cluster.AppendEntriesResponse")
	proto.RegisterType((*RequestVoteRequest)(nil), "cluster.RequestVoteRequest")
	proto.RegisterType((*RequestVoteResponse)(nil), "cluster.RequestVoteResponse")
	proto.RegisterType((*InstallSnapshotRequest)(nil), "cluster.InstallSnapshotRequest")
	proto.RegisterType((*InstallSnapshotResponse)(nil), "cluster.InstallSnapshotResponse")
}

func init() { proto.RegisterFile("raft.proto", fileDescriptor_b042552c306ae59b) }

var fileDescriptor_b042552c306ae59b = []byte{
	// 535 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xa4, 0x54, 0xcd, 0x6e, 0xd3, 0x40,
	0x10, 0x66, 0x1b, 0x27, 0x69, 0x26, 0x29, 0x88, 0x25, 0x14, 0x2b, 0x14, 0x30, 0x3e, 0x54, 0x39,
	0xa0, 0x08, 0x95, 0x27, 0x20, 0x05, 0x55, 0x45, 0x16, 0x8a, 0x36, 0xa8, 0x12, 0xc7, 0x6d, 0x3c,
	0x0e, 0x56, 0x12, 0xaf, 0xd9, 0xdd, 0x00, 0xe1, 0x05, 0xb8, 0xf0, 0x38, 0x3c, 0x0f, 0x4f, 0xc2,
	0x01, 0x79, 0xbd, 0x76, 0x9d, 0x9f, 0xe6, 0x00, 0xb7, 0xf9, 0xbe, 0xf1, 0xce, 0x7e, 0xf3, 0xf9,
	0xb3, 0x01, 0x24, 0x8f, 0xf4, 0x20, 0x95, 0x42, 0x0b, 0xda, 0x9c, 0xcc, 0x97, 0x4a, 0xa3, 0xf4,
	0x67, 0x50, 0x0b, 0xc4, 0x94, 0x76, 0xa1, 0x7e, 0x99, 0x84, 0xf8, 0xcd, 0x25, 0x1e, 0xe9, 0x3b,
	0x2c, 0x07, 0x94, 0x82, 0xf3, 0x01, 0xe5, 0xc2, 0x3d, 0x30, 0xa4, 0xa9, 0x0d, 0xb7, 0x4a, 0xd1,
	0xad, 0x79, 0xa4, 0x7f, 0xc4, 0x4c, 0x9d, 0x71, 0x6f, 0xb8, 0xe6, 0xae, 0xe3, 0x91, 0x7e, 0x87,
	0x99, 0x3a, 0xe3, 0x52, 0x44, 0xe9, 0xd6, 0x3d, 0xd2, 0x6f, 0x31, 0x53, 0xfb, 0xbf, 0x09, 0x74,
	0x5f, 0xa7, 0x29, 0x26, 0xe1, 0xdb, 0x44, 0xcb, 0x18, 0x15, 0xc3, 0xcf, 0x4b, 0x54, 0xba, 0xbc,
	0x88, 0x54, 0x2e, 0x3a, 0x86, 0x46, 0x80, 0x3c, 0x44, 0x69, 0xae, 0xef, 0x30, 0x8b, 0xa8, 0x0f,
	0x9d, 0x91, 0xc4, 0x2f, 0x81, 0x98, 0x66, 0x43, 0x56, 0x46, 0x88, 0xc3, 0xd6, 0x38, 0xea, 0x41,
	0xdb, 0x62, 0x33, 0xd6, 0x31, 0x8f, 0x54, 0x29, 0x7a, 0x0a, 0x4d, 0xab, 0xc1, 0xad, 0x7b, 0xb5,
	0x7e, 0xfb, 0xac, 0x33, 0xb0, 0x96, 0x0c, 0x02, 0x31, 0x65, 0x45, 0x93, 0xbe, 0x80, 0xfb, 0xf9,
	0xbd, 0xe7, 0x62, 0xb1, 0x88, 0x75, 0x6e, 0x52, 0xc3, 0xcc, 0xdb, 0x6e, 0xf8, 0x3f, 0x08, 0x3c,
	0xdc, 0x58, 0x50, 0xa5, 0x22, 0x51, 0xb8, 0x73, 0x43, 0x17, 0x9a, 0x01, 0x57, 0x3a, 0x10, 0x53,
	0xeb, 0x70, 0x01, 0xb3, 0xce, 0x78, 0x39, 0x99, 0xa0, 0x52, 0x66, 0xbd, 0x43, 0x56, 0x40, 0x7a,
	0x0a, 0x77, 0xdf, 0x0b, 0x86, 0x5a, 0xae, 0x86, 0x7c, 0x32, 0x13, 0x51, 0x64, 0x96, 0x3b, 0x64,
	0x1b, 0xac, 0xff, 0x93, 0x00, 0xb5, 0xee, 0x5e, 0x09, 0x8d, 0xfb, 0x8c, 0x3e, 0x81, 0xd6, 0x39,
	0x4f, 0xc2, 0x38, 0xe4, 0x1a, 0xad, 0xd7, 0x37, 0x44, 0x66, 0xb7, 0x55, 0x95, 0xef, 0x6e, 0xed,
	0xae, 0x72, 0x99, 0xdd, 0x16, 0x57, 0xed, 0xae, 0x50, 0xfe, 0x47, 0x78, 0xb0, 0xa6, 0x66, 0x8f,
	0x2b, 0x5d, 0xa8, 0x8f, 0x10, 0xa5, 0xb2, 0x52, 0x72, 0x90, 0x39, 0x72, 0x21, 0x79, 0xa2, 0x31,
	0x2c, 0x1c, 0xb1, 0xd0, 0xff, 0x45, 0xe0, 0xf8, 0x32, 0x51, 0x9a, 0xcf, 0xe7, 0xe3, 0x84, 0xa7,
	0xea, 0x93, 0xd0, 0xff, 0x18, 0xab, 0xff, 0xdf, 0xf3, 0x46, 0x7c, 0xbd, 0x2a, 0x9e, 0x82, 0x33,
	0x8e, 0xbf, 0xa3, 0xc9, 0x4d, 0x8d, 0x99, 0xda, 0xbf, 0x80, 0x47, 0x5b, 0xaa, 0xf7, 0x67, 0xa5,
	0x48, 0xc4, 0xc1, 0x5a, 0x22, 0xce, 0xfe, 0x10, 0x70, 0x18, 0x8f, 0x34, 0x1d, 0xc1, 0xd1, 0x5a,
	0xf6, 0xe8, 0x93, 0x32, 0xd2, 0xbb, 0x3e, 0xba, 0xde, 0xd3, 0xdb, 0xda, 0xb9, 0x0c, 0xff, 0x0e,
	0x7d, 0x07, 0xed, 0xca, 0x5b, 0xa3, 0x8f, 0xcb, 0x03, 0xdb, 0xc9, 0xea, 0x9d, 0xec, 0x6e, 0x96,
	0xb3, 0xae, 0xe0, 0xde, 0xc6, 0xbe, 0xf4, 0x59, 0x79, 0x64, 0xf7, 0xfb, 0xeb, 0x79, 0xb7, 0x3f,
	0x50, 0xcc, 0x1d, 0xbe, 0x84, 0xe7, 0x13, 0xb1, 0x18, 0xc4, 0x49, 0x14, 0x27, 0xb1, 0xbe, 0x5e,
	0x69, 0x1c, 0x44, 0x92, 0x2f, 0xf0, 0xab, 0x90, 0xb3, 0xe2, 0xf8, 0xb0, 0x95, 0x19, 0x34, 0xca,
	0xfe, 0x7c, 0x23, 0x72, 0xdd, 0x30, 0xbf, 0xc0, 0x57, 0x7f, 0x03, 0x00, 0x00, 0xff, 0xff, 0xca,
	0x03, 0xa0, 0x90, 0x10, 0x05, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// RaftClient is the client API for Raft service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RaftClient interface {
	AppendEntries(ctx context.Context, in *AppendEntriesRequest, opts ...grpc.CallOption) (*AppendEntriesResponse, error)
	RequestVote(ctx context.Context, in *RequestVoteRequest, opts ...grpc.CallOption) (*RequestVoteResponse, error)
	InstallSnapshot(ctx context.Context, in *InstallSnapshotRequest, opts ...grpc.CallOption) (*InstallSnapshotResponse, error)
}

type raftClient struct {
	cc *grpc.ClientConn
}

func NewRaftClient(cc *grpc.ClientConn) RaftClient {
	return &raftClient{cc}
}

func (c *raftClient) AppendEntries(ctx context.Context, in *AppendEntriesRequest, opts ...grpc.CallOption) (*AppendEntriesResponse, error) {
	out := new(AppendEntriesResponse)
	err := c.cc.Invoke(ctx, "/cluster.Raft/AppendEntries", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *raftClient) RequestVote(ctx context.Context, in *RequestVoteRequest, opts ...grpc.CallOption) (*RequestVoteResponse, error) {
	out := new(RequestVoteResponse)
	err := c.cc.Invoke(ctx, "/cluster.Raft/RequestVote", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *raftClient) InstallSnapshot(ctx context.Context, in *InstallSnapshotRequest, opts ...grpc.CallOption) (*InstallSnapshotResponse, error) {
	out := new(InstallSnapshotResponse)
	err := c.cc.Invoke(ctx, "/cluster.Raft/InstallSnapshot", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RaftServer is the server API for Raft service.
type RaftServer interface {
	AppendEntries(context.Context, *AppendEntriesRequest) (*AppendEntriesResponse, error)
	RequestVote(context.Context, *RequestVoteRequest) (*RequestVoteResponse, error)
	InstallSnapshot(context.Context, *InstallSnapshotRequest) (*InstallSnapshotResponse, error)
}

func RegisterRaftServer(s *grpc.Server, srv RaftServer) {
	s.RegisterService(&_Raft_serviceDesc, srv)
}

func _Raft_AppendEntries_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AppendEntriesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RaftServer).AppendEntries(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cluster.Raft/AppendEntries",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RaftServer).AppendEntries(ctx, req.(*AppendEntriesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Raft_RequestVote_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RequestVoteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RaftServer).RequestVote(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cluster.Raft/RequestVote",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RaftServer).RequestVote(ctx, req.(*RequestVoteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Raft_InstallSnapshot_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(InstallSnapshotRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RaftServer).InstallSnapshot(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cluster.Raft/InstallSnapshot",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RaftServer).InstallSnapshot(ctx, req.(*InstallSnapshotRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Raft_serviceDesc = grpc.ServiceDesc{
	ServiceName: "cluster.Raft",
	HandlerType: (*RaftServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "AppendEntries",
			Handler:    _Raft_AppendEntries_Handler,
		},
		{
			MethodName: "RequestVote",
			Handler:    _Raft_RequestVote_Handler,
		},
		{
			MethodName: "InstallSnapshot",
			Handler:    _Raft_InstallSnapshot_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "raft.proto",
}
