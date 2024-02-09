// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
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
//
'use strict';
var grpc = require('@grpc/grpc-js');
var teleport_lib_teleterm_vnet_v1_vnet_service_pb = require('../../../../../teleport/lib/teleterm/vnet/v1/vnet_service_pb.js');

function serialize_teleport_lib_teleterm_vnet_v1_StartRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.vnet.v1.StartRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_vnet_v1_StartRequest(buffer_arg) {
  return teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_vnet_v1_StartResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.vnet.v1.StartResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_vnet_v1_StartResponse(buffer_arg) {
  return teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_vnet_v1_StopRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.vnet.v1.StopRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_vnet_v1_StopRequest(buffer_arg) {
  return teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_vnet_v1_StopResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.vnet.v1.StopResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_vnet_v1_StopResponse(buffer_arg) {
  return teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// VnetService provides methods to manage a VNet instance. Only one VNet instance can be active at a
// time.
var VnetServiceService = exports.VnetServiceService = {
  // Start starts VNet for the given root cluster.
start: {
    path: '/teleport.lib.teleterm.vnet.v1.VnetService/Start',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest,
    responseType: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse,
    requestSerialize: serialize_teleport_lib_teleterm_vnet_v1_StartRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_vnet_v1_StartRequest,
    responseSerialize: serialize_teleport_lib_teleterm_vnet_v1_StartResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_vnet_v1_StartResponse,
  },
  // Stop stops VNet for the given root cluster.
stop: {
    path: '/teleport.lib.teleterm.vnet.v1.VnetService/Stop',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest,
    responseType: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse,
    requestSerialize: serialize_teleport_lib_teleterm_vnet_v1_StopRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_vnet_v1_StopRequest,
    responseSerialize: serialize_teleport_lib_teleterm_vnet_v1_StopResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_vnet_v1_StopResponse,
  },
};

exports.VnetServiceClient = grpc.makeGenericClientConstructor(VnetServiceService);
