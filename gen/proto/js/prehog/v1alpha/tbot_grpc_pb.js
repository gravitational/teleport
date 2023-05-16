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
var prehog_v1alpha_tbot_pb = require('../../prehog/v1alpha/tbot_pb.js');
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');

function serialize_prehog_v1alpha_SubmitTbotEventRequest(arg) {
  if (!(arg instanceof prehog_v1alpha_tbot_pb.SubmitTbotEventRequest)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitTbotEventRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitTbotEventRequest(buffer_arg) {
  return prehog_v1alpha_tbot_pb.SubmitTbotEventRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_prehog_v1alpha_SubmitTbotEventResponse(arg) {
  if (!(arg instanceof prehog_v1alpha_tbot_pb.SubmitTbotEventResponse)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitTbotEventResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitTbotEventResponse(buffer_arg) {
  return prehog_v1alpha_tbot_pb.SubmitTbotEventResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var TbotReportingServiceService = exports.TbotReportingServiceService = {
  submitTbotEvent: {
    path: '/prehog.v1alpha.TbotReportingService/SubmitTbotEvent',
    requestStream: false,
    responseStream: false,
    requestType: prehog_v1alpha_tbot_pb.SubmitTbotEventRequest,
    responseType: prehog_v1alpha_tbot_pb.SubmitTbotEventResponse,
    requestSerialize: serialize_prehog_v1alpha_SubmitTbotEventRequest,
    requestDeserialize: deserialize_prehog_v1alpha_SubmitTbotEventRequest,
    responseSerialize: serialize_prehog_v1alpha_SubmitTbotEventResponse,
    responseDeserialize: deserialize_prehog_v1alpha_SubmitTbotEventResponse,
  },
};

exports.TbotReportingServiceClient = grpc.makeGenericClientConstructor(TbotReportingServiceService);
