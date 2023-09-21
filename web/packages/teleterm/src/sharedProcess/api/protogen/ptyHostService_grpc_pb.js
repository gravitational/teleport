// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2023 Gravitational, Inc
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
// TODO(ravicious): Before introducing any changes, move this file to the /proto dir and
// remove the generate-grpc-shared script.
//
'use strict';
var grpc = require('@grpc/grpc-js');
var ptyHostService_pb = require('./ptyHostService_pb.js');
var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');

function serialize_PtyClientEvent(arg) {
  if (!(arg instanceof ptyHostService_pb.PtyClientEvent)) {
    throw new Error('Expected argument of type PtyClientEvent');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_PtyClientEvent(buffer_arg) {
  return ptyHostService_pb.PtyClientEvent.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_PtyCreate(arg) {
  if (!(arg instanceof ptyHostService_pb.PtyCreate)) {
    throw new Error('Expected argument of type PtyCreate');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_PtyCreate(buffer_arg) {
  return ptyHostService_pb.PtyCreate.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_PtyCwd(arg) {
  if (!(arg instanceof ptyHostService_pb.PtyCwd)) {
    throw new Error('Expected argument of type PtyCwd');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_PtyCwd(buffer_arg) {
  return ptyHostService_pb.PtyCwd.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_PtyId(arg) {
  if (!(arg instanceof ptyHostService_pb.PtyId)) {
    throw new Error('Expected argument of type PtyId');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_PtyId(buffer_arg) {
  return ptyHostService_pb.PtyId.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_PtyServerEvent(arg) {
  if (!(arg instanceof ptyHostService_pb.PtyServerEvent)) {
    throw new Error('Expected argument of type PtyServerEvent');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_PtyServerEvent(buffer_arg) {
  return ptyHostService_pb.PtyServerEvent.deserializeBinary(new Uint8Array(buffer_arg));
}


var PtyHostService = exports.PtyHostService = {
  createPtyProcess: {
    path: '/PtyHost/CreatePtyProcess',
    requestStream: false,
    responseStream: false,
    requestType: ptyHostService_pb.PtyCreate,
    responseType: ptyHostService_pb.PtyId,
    requestSerialize: serialize_PtyCreate,
    requestDeserialize: deserialize_PtyCreate,
    responseSerialize: serialize_PtyId,
    responseDeserialize: deserialize_PtyId,
  },
  exchangeEvents: {
    path: '/PtyHost/ExchangeEvents',
    requestStream: true,
    responseStream: true,
    requestType: ptyHostService_pb.PtyClientEvent,
    responseType: ptyHostService_pb.PtyServerEvent,
    requestSerialize: serialize_PtyClientEvent,
    requestDeserialize: deserialize_PtyClientEvent,
    responseSerialize: serialize_PtyServerEvent,
    responseDeserialize: deserialize_PtyServerEvent,
  },
  getCwd: {
    path: '/PtyHost/GetCwd',
    requestStream: false,
    responseStream: false,
    requestType: ptyHostService_pb.PtyId,
    responseType: ptyHostService_pb.PtyCwd,
    requestSerialize: serialize_PtyId,
    requestDeserialize: deserialize_PtyId,
    responseSerialize: serialize_PtyCwd,
    responseDeserialize: deserialize_PtyCwd,
  },
};

exports.PtyHostClient = grpc.makeGenericClientConstructor(PtyHostService);
