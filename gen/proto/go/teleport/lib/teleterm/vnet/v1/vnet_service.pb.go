// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        (unknown)
// source: teleport/lib/teleterm/vnet/v1/vnet_service.proto

package vnetv1

import (
	v1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// BackgroundItemStatus maps to SMAppServiceStatus of the Service Management framework in macOS.
// https://developer.apple.com/documentation/servicemanagement/smappservice/status-swift.enum?language=objc
type BackgroundItemStatus int32

const (
	BackgroundItemStatus_BACKGROUND_ITEM_STATUS_UNSPECIFIED    BackgroundItemStatus = 0
	BackgroundItemStatus_BACKGROUND_ITEM_STATUS_NOT_REGISTERED BackgroundItemStatus = 1
	// This is the status the background item should have before tsh attempts to send a message to the
	// daemon.
	BackgroundItemStatus_BACKGROUND_ITEM_STATUS_ENABLED           BackgroundItemStatus = 2
	BackgroundItemStatus_BACKGROUND_ITEM_STATUS_REQUIRES_APPROVAL BackgroundItemStatus = 3
	BackgroundItemStatus_BACKGROUND_ITEM_STATUS_NOT_FOUND         BackgroundItemStatus = 4
	BackgroundItemStatus_BACKGROUND_ITEM_STATUS_NOT_SUPPORTED     BackgroundItemStatus = 5
)

// Enum value maps for BackgroundItemStatus.
var (
	BackgroundItemStatus_name = map[int32]string{
		0: "BACKGROUND_ITEM_STATUS_UNSPECIFIED",
		1: "BACKGROUND_ITEM_STATUS_NOT_REGISTERED",
		2: "BACKGROUND_ITEM_STATUS_ENABLED",
		3: "BACKGROUND_ITEM_STATUS_REQUIRES_APPROVAL",
		4: "BACKGROUND_ITEM_STATUS_NOT_FOUND",
		5: "BACKGROUND_ITEM_STATUS_NOT_SUPPORTED",
	}
	BackgroundItemStatus_value = map[string]int32{
		"BACKGROUND_ITEM_STATUS_UNSPECIFIED":       0,
		"BACKGROUND_ITEM_STATUS_NOT_REGISTERED":    1,
		"BACKGROUND_ITEM_STATUS_ENABLED":           2,
		"BACKGROUND_ITEM_STATUS_REQUIRES_APPROVAL": 3,
		"BACKGROUND_ITEM_STATUS_NOT_FOUND":         4,
		"BACKGROUND_ITEM_STATUS_NOT_SUPPORTED":     5,
	}
)

func (x BackgroundItemStatus) Enum() *BackgroundItemStatus {
	p := new(BackgroundItemStatus)
	*p = x
	return p
}

func (x BackgroundItemStatus) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (BackgroundItemStatus) Descriptor() protoreflect.EnumDescriptor {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_enumTypes[0].Descriptor()
}

func (BackgroundItemStatus) Type() protoreflect.EnumType {
	return &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_enumTypes[0]
}

func (x BackgroundItemStatus) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use BackgroundItemStatus.Descriptor instead.
func (BackgroundItemStatus) EnumDescriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{0}
}

// Request for Start.
type StartRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *StartRequest) Reset() {
	*x = StartRequest{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StartRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StartRequest) ProtoMessage() {}

func (x *StartRequest) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StartRequest.ProtoReflect.Descriptor instead.
func (*StartRequest) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{0}
}

// Response for Start.
type StartResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *StartResponse) Reset() {
	*x = StartResponse{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StartResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StartResponse) ProtoMessage() {}

func (x *StartResponse) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StartResponse.ProtoReflect.Descriptor instead.
func (*StartResponse) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{1}
}

// Request for Stop.
type StopRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *StopRequest) Reset() {
	*x = StopRequest{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StopRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StopRequest) ProtoMessage() {}

func (x *StopRequest) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StopRequest.ProtoReflect.Descriptor instead.
func (*StopRequest) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{2}
}

// Response for Stop.
type StopResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *StopResponse) Reset() {
	*x = StopResponse{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StopResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StopResponse) ProtoMessage() {}

func (x *StopResponse) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StopResponse.ProtoReflect.Descriptor instead.
func (*StopResponse) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{3}
}

// Request for ListDNSZones.
type ListDNSZonesRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListDNSZonesRequest) Reset() {
	*x = ListDNSZonesRequest{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListDNSZonesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListDNSZonesRequest) ProtoMessage() {}

func (x *ListDNSZonesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListDNSZonesRequest.ProtoReflect.Descriptor instead.
func (*ListDNSZonesRequest) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{4}
}

// Response for ListDNSZones.
type ListDNSZonesResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// dns_zones is a deduplicated list of DNS zones.
	DnsZones      []string `protobuf:"bytes,1,rep,name=dns_zones,json=dnsZones,proto3" json:"dns_zones,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListDNSZonesResponse) Reset() {
	*x = ListDNSZonesResponse{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListDNSZonesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListDNSZonesResponse) ProtoMessage() {}

func (x *ListDNSZonesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListDNSZonesResponse.ProtoReflect.Descriptor instead.
func (*ListDNSZonesResponse) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{5}
}

func (x *ListDNSZonesResponse) GetDnsZones() []string {
	if x != nil {
		return x.DnsZones
	}
	return nil
}

// Request for GetBackgroundItemStatus.
type GetBackgroundItemStatusRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetBackgroundItemStatusRequest) Reset() {
	*x = GetBackgroundItemStatusRequest{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetBackgroundItemStatusRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBackgroundItemStatusRequest) ProtoMessage() {}

func (x *GetBackgroundItemStatusRequest) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBackgroundItemStatusRequest.ProtoReflect.Descriptor instead.
func (*GetBackgroundItemStatusRequest) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{6}
}

// Response for GetBackgroundItemStatus.
type GetBackgroundItemStatusResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Status        BackgroundItemStatus   `protobuf:"varint,1,opt,name=status,proto3,enum=teleport.lib.teleterm.vnet.v1.BackgroundItemStatus" json:"status,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetBackgroundItemStatusResponse) Reset() {
	*x = GetBackgroundItemStatusResponse{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetBackgroundItemStatusResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBackgroundItemStatusResponse) ProtoMessage() {}

func (x *GetBackgroundItemStatusResponse) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBackgroundItemStatusResponse.ProtoReflect.Descriptor instead.
func (*GetBackgroundItemStatusResponse) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{7}
}

func (x *GetBackgroundItemStatusResponse) GetStatus() BackgroundItemStatus {
	if x != nil {
		return x.Status
	}
	return BackgroundItemStatus_BACKGROUND_ITEM_STATUS_UNSPECIFIED
}

// Request for RunDiagnostics.
type RunDiagnosticsRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RunDiagnosticsRequest) Reset() {
	*x = RunDiagnosticsRequest{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RunDiagnosticsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunDiagnosticsRequest) ProtoMessage() {}

func (x *RunDiagnosticsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunDiagnosticsRequest.ProtoReflect.Descriptor instead.
func (*RunDiagnosticsRequest) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{8}
}

// Response for RunDiagnostics.
type RunDiagnosticsResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Report        *v1.Report             `protobuf:"bytes,1,opt,name=report,proto3" json:"report,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RunDiagnosticsResponse) Reset() {
	*x = RunDiagnosticsResponse{}
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RunDiagnosticsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunDiagnosticsResponse) ProtoMessage() {}

func (x *RunDiagnosticsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunDiagnosticsResponse.ProtoReflect.Descriptor instead.
func (*RunDiagnosticsResponse) Descriptor() ([]byte, []int) {
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP(), []int{9}
}

func (x *RunDiagnosticsResponse) GetReport() *v1.Report {
	if x != nil {
		return x.Report
	}
	return nil
}

var File_teleport_lib_teleterm_vnet_v1_vnet_service_proto protoreflect.FileDescriptor

var file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDesc = string([]byte{
	0x0a, 0x30, 0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x2f, 0x6c, 0x69, 0x62, 0x2f, 0x74,
	0x65, 0x6c, 0x65, 0x74, 0x65, 0x72, 0x6d, 0x2f, 0x76, 0x6e, 0x65, 0x74, 0x2f, 0x76, 0x31, 0x2f,
	0x76, 0x6e, 0x65, 0x74, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x1d, 0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62,
	0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x72, 0x6d, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76,
	0x31, 0x1a, 0x24, 0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x2f, 0x6c, 0x69, 0x62, 0x2f,
	0x76, 0x6e, 0x65, 0x74, 0x2f, 0x64, 0x69, 0x61, 0x67, 0x2f, 0x76, 0x31, 0x2f, 0x64, 0x69, 0x61,
	0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x0e, 0x0a, 0x0c, 0x53, 0x74, 0x61, 0x72, 0x74,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x0f, 0x0a, 0x0d, 0x53, 0x74, 0x61, 0x72, 0x74,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x0d, 0x0a, 0x0b, 0x53, 0x74, 0x6f, 0x70,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x0e, 0x0a, 0x0c, 0x53, 0x74, 0x6f, 0x70, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x15, 0x0a, 0x13, 0x4c, 0x69, 0x73, 0x74, 0x44,
	0x4e, 0x53, 0x5a, 0x6f, 0x6e, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x33,
	0x0a, 0x14, 0x4c, 0x69, 0x73, 0x74, 0x44, 0x4e, 0x53, 0x5a, 0x6f, 0x6e, 0x65, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x64, 0x6e, 0x73, 0x5f, 0x7a, 0x6f,
	0x6e, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x08, 0x64, 0x6e, 0x73, 0x5a, 0x6f,
	0x6e, 0x65, 0x73, 0x22, 0x20, 0x0a, 0x1e, 0x47, 0x65, 0x74, 0x42, 0x61, 0x63, 0x6b, 0x67, 0x72,
	0x6f, 0x75, 0x6e, 0x64, 0x49, 0x74, 0x65, 0x6d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x6e, 0x0a, 0x1f, 0x47, 0x65, 0x74, 0x42, 0x61, 0x63, 0x6b,
	0x67, 0x72, 0x6f, 0x75, 0x6e, 0x64, 0x49, 0x74, 0x65, 0x6d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4b, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x33, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x70,
	0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x72, 0x6d,
	0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x67, 0x72, 0x6f,
	0x75, 0x6e, 0x64, 0x49, 0x74, 0x65, 0x6d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73,
	0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0x17, 0x0a, 0x15, 0x52, 0x75, 0x6e, 0x44, 0x69, 0x61, 0x67,
	0x6e, 0x6f, 0x73, 0x74, 0x69, 0x63, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x53,
	0x0a, 0x16, 0x52, 0x75, 0x6e, 0x44, 0x69, 0x61, 0x67, 0x6e, 0x6f, 0x73, 0x74, 0x69, 0x63, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x39, 0x0a, 0x06, 0x72, 0x65, 0x70, 0x6f,
	0x72, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x21, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x70,
	0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x64, 0x69, 0x61,
	0x67, 0x2e, 0x76, 0x31, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x06, 0x72, 0x65, 0x70,
	0x6f, 0x72, 0x74, 0x2a, 0x8b, 0x02, 0x0a, 0x14, 0x42, 0x61, 0x63, 0x6b, 0x67, 0x72, 0x6f, 0x75,
	0x6e, 0x64, 0x49, 0x74, 0x65, 0x6d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x26, 0x0a, 0x22,
	0x42, 0x41, 0x43, 0x4b, 0x47, 0x52, 0x4f, 0x55, 0x4e, 0x44, 0x5f, 0x49, 0x54, 0x45, 0x4d, 0x5f,
	0x53, 0x54, 0x41, 0x54, 0x55, 0x53, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49,
	0x45, 0x44, 0x10, 0x00, 0x12, 0x29, 0x0a, 0x25, 0x42, 0x41, 0x43, 0x4b, 0x47, 0x52, 0x4f, 0x55,
	0x4e, 0x44, 0x5f, 0x49, 0x54, 0x45, 0x4d, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x55, 0x53, 0x5f, 0x4e,
	0x4f, 0x54, 0x5f, 0x52, 0x45, 0x47, 0x49, 0x53, 0x54, 0x45, 0x52, 0x45, 0x44, 0x10, 0x01, 0x12,
	0x22, 0x0a, 0x1e, 0x42, 0x41, 0x43, 0x4b, 0x47, 0x52, 0x4f, 0x55, 0x4e, 0x44, 0x5f, 0x49, 0x54,
	0x45, 0x4d, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x55, 0x53, 0x5f, 0x45, 0x4e, 0x41, 0x42, 0x4c, 0x45,
	0x44, 0x10, 0x02, 0x12, 0x2c, 0x0a, 0x28, 0x42, 0x41, 0x43, 0x4b, 0x47, 0x52, 0x4f, 0x55, 0x4e,
	0x44, 0x5f, 0x49, 0x54, 0x45, 0x4d, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x55, 0x53, 0x5f, 0x52, 0x45,
	0x51, 0x55, 0x49, 0x52, 0x45, 0x53, 0x5f, 0x41, 0x50, 0x50, 0x52, 0x4f, 0x56, 0x41, 0x4c, 0x10,
	0x03, 0x12, 0x24, 0x0a, 0x20, 0x42, 0x41, 0x43, 0x4b, 0x47, 0x52, 0x4f, 0x55, 0x4e, 0x44, 0x5f,
	0x49, 0x54, 0x45, 0x4d, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x55, 0x53, 0x5f, 0x4e, 0x4f, 0x54, 0x5f,
	0x46, 0x4f, 0x55, 0x4e, 0x44, 0x10, 0x04, 0x12, 0x28, 0x0a, 0x24, 0x42, 0x41, 0x43, 0x4b, 0x47,
	0x52, 0x4f, 0x55, 0x4e, 0x44, 0x5f, 0x49, 0x54, 0x45, 0x4d, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x55,
	0x53, 0x5f, 0x4e, 0x4f, 0x54, 0x5f, 0x53, 0x55, 0x50, 0x50, 0x4f, 0x52, 0x54, 0x45, 0x44, 0x10,
	0x05, 0x32, 0xe5, 0x04, 0x0a, 0x0b, 0x56, 0x6e, 0x65, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x12, 0x62, 0x0a, 0x05, 0x53, 0x74, 0x61, 0x72, 0x74, 0x12, 0x2b, 0x2e, 0x74, 0x65, 0x6c,
	0x65, 0x70, 0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65,
	0x72, 0x6d, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x74, 0x61, 0x72, 0x74,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2c, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f,
	0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x72, 0x6d, 0x2e,
	0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x74, 0x61, 0x72, 0x74, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x5f, 0x0a, 0x04, 0x53, 0x74, 0x6f, 0x70, 0x12, 0x2a, 0x2e,
	0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c,
	0x65, 0x74, 0x65, 0x72, 0x6d, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x74,
	0x6f, 0x70, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2b, 0x2e, 0x74, 0x65, 0x6c, 0x65,
	0x70, 0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x72,
	0x6d, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x74, 0x6f, 0x70, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x77, 0x0a, 0x0c, 0x4c, 0x69, 0x73, 0x74, 0x44, 0x4e,
	0x53, 0x5a, 0x6f, 0x6e, 0x65, 0x73, 0x12, 0x32, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72,
	0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x72, 0x6d, 0x2e, 0x76,
	0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x44, 0x4e, 0x53, 0x5a, 0x6f,
	0x6e, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x33, 0x2e, 0x74, 0x65, 0x6c,
	0x65, 0x70, 0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65,
	0x72, 0x6d, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x44,
	0x4e, 0x53, 0x5a, 0x6f, 0x6e, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x98, 0x01, 0x0a, 0x17, 0x47, 0x65, 0x74, 0x42, 0x61, 0x63, 0x6b, 0x67, 0x72, 0x6f, 0x75, 0x6e,
	0x64, 0x49, 0x74, 0x65, 0x6d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x3d, 0x2e, 0x74, 0x65,
	0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74,
	0x65, 0x72, 0x6d, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x42,
	0x61, 0x63, 0x6b, 0x67, 0x72, 0x6f, 0x75, 0x6e, 0x64, 0x49, 0x74, 0x65, 0x6d, 0x53, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x3e, 0x2e, 0x74, 0x65, 0x6c,
	0x65, 0x70, 0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65,
	0x72, 0x6d, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x42, 0x61,
	0x63, 0x6b, 0x67, 0x72, 0x6f, 0x75, 0x6e, 0x64, 0x49, 0x74, 0x65, 0x6d, 0x53, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x7d, 0x0a, 0x0e, 0x52, 0x75,
	0x6e, 0x44, 0x69, 0x61, 0x67, 0x6e, 0x6f, 0x73, 0x74, 0x69, 0x63, 0x73, 0x12, 0x34, 0x2e, 0x74,
	0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69, 0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65,
	0x74, 0x65, 0x72, 0x6d, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x52, 0x75, 0x6e,
	0x44, 0x69, 0x61, 0x67, 0x6e, 0x6f, 0x73, 0x74, 0x69, 0x63, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x35, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x2e, 0x6c, 0x69,
	0x62, 0x2e, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x72, 0x6d, 0x2e, 0x76, 0x6e, 0x65, 0x74, 0x2e,
	0x76, 0x31, 0x2e, 0x52, 0x75, 0x6e, 0x44, 0x69, 0x61, 0x67, 0x6e, 0x6f, 0x73, 0x74, 0x69, 0x63,
	0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x55, 0x5a, 0x53, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x67, 0x72, 0x61, 0x76, 0x69, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x2f, 0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x2f, 0x67,
	0x65, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x6f, 0x2f, 0x74, 0x65, 0x6c, 0x65,
	0x70, 0x6f, 0x72, 0x74, 0x2f, 0x6c, 0x69, 0x62, 0x2f, 0x74, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x72,
	0x6d, 0x2f, 0x76, 0x6e, 0x65, 0x74, 0x2f, 0x76, 0x31, 0x3b, 0x76, 0x6e, 0x65, 0x74, 0x76, 0x31,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescOnce sync.Once
	file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescData []byte
)

func file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescGZIP() []byte {
	file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescOnce.Do(func() {
		file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDesc), len(file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDesc)))
	})
	return file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDescData
}

var file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_goTypes = []any{
	(BackgroundItemStatus)(0),               // 0: teleport.lib.teleterm.vnet.v1.BackgroundItemStatus
	(*StartRequest)(nil),                    // 1: teleport.lib.teleterm.vnet.v1.StartRequest
	(*StartResponse)(nil),                   // 2: teleport.lib.teleterm.vnet.v1.StartResponse
	(*StopRequest)(nil),                     // 3: teleport.lib.teleterm.vnet.v1.StopRequest
	(*StopResponse)(nil),                    // 4: teleport.lib.teleterm.vnet.v1.StopResponse
	(*ListDNSZonesRequest)(nil),             // 5: teleport.lib.teleterm.vnet.v1.ListDNSZonesRequest
	(*ListDNSZonesResponse)(nil),            // 6: teleport.lib.teleterm.vnet.v1.ListDNSZonesResponse
	(*GetBackgroundItemStatusRequest)(nil),  // 7: teleport.lib.teleterm.vnet.v1.GetBackgroundItemStatusRequest
	(*GetBackgroundItemStatusResponse)(nil), // 8: teleport.lib.teleterm.vnet.v1.GetBackgroundItemStatusResponse
	(*RunDiagnosticsRequest)(nil),           // 9: teleport.lib.teleterm.vnet.v1.RunDiagnosticsRequest
	(*RunDiagnosticsResponse)(nil),          // 10: teleport.lib.teleterm.vnet.v1.RunDiagnosticsResponse
	(*v1.Report)(nil),                       // 11: teleport.lib.vnet.diag.v1.Report
}
var file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_depIdxs = []int32{
	0,  // 0: teleport.lib.teleterm.vnet.v1.GetBackgroundItemStatusResponse.status:type_name -> teleport.lib.teleterm.vnet.v1.BackgroundItemStatus
	11, // 1: teleport.lib.teleterm.vnet.v1.RunDiagnosticsResponse.report:type_name -> teleport.lib.vnet.diag.v1.Report
	1,  // 2: teleport.lib.teleterm.vnet.v1.VnetService.Start:input_type -> teleport.lib.teleterm.vnet.v1.StartRequest
	3,  // 3: teleport.lib.teleterm.vnet.v1.VnetService.Stop:input_type -> teleport.lib.teleterm.vnet.v1.StopRequest
	5,  // 4: teleport.lib.teleterm.vnet.v1.VnetService.ListDNSZones:input_type -> teleport.lib.teleterm.vnet.v1.ListDNSZonesRequest
	7,  // 5: teleport.lib.teleterm.vnet.v1.VnetService.GetBackgroundItemStatus:input_type -> teleport.lib.teleterm.vnet.v1.GetBackgroundItemStatusRequest
	9,  // 6: teleport.lib.teleterm.vnet.v1.VnetService.RunDiagnostics:input_type -> teleport.lib.teleterm.vnet.v1.RunDiagnosticsRequest
	2,  // 7: teleport.lib.teleterm.vnet.v1.VnetService.Start:output_type -> teleport.lib.teleterm.vnet.v1.StartResponse
	4,  // 8: teleport.lib.teleterm.vnet.v1.VnetService.Stop:output_type -> teleport.lib.teleterm.vnet.v1.StopResponse
	6,  // 9: teleport.lib.teleterm.vnet.v1.VnetService.ListDNSZones:output_type -> teleport.lib.teleterm.vnet.v1.ListDNSZonesResponse
	8,  // 10: teleport.lib.teleterm.vnet.v1.VnetService.GetBackgroundItemStatus:output_type -> teleport.lib.teleterm.vnet.v1.GetBackgroundItemStatusResponse
	10, // 11: teleport.lib.teleterm.vnet.v1.VnetService.RunDiagnostics:output_type -> teleport.lib.teleterm.vnet.v1.RunDiagnosticsResponse
	7,  // [7:12] is the sub-list for method output_type
	2,  // [2:7] is the sub-list for method input_type
	2,  // [2:2] is the sub-list for extension type_name
	2,  // [2:2] is the sub-list for extension extendee
	0,  // [0:2] is the sub-list for field type_name
}

func init() { file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_init() }
func file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_init() {
	if File_teleport_lib_teleterm_vnet_v1_vnet_service_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDesc), len(file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_goTypes,
		DependencyIndexes: file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_depIdxs,
		EnumInfos:         file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_enumTypes,
		MessageInfos:      file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_msgTypes,
	}.Build()
	File_teleport_lib_teleterm_vnet_v1_vnet_service_proto = out.File
	file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_goTypes = nil
	file_teleport_lib_teleterm_vnet_v1_vnet_service_proto_depIdxs = nil
}
