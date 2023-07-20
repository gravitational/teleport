// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
'use strict';
var grpc = require('@grpc/grpc-js');
var teleport_lib_teleterm_v1_service_pb = require('../../../../teleport/lib/teleterm/v1/service_pb.js');
var teleport_lib_teleterm_v1_access_request_pb = require('../../../../teleport/lib/teleterm/v1/access_request_pb.js');
var teleport_lib_teleterm_v1_auth_settings_pb = require('../../../../teleport/lib/teleterm/v1/auth_settings_pb.js');
var teleport_lib_teleterm_v1_cluster_pb = require('../../../../teleport/lib/teleterm/v1/cluster_pb.js');
var teleport_lib_teleterm_v1_database_pb = require('../../../../teleport/lib/teleterm/v1/database_pb.js');
var teleport_lib_teleterm_v1_gateway_pb = require('../../../../teleport/lib/teleterm/v1/gateway_pb.js');
var teleport_lib_teleterm_v1_kube_pb = require('../../../../teleport/lib/teleterm/v1/kube_pb.js');
var teleport_lib_teleterm_v1_server_pb = require('../../../../teleport/lib/teleterm/v1/server_pb.js');
var teleport_lib_teleterm_v1_usage_events_pb = require('../../../../teleport/lib/teleterm/v1/usage_events_pb.js');

function serialize_teleport_lib_teleterm_v1_AddClusterRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.AddClusterRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.AddClusterRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_AddClusterRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.AddClusterRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_AssumeRoleRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.AssumeRoleRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_AssumeRoleRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_AuthSettings(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.AuthSettings');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_AuthSettings(buffer_arg) {
  return teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_Cluster(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_cluster_pb.Cluster)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.Cluster');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_Cluster(buffer_arg) {
  return teleport_lib_teleterm_v1_cluster_pb.Cluster.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_CreateAccessRequestRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.CreateAccessRequestRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_CreateAccessRequestRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_CreateAccessRequestResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.CreateAccessRequestResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_CreateAccessRequestResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_CreateGatewayRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.CreateGatewayRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_CreateGatewayRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_DeleteAccessRequestRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.DeleteAccessRequestRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_DeleteAccessRequestRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_EmptyResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.EmptyResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.EmptyResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_EmptyResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.EmptyResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_FileTransferProgress(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.FileTransferProgress)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.FileTransferProgress');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_FileTransferProgress(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.FileTransferProgress.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_FileTransferRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.FileTransferRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.FileTransferRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_FileTransferRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.FileTransferRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_Gateway(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_gateway_pb.Gateway)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.Gateway');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_Gateway(buffer_arg) {
  return teleport_lib_teleterm_v1_gateway_pb.Gateway.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetAccessRequestRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetAccessRequestRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetAccessRequestRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetAccessRequestResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetAccessRequestResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetAccessRequestResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetAccessRequestsRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetAccessRequestsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetAccessRequestsRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetAccessRequestsResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetAccessRequestsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetAccessRequestsResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetAuthSettingsRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetAuthSettingsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetAuthSettingsRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetClusterRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetClusterRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetClusterRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetClusterRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetClusterRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetDatabasesRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetDatabasesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetDatabasesRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetDatabasesResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetDatabasesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetDatabasesResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetKubesRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetKubesRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetKubesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetKubesRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetKubesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetKubesResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetKubesResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetKubesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetKubesResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetKubesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetRequestableRolesRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetRequestableRolesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetRequestableRolesRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetRequestableRolesResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetRequestableRolesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetRequestableRolesResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetServersRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetServersRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetServersRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetServersRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetServersRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_GetServersResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.GetServersResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.GetServersResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_GetServersResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.GetServersResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ListClustersRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.ListClustersRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ListClustersRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ListClustersRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.ListClustersRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ListClustersResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.ListClustersResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ListClustersResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ListClustersResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.ListClustersResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ListDatabaseUsersRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ListDatabaseUsersRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ListDatabaseUsersRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ListDatabaseUsersResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ListDatabaseUsersResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ListDatabaseUsersResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ListGatewaysRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ListGatewaysRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ListGatewaysRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ListGatewaysResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ListGatewaysResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ListGatewaysResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ListLeafClustersRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ListLeafClustersRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ListLeafClustersRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_LoginPasswordlessRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.LoginPasswordlessRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_LoginPasswordlessRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_LoginPasswordlessResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.LoginPasswordlessResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_LoginPasswordlessResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_LoginRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.LoginRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.LoginRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_LoginRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.LoginRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_LogoutRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.LogoutRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.LogoutRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_LogoutRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.LogoutRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_RemoveClusterRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.RemoveClusterRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_RemoveClusterRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_RemoveGatewayRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.RemoveGatewayRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_RemoveGatewayRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ReportUsageEventRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ReportUsageEventRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ReportUsageEventRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ReviewAccessRequestRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ReviewAccessRequestRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ReviewAccessRequestRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ReviewAccessRequestResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ReviewAccessRequestResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ReviewAccessRequestResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_SetGatewayLocalPortRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.SetGatewayLocalPortRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_SetGatewayLocalPortRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_SetGatewayTargetSubresourceNameRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_SetGatewayTargetSubresourceNameRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_UpdateHeadlessAuthenticationStateRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_UpdateHeadlessAuthenticationStateRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_UpdateHeadlessAuthenticationStateResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_UpdateHeadlessAuthenticationStateResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_UpdateTshdEventsServerAddressRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_UpdateTshdEventsServerAddressRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_UpdateTshdEventsServerAddressResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_UpdateTshdEventsServerAddressResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// TerminalService is used by the Electron app to communicate with the tsh daemon.
//
// While we aim to preserve backwards compatibility in order to satisfy CI checks and follow the
// proto practices used within the company, this service is not guaranteed to be stable across
// versions. The packaging process of Teleport Connect ensures that the server and the client use
// the same version of the service.
var TerminalServiceService = exports.TerminalServiceService = {
  // UpdateTshdEventsServerAddress lets the Electron app update the address the tsh daemon is
// supposed to use when connecting to the tshd events gRPC service. This RPC needs to be made
// before any other from this service.
//
// The service is supposed to return a response from this call only after the client is ready.
updateTshdEventsServerAddress: {
    path: '/teleport.lib.teleterm.v1.TerminalService/UpdateTshdEventsServerAddress',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_UpdateTshdEventsServerAddressRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_UpdateTshdEventsServerAddressRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_UpdateTshdEventsServerAddressResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_UpdateTshdEventsServerAddressResponse,
  },
  // ListRootClusters lists root clusters
// Does not include detailed cluster information that would require a network request.
listRootClusters: {
    path: '/teleport.lib.teleterm.v1.TerminalService/ListRootClusters',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.ListClustersRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.ListClustersResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_ListClustersRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_ListClustersRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_ListClustersResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_ListClustersResponse,
  },
  // ListLeafClusters lists leaf clusters
// Does not include detailed cluster information that would require a network request.
listLeafClusters: {
    path: '/teleport.lib.teleterm.v1.TerminalService/ListLeafClusters',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.ListClustersResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_ListLeafClustersRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_ListLeafClustersRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_ListClustersResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_ListClustersResponse,
  },
  // GetDatabases returns a filtered and paginated list of databases
getDatabases: {
    path: '/teleport.lib.teleterm.v1.TerminalService/GetDatabases',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_GetDatabasesRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_GetDatabasesRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_GetDatabasesResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_GetDatabasesResponse,
  },
  // ListDatabaseUsers lists allowed users for the given database based on the role set.
listDatabaseUsers: {
    path: '/teleport.lib.teleterm.v1.TerminalService/ListDatabaseUsers',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_ListDatabaseUsersRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_ListDatabaseUsersRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_ListDatabaseUsersResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_ListDatabaseUsersResponse,
  },
  // GetServers returns filtered, sorted, and paginated servers
getServers: {
    path: '/teleport.lib.teleterm.v1.TerminalService/GetServers',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.GetServersRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.GetServersResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_GetServersRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_GetServersRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_GetServersResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_GetServersResponse,
  },
  // GetAccessRequests lists filtered AccessRequests
getAccessRequests: {
    path: '/teleport.lib.teleterm.v1.TerminalService/GetAccessRequests',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_GetAccessRequestsRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_GetAccessRequestsRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_GetAccessRequestsResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_GetAccessRequestsResponse,
  },
  // GetAccessRequest retreives a single Access Request
getAccessRequest: {
    path: '/teleport.lib.teleterm.v1.TerminalService/GetAccessRequest',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_GetAccessRequestRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_GetAccessRequestRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_GetAccessRequestResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_GetAccessRequestResponse,
  },
  // DeleteAccessRequest deletes the access request by id
deleteAccessRequest: {
    path: '/teleport.lib.teleterm.v1.TerminalService/DeleteAccessRequest',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.EmptyResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_DeleteAccessRequestRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_DeleteAccessRequestRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_EmptyResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_EmptyResponse,
  },
  // CreateAccessRequest creates an access request
createAccessRequest: {
    path: '/teleport.lib.teleterm.v1.TerminalService/CreateAccessRequest',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_CreateAccessRequestRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_CreateAccessRequestRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_CreateAccessRequestResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_CreateAccessRequestResponse,
  },
  // ReviewAccessRequest submits a review for an Access Request
reviewAccessRequest: {
    path: '/teleport.lib.teleterm.v1.TerminalService/ReviewAccessRequest',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_ReviewAccessRequestRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_ReviewAccessRequestRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_ReviewAccessRequestResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_ReviewAccessRequestResponse,
  },
  // GetRequestableRoles gets all requestable roles
getRequestableRoles: {
    path: '/teleport.lib.teleterm.v1.TerminalService/GetRequestableRoles',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_GetRequestableRolesRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_GetRequestableRolesRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_GetRequestableRolesResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_GetRequestableRolesResponse,
  },
  // AssumeRole assumes the role of the given access request
assumeRole: {
    path: '/teleport.lib.teleterm.v1.TerminalService/AssumeRole',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.EmptyResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_AssumeRoleRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_AssumeRoleRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_EmptyResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_EmptyResponse,
  },
  // GetKubes returns filtered, sorted, and paginated kubes
getKubes: {
    path: '/teleport.lib.teleterm.v1.TerminalService/GetKubes',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.GetKubesRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.GetKubesResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_GetKubesRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_GetKubesRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_GetKubesResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_GetKubesResponse,
  },
  // AddCluster adds a cluster to profile
addCluster: {
    path: '/teleport.lib.teleterm.v1.TerminalService/AddCluster',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.AddClusterRequest,
    responseType: teleport_lib_teleterm_v1_cluster_pb.Cluster,
    requestSerialize: serialize_teleport_lib_teleterm_v1_AddClusterRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_AddClusterRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_Cluster,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_Cluster,
  },
  // RemoveCluster removes a cluster from profile
removeCluster: {
    path: '/teleport.lib.teleterm.v1.TerminalService/RemoveCluster',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.EmptyResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_RemoveClusterRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_RemoveClusterRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_EmptyResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_EmptyResponse,
  },
  // ListGateways lists gateways
listGateways: {
    path: '/teleport.lib.teleterm.v1.TerminalService/ListGateways',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_ListGatewaysRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_ListGatewaysRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_ListGatewaysResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_ListGatewaysResponse,
  },
  // CreateGateway creates a gateway
createGateway: {
    path: '/teleport.lib.teleterm.v1.TerminalService/CreateGateway',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest,
    responseType: teleport_lib_teleterm_v1_gateway_pb.Gateway,
    requestSerialize: serialize_teleport_lib_teleterm_v1_CreateGatewayRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_CreateGatewayRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_Gateway,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_Gateway,
  },
  // RemoveGateway removes a gateway
removeGateway: {
    path: '/teleport.lib.teleterm.v1.TerminalService/RemoveGateway',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.EmptyResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_RemoveGatewayRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_RemoveGatewayRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_EmptyResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_EmptyResponse,
  },
  // SetGatewayTargetSubresourceName changes the TargetSubresourceName field of gateway.Gateway
// and returns the updated version of gateway.Gateway.
//
// In Connect this is used to update the db name of a db connection along with the CLI command.
setGatewayTargetSubresourceName: {
    path: '/teleport.lib.teleterm.v1.TerminalService/SetGatewayTargetSubresourceName',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest,
    responseType: teleport_lib_teleterm_v1_gateway_pb.Gateway,
    requestSerialize: serialize_teleport_lib_teleterm_v1_SetGatewayTargetSubresourceNameRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_SetGatewayTargetSubresourceNameRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_Gateway,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_Gateway,
  },
  // SetGatewayLocalPort starts a new gateway on the new port, stops the old gateway and then
// assigns the URI of the old gateway to the new one. It does so without fetching a new db cert.
setGatewayLocalPort: {
    path: '/teleport.lib.teleterm.v1.TerminalService/SetGatewayLocalPort',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest,
    responseType: teleport_lib_teleterm_v1_gateway_pb.Gateway,
    requestSerialize: serialize_teleport_lib_teleterm_v1_SetGatewayLocalPortRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_SetGatewayLocalPortRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_Gateway,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_Gateway,
  },
  // GetAuthSettings returns cluster auth settigns
getAuthSettings: {
    path: '/teleport.lib.teleterm.v1.TerminalService/GetAuthSettings',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest,
    responseType: teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings,
    requestSerialize: serialize_teleport_lib_teleterm_v1_GetAuthSettingsRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_GetAuthSettingsRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_AuthSettings,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_AuthSettings,
  },
  // GetCluster returns cluster. Makes a network request and includes detailed
// information about enterprise features availabed on the connected auth server
getCluster: {
    path: '/teleport.lib.teleterm.v1.TerminalService/GetCluster',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.GetClusterRequest,
    responseType: teleport_lib_teleterm_v1_cluster_pb.Cluster,
    requestSerialize: serialize_teleport_lib_teleterm_v1_GetClusterRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_GetClusterRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_Cluster,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_Cluster,
  },
  // Login logs in a user to a cluster
login: {
    path: '/teleport.lib.teleterm.v1.TerminalService/Login',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.LoginRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.EmptyResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_LoginRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_LoginRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_EmptyResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_EmptyResponse,
  },
  // LoginPasswordless logs in a user to a cluster passwordlessly.
//
// The RPC is streaming both ways and the message sequence example for hardware keys are:
// (-> means client-to-server, <- means server-to-client)
//
// Hardware keys:
// -> Init
// <- Send PasswordlessPrompt enum TAP to choose a device
// -> Receive TAP device response
// <- Send PasswordlessPrompt enum PIN
// -> Receive PIN response
// <- Send PasswordlessPrompt enum RETAP to confirm
// -> Receive RETAP device response
// <- Send list of credentials (e.g. usernames) associated with device
// -> Receive the index number associated with the selected credential in list
// <- End
loginPasswordless: {
    path: '/teleport.lib.teleterm.v1.TerminalService/LoginPasswordless',
    requestStream: true,
    responseStream: true,
    requestType: teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_LoginPasswordlessRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_LoginPasswordlessRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_LoginPasswordlessResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_LoginPasswordlessResponse,
  },
  // ClusterLogin logs out a user from cluster
logout: {
    path: '/teleport.lib.teleterm.v1.TerminalService/Logout',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.LogoutRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.EmptyResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_LogoutRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_LogoutRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_EmptyResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_EmptyResponse,
  },
  // TransferFile sends a request to download/upload a file
transferFile: {
    path: '/teleport.lib.teleterm.v1.TerminalService/TransferFile',
    requestStream: false,
    responseStream: true,
    requestType: teleport_lib_teleterm_v1_service_pb.FileTransferRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.FileTransferProgress,
    requestSerialize: serialize_teleport_lib_teleterm_v1_FileTransferRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_FileTransferRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_FileTransferProgress,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_FileTransferProgress,
  },
  // ReportUsageEvent allows to send usage events that are then anonymized and forwarded to prehog
reportUsageEvent: {
    path: '/teleport.lib.teleterm.v1.TerminalService/ReportUsageEvent',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.EmptyResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_ReportUsageEventRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_ReportUsageEventRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_EmptyResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_EmptyResponse,
  },
  // UpdateHeadlessAuthenticationState updates a headless authentication resource's state.
// An MFA challenge will be prompted when approving a headless authentication.
updateHeadlessAuthenticationState: {
    path: '/teleport.lib.teleterm.v1.TerminalService/UpdateHeadlessAuthenticationState',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest,
    responseType: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_UpdateHeadlessAuthenticationStateRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_UpdateHeadlessAuthenticationStateRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_UpdateHeadlessAuthenticationStateResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_UpdateHeadlessAuthenticationStateResponse,
  },
};

exports.TerminalServiceClient = grpc.makeGenericClientConstructor(TerminalServiceService);
