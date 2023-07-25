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
var teleport_lib_teleterm_v1_tshd_events_service_pb = require('../../../../teleport/lib/teleterm/v1/tshd_events_service_pb.js');

function serialize_teleport_lib_teleterm_v1_ReloginRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ReloginRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ReloginRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_ReloginResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.ReloginResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_ReloginResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_SendNotificationRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.SendNotificationRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_SendNotificationRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_SendNotificationResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.SendNotificationResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_SendNotificationResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_SendPendingHeadlessAuthenticationRequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_tshd_events_service_pb.SendPendingHeadlessAuthenticationRequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.SendPendingHeadlessAuthenticationRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_SendPendingHeadlessAuthenticationRequest(buffer_arg) {
  return teleport_lib_teleterm_v1_tshd_events_service_pb.SendPendingHeadlessAuthenticationRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_SendPendingHeadlessAuthenticationResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_tshd_events_service_pb.SendPendingHeadlessAuthenticationResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.SendPendingHeadlessAuthenticationResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_SendPendingHeadlessAuthenticationResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_tshd_events_service_pb.SendPendingHeadlessAuthenticationResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// TshdEventsService is served by the Electron app. The tsh daemon calls this service to notify the
// app about actions that happen outside of the app itself.
var TshdEventsServiceService = exports.TshdEventsServiceService = {
  // Relogin makes the Electron app display a login modal for the specific root cluster. The request
// returns a response after the relogin procedure has been successfully finished.
relogin: {
    path: '/teleport.lib.teleterm.v1.TshdEventsService/Relogin',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest,
    responseType: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_ReloginRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_ReloginRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_ReloginResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_ReloginResponse,
  },
  // SendNotification causes the Electron app to display a notification in the UI. The request
// accepts a specific message rather than a generic string so that the Electron is in control as
// to what message is displayed and how exactly it looks.
sendNotification: {
    path: '/teleport.lib.teleterm.v1.TshdEventsService/SendNotification',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest,
    responseType: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_SendNotificationRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_SendNotificationRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_SendNotificationResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_SendNotificationResponse,
  },
  // SendPendingHeadlessAuthentication notifies the Electron app of a pending headless authentication,
// which it can use to initiate headless authentication resolution in the UI.
sendPendingHeadlessAuthentication: {
    path: '/teleport.lib.teleterm.v1.TshdEventsService/SendPendingHeadlessAuthentication',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_tshd_events_service_pb.SendPendingHeadlessAuthenticationRequest,
    responseType: teleport_lib_teleterm_v1_tshd_events_service_pb.SendPendingHeadlessAuthenticationResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_SendPendingHeadlessAuthenticationRequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_SendPendingHeadlessAuthenticationRequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_SendPendingHeadlessAuthenticationResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_SendPendingHeadlessAuthenticationResponse,
  },
};

exports.TshdEventsServiceClient = grpc.makeGenericClientConstructor(TshdEventsServiceService);
