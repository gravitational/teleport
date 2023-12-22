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
