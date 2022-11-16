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
var v1_tshd_events_service_pb = require('../v1/tshd_events_service_pb.js');

function serialize_teleport_terminal_v1_TestRequest(arg) {
  if (!(arg instanceof v1_tshd_events_service_pb.TestRequest)) {
    throw new Error('Expected argument of type teleport.terminal.v1.TestRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_terminal_v1_TestRequest(buffer_arg) {
  return v1_tshd_events_service_pb.TestRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_terminal_v1_TestResponse(arg) {
  if (!(arg instanceof v1_tshd_events_service_pb.TestResponse)) {
    throw new Error('Expected argument of type teleport.terminal.v1.TestResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_terminal_v1_TestResponse(buffer_arg) {
  return v1_tshd_events_service_pb.TestResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// TshdEventsService is served by the Electron app. The tsh daemon calls this service to notify the
// app about actions that happen outside of the app itself. For example, when the user tries to
// connect to a gateway served by the daemon but the cert has since expired and needs to be
// reissued.
var TshdEventsServiceService = exports.TshdEventsServiceService = {
  // Test is an RPC that's used to demonstrate how the implementation of a tshd event may look like
// from the beginning till the end.
// TODO(ravicious): Remove this once we add an actual RPC to tshd events service.
test: {
    path: '/teleport.terminal.v1.TshdEventsService/Test',
    requestStream: false,
    responseStream: false,
    requestType: v1_tshd_events_service_pb.TestRequest,
    responseType: v1_tshd_events_service_pb.TestResponse,
    requestSerialize: serialize_teleport_terminal_v1_TestRequest,
    requestDeserialize: deserialize_teleport_terminal_v1_TestRequest,
    responseSerialize: serialize_teleport_terminal_v1_TestResponse,
    responseDeserialize: deserialize_teleport_terminal_v1_TestResponse,
  },
};

exports.TshdEventsServiceClient = grpc.makeGenericClientConstructor(TshdEventsServiceService);
