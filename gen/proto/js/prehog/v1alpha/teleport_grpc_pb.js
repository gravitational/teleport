// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2022 Gravitational, Inc
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
var prehog_v1alpha_teleport_pb = require('../../prehog/v1alpha/teleport_pb.js');
var google_protobuf_duration_pb = require('google-protobuf/google/protobuf/duration_pb.js');
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');

function serialize_prehog_v1alpha_HelloTeleportRequest(arg) {
  if (!(arg instanceof prehog_v1alpha_teleport_pb.HelloTeleportRequest)) {
    throw new Error('Expected argument of type prehog.v1alpha.HelloTeleportRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_HelloTeleportRequest(buffer_arg) {
  return prehog_v1alpha_teleport_pb.HelloTeleportRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_prehog_v1alpha_HelloTeleportResponse(arg) {
  if (!(arg instanceof prehog_v1alpha_teleport_pb.HelloTeleportResponse)) {
    throw new Error('Expected argument of type prehog.v1alpha.HelloTeleportResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_HelloTeleportResponse(buffer_arg) {
  return prehog_v1alpha_teleport_pb.HelloTeleportResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_prehog_v1alpha_SubmitEventRequest(arg) {
  if (!(arg instanceof prehog_v1alpha_teleport_pb.SubmitEventRequest)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitEventRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitEventRequest(buffer_arg) {
  return prehog_v1alpha_teleport_pb.SubmitEventRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_prehog_v1alpha_SubmitEventResponse(arg) {
  if (!(arg instanceof prehog_v1alpha_teleport_pb.SubmitEventResponse)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitEventResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitEventResponse(buffer_arg) {
  return prehog_v1alpha_teleport_pb.SubmitEventResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_prehog_v1alpha_SubmitEventsRequest(arg) {
  if (!(arg instanceof prehog_v1alpha_teleport_pb.SubmitEventsRequest)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitEventsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitEventsRequest(buffer_arg) {
  return prehog_v1alpha_teleport_pb.SubmitEventsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_prehog_v1alpha_SubmitEventsResponse(arg) {
  if (!(arg instanceof prehog_v1alpha_teleport_pb.SubmitEventsResponse)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitEventsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitEventsResponse(buffer_arg) {
  return prehog_v1alpha_teleport_pb.SubmitEventsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var TeleportReportingServiceService = exports.TeleportReportingServiceService = {
  // equivalent to SubmitEvents with a single event, should be unused by now
submitEvent: {
    path: '/prehog.v1alpha.TeleportReportingService/SubmitEvent',
    requestStream: false,
    responseStream: false,
    requestType: prehog_v1alpha_teleport_pb.SubmitEventRequest,
    responseType: prehog_v1alpha_teleport_pb.SubmitEventResponse,
    requestSerialize: serialize_prehog_v1alpha_SubmitEventRequest,
    requestDeserialize: deserialize_prehog_v1alpha_SubmitEventRequest,
    responseSerialize: serialize_prehog_v1alpha_SubmitEventResponse,
    responseDeserialize: deserialize_prehog_v1alpha_SubmitEventResponse,
  },
  // encodes and forwards usage events to the PostHog event database; each
// event is annotated with some properties that depend on the identity of the
// caller:
// - tp.account_id (UUID in string form, can be empty if missing from the
//   license)
// - tp.license_name (should always be a UUID)
// - tp.license_authority (name of the authority that signed the license file
//   used for authentication)
// - tp.is_cloud (boolean)
submitEvents: {
    path: '/prehog.v1alpha.TeleportReportingService/SubmitEvents',
    requestStream: false,
    responseStream: false,
    requestType: prehog_v1alpha_teleport_pb.SubmitEventsRequest,
    responseType: prehog_v1alpha_teleport_pb.SubmitEventsResponse,
    requestSerialize: serialize_prehog_v1alpha_SubmitEventsRequest,
    requestDeserialize: deserialize_prehog_v1alpha_SubmitEventsRequest,
    responseSerialize: serialize_prehog_v1alpha_SubmitEventsResponse,
    responseDeserialize: deserialize_prehog_v1alpha_SubmitEventsResponse,
  },
  helloTeleport: {
    path: '/prehog.v1alpha.TeleportReportingService/HelloTeleport',
    requestStream: false,
    responseStream: false,
    requestType: prehog_v1alpha_teleport_pb.HelloTeleportRequest,
    responseType: prehog_v1alpha_teleport_pb.HelloTeleportResponse,
    requestSerialize: serialize_prehog_v1alpha_HelloTeleportRequest,
    requestDeserialize: deserialize_prehog_v1alpha_HelloTeleportRequest,
    responseSerialize: serialize_prehog_v1alpha_HelloTeleportResponse,
    responseDeserialize: deserialize_prehog_v1alpha_HelloTeleportResponse,
  },
};

exports.TeleportReportingServiceClient = grpc.makeGenericClientConstructor(TeleportReportingServiceService);
