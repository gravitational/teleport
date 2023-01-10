// GENERATED CODE -- DO NOT EDIT!

'use strict';
var grpc = require('@grpc/grpc-js');
var prehog_v1alpha_connect_pb = require('../../prehog/v1alpha/connect_pb.js');
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');

function serialize_prehog_v1alpha_SubmitConnectEventRequest(arg) {
  if (!(arg instanceof prehog_v1alpha_connect_pb.SubmitConnectEventRequest)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitConnectEventRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitConnectEventRequest(buffer_arg) {
  return prehog_v1alpha_connect_pb.SubmitConnectEventRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_prehog_v1alpha_SubmitConnectEventResponse(arg) {
  if (!(arg instanceof prehog_v1alpha_connect_pb.SubmitConnectEventResponse)) {
    throw new Error('Expected argument of type prehog.v1alpha.SubmitConnectEventResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_prehog_v1alpha_SubmitConnectEventResponse(buffer_arg) {
  return prehog_v1alpha_connect_pb.SubmitConnectEventResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var ConnectReportingServiceService = exports.ConnectReportingServiceService = {
  submitConnectEvent: {
    path: '/prehog.v1alpha.ConnectReportingService/SubmitConnectEvent',
    requestStream: false,
    responseStream: false,
    requestType: prehog_v1alpha_connect_pb.SubmitConnectEventRequest,
    responseType: prehog_v1alpha_connect_pb.SubmitConnectEventResponse,
    requestSerialize: serialize_prehog_v1alpha_SubmitConnectEventRequest,
    requestDeserialize: deserialize_prehog_v1alpha_SubmitConnectEventRequest,
    responseSerialize: serialize_prehog_v1alpha_SubmitConnectEventResponse,
    responseDeserialize: deserialize_prehog_v1alpha_SubmitConnectEventResponse,
  },
};

exports.ConnectReportingServiceClient = grpc.makeGenericClientConstructor(ConnectReportingServiceService);
