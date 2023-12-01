// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
