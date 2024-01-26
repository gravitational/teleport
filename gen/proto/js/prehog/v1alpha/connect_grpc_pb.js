// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
//
// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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
var prehog_v1alpha_connect_pb = require('../../prehog/v1alpha/connect_pb.js');
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');

function serialize_prehog_v1alpha_SubmitConnectEventRequest(arg) {
  if (!(arg instanceof prehog_v1alpha_connect_pb.SubmitConnectEventRequest)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitConnectEventRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitConnectEventRequest(buffer_arg) {
  return prehog_v1alpha_connect_pb.SubmitConnectEventRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_prehog_v1alpha_SubmitConnectEventResponse(arg) {
  if (!(arg instanceof prehog_v1alpha_connect_pb.SubmitConnectEventResponse)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitConnectEventResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitConnectEventResponse(buffer_arg) {
  return prehog_v1alpha_connect_pb.SubmitConnectEventResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var ConnectReportingServiceService = exports.ConnectReportingServiceService = {
  submitConnectEvent: {
    path: '/prehog.v1alpha.ConnectReportingService/SubmitConnectEvent',
    requestStream: false,
    responseStream: false,
    requestType: prehog_v1alpha_connect_pb.SubmitConnectEventRequest,
    responseType: prehog_v1alpha_connect_pb.SubmitConnectEventResponse,
    requestSerialize: serialize_prehog_v1alpha_SubmitConnectEventRequest,
    requestDeserialize: deserialize_prehog_v1alpha_SubmitConnectEventRequest,
    responseSerialize: serialize_prehog_v1alpha_SubmitConnectEventResponse,
    responseDeserialize: deserialize_prehog_v1alpha_SubmitConnectEventResponse,
  },
};

exports.ConnectReportingServiceClient = grpc.makeGenericClientConstructor(ConnectReportingServiceService);
