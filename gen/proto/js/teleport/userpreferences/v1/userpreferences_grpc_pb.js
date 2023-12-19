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
'use strict';
var grpc = require('@grpc/grpc-js');
var teleport_userpreferences_v1_userpreferences_pb = require('../../../teleport/userpreferences/v1/userpreferences_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var teleport_userpreferences_v1_assist_pb = require('../../../teleport/userpreferences/v1/assist_pb.js');
var teleport_userpreferences_v1_cluster_preferences_pb = require('../../../teleport/userpreferences/v1/cluster_preferences_pb.js');
var teleport_userpreferences_v1_onboard_pb = require('../../../teleport/userpreferences/v1/onboard_pb.js');
var teleport_userpreferences_v1_theme_pb = require('../../../teleport/userpreferences/v1/theme_pb.js');
var teleport_userpreferences_v1_unified_resource_preferences_pb = require('../../../teleport/userpreferences/v1/unified_resource_preferences_pb.js');

function serialize_google_protobuf_Empty(arg) {
  if (!(arg instanceof google_protobuf_empty_pb.Empty)) {
    throw new Error('Expected argument of type google.protobuf.Empty');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_google_protobuf_Empty(buffer_arg) {
  return google_protobuf_empty_pb.Empty.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_userpreferences_v1_GetUserPreferencesRequest(arg) {
  if (!(arg instanceof teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest)) {
    throw new Error('Expected argument of type teleport.userpreferences.v1.GetUserPreferencesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_userpreferences_v1_GetUserPreferencesRequest(buffer_arg) {
  return teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_userpreferences_v1_GetUserPreferencesResponse(arg) {
  if (!(arg instanceof teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse)) {
    throw new Error('Expected argument of type teleport.userpreferences.v1.GetUserPreferencesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_userpreferences_v1_GetUserPreferencesResponse(buffer_arg) {
  return teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_userpreferences_v1_UpsertUserPreferencesRequest(arg) {
  if (!(arg instanceof teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest)) {
    throw new Error('Expected argument of type teleport.userpreferences.v1.UpsertUserPreferencesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_userpreferences_v1_UpsertUserPreferencesRequest(buffer_arg) {
  return teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}


// UserPreferencesService is a service that stores user settings.
var UserPreferencesServiceService = exports.UserPreferencesServiceService = {
  // GetUserPreferences returns the user preferences for a given user.
getUserPreferences: {
    path: '/teleport.userpreferences.v1.UserPreferencesService/GetUserPreferences',
    requestStream: false,
    responseStream: false,
    requestType: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest,
    responseType: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse,
    requestSerialize: serialize_teleport_userpreferences_v1_GetUserPreferencesRequest,
    requestDeserialize: deserialize_teleport_userpreferences_v1_GetUserPreferencesRequest,
    responseSerialize: serialize_teleport_userpreferences_v1_GetUserPreferencesResponse,
    responseDeserialize: deserialize_teleport_userpreferences_v1_GetUserPreferencesResponse,
  },
  // UpsertUserPreferences creates or updates user preferences for a given username.
upsertUserPreferences: {
    path: '/teleport.userpreferences.v1.UserPreferencesService/UpsertUserPreferences',
    requestStream: false,
    responseStream: false,
    requestType: teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_teleport_userpreferences_v1_UpsertUserPreferencesRequest,
    requestDeserialize: deserialize_teleport_userpreferences_v1_UpsertUserPreferencesRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
};

exports.UserPreferencesServiceClient = grpc.makeGenericClientConstructor(UserPreferencesServiceService);
