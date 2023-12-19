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
  // encodes and forwards usage reports to the PostHog event database; each
// event is annotated with some properties that depend on the identity of the
// caller:
// - tp.account_id (UUID in string form, can be empty if missing from the
//   license)
// - tp.license_name (should always be a UUID)
// - tp.license_authority (name of the authority that signed the license file
//   used for authentication)
// - tp.is_cloud (boolean)
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
