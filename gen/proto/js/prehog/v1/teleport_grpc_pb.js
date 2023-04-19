// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2023 Gravitational, Inc
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
//
'use strict';
var grpc = require('@grpc/grpc-js');
var prehog_v1_teleport_pb = require('../../prehog/v1/teleport_pb.js');
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');

function serialize_prehog_v1_SubmitUsageReportsRequest(arg) {
  if (!(arg instanceof prehog_v1_teleport_pb.SubmitUsageReportsRequest)) {
    throw new Error('Expected argument of type prehog.v1.SubmitUsageReportsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1_SubmitUsageReportsRequest(buffer_arg) {
  return prehog_v1_teleport_pb.SubmitUsageReportsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_prehog_v1_SubmitUsageReportsResponse(arg) {
  if (!(arg instanceof prehog_v1_teleport_pb.SubmitUsageReportsResponse)) {
    throw new Error('Expected argument of type prehog.v1.SubmitUsageReportsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1_SubmitUsageReportsResponse(buffer_arg) {
  return prehog_v1_teleport_pb.SubmitUsageReportsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var TeleportReportingServiceService = exports.TeleportReportingServiceService = {
  submitUsageReports: {
    path: '/prehog.v1.TeleportReportingService/SubmitUsageReports',
    requestStream: false,
    responseStream: false,
    requestType: prehog_v1_teleport_pb.SubmitUsageReportsRequest,
    responseType: prehog_v1_teleport_pb.SubmitUsageReportsResponse,
    requestSerialize: serialize_prehog_v1_SubmitUsageReportsRequest,
    requestDeserialize: deserialize_prehog_v1_SubmitUsageReportsRequest,
    responseSerialize: serialize_prehog_v1_SubmitUsageReportsResponse,
    responseDeserialize: deserialize_prehog_v1_SubmitUsageReportsResponse,
  },
};

exports.TeleportReportingServiceClient = grpc.makeGenericClientConstructor(TeleportReportingServiceService);
