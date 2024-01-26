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
var teleport_lib_teleterm_v1_tshd_events_service_pb = require('../../../../teleport/lib/teleterm/v1/tshd_events_service_pb.js');

function serialize_teleport_lib_teleterm_v1_PromptMFARequest(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.PromptMFARequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_PromptMFARequest(buffer_arg) {
  return teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_lib_teleterm_v1_PromptMFAResponse(arg) {
  if (!(arg instanceof teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse)) {
    throw new Error('Expected argument of type teleport.lib.teleterm.v1.PromptMFAResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_lib_teleterm_v1_PromptMFAResponse(buffer_arg) {
  return teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

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
  // PromptMFA notifies the Electron app that the daemon is waiting for the user to answer an MFA prompt.
promptMFA: {
    path: '/teleport.lib.teleterm.v1.TshdEventsService/PromptMFA',
    requestStream: false,
    responseStream: false,
    requestType: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest,
    responseType: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse,
    requestSerialize: serialize_teleport_lib_teleterm_v1_PromptMFARequest,
    requestDeserialize: deserialize_teleport_lib_teleterm_v1_PromptMFARequest,
    responseSerialize: serialize_teleport_lib_teleterm_v1_PromptMFAResponse,
    responseDeserialize: deserialize_teleport_lib_teleterm_v1_PromptMFAResponse,
  },
};

exports.TshdEventsServiceClient = grpc.makeGenericClientConstructor(TshdEventsServiceService);
