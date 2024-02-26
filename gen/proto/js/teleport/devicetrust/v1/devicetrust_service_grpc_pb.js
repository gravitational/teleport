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
var teleport_devicetrust_v1_devicetrust_service_pb = require('../../../teleport/devicetrust/v1/devicetrust_service_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_field_mask_pb = require('google-protobuf/google/protobuf/field_mask_pb.js');
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');
var google_rpc_status_pb = require('../../../google/rpc/status_pb.js');
var teleport_devicetrust_v1_device_pb = require('../../../teleport/devicetrust/v1/device_pb.js');
var teleport_devicetrust_v1_device_collected_data_pb = require('../../../teleport/devicetrust/v1/device_collected_data_pb.js');
var teleport_devicetrust_v1_device_enroll_token_pb = require('../../../teleport/devicetrust/v1/device_enroll_token_pb.js');
var teleport_devicetrust_v1_device_source_pb = require('../../../teleport/devicetrust/v1/device_source_pb.js');
var teleport_devicetrust_v1_device_web_token_pb = require('../../../teleport/devicetrust/v1/device_web_token_pb.js');
var teleport_devicetrust_v1_tpm_pb = require('../../../teleport/devicetrust/v1/tpm_pb.js');
var teleport_devicetrust_v1_usage_pb = require('../../../teleport/devicetrust/v1/usage_pb.js');
var teleport_devicetrust_v1_user_certificates_pb = require('../../../teleport/devicetrust/v1/user_certificates_pb.js');

function serialize_google_protobuf_Empty(arg) {
  if (!(arg instanceof google_protobuf_empty_pb.Empty)) {
    throw new Error('Expected argument of type google.protobuf.Empty');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_google_protobuf_Empty(buffer_arg) {
  return google_protobuf_empty_pb.Empty.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_AuthenticateDeviceRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.AuthenticateDeviceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_AuthenticateDeviceRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_AuthenticateDeviceResponse(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.AuthenticateDeviceResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_AuthenticateDeviceResponse(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_BulkCreateDevicesRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.BulkCreateDevicesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_BulkCreateDevicesRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_BulkCreateDevicesResponse(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.BulkCreateDevicesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_BulkCreateDevicesResponse(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_CreateDeviceEnrollTokenRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_CreateDeviceEnrollTokenRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_CreateDeviceRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.CreateDeviceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_CreateDeviceRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_DeleteDeviceRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.DeleteDeviceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_DeleteDeviceRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_Device(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_device_pb.Device)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.Device');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_Device(buffer_arg) {
  return teleport_devicetrust_v1_device_pb.Device.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_DeviceEnrollToken(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.DeviceEnrollToken');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_DeviceEnrollToken(buffer_arg) {
  return teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_DevicesUsage(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_usage_pb.DevicesUsage)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.DevicesUsage');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_DevicesUsage(buffer_arg) {
  return teleport_devicetrust_v1_usage_pb.DevicesUsage.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_EnrollDeviceRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.EnrollDeviceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_EnrollDeviceRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_EnrollDeviceResponse(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.EnrollDeviceResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_EnrollDeviceResponse(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_FindDevicesRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.FindDevicesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_FindDevicesRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_FindDevicesResponse(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.FindDevicesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_FindDevicesResponse(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_GetDeviceRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.GetDeviceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_GetDeviceRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_GetDevicesUsageRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.GetDevicesUsageRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_GetDevicesUsageRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_ListDevicesRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.ListDevicesRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_ListDevicesRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_ListDevicesResponse(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.ListDevicesResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_ListDevicesResponse(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_SyncInventoryRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.SyncInventoryRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_SyncInventoryRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_SyncInventoryResponse(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.SyncInventoryResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_SyncInventoryResponse(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_UpdateDeviceRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.UpdateDeviceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_UpdateDeviceRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_devicetrust_v1_UpsertDeviceRequest(arg) {
  if (!(arg instanceof teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest)) {
    throw new Error('Expected argument of type teleport.devicetrust.v1.UpsertDeviceRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_devicetrust_v1_UpsertDeviceRequest(buffer_arg) {
  return teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest.deserializeBinary(new Uint8Array(buffer_arg));
}


// DeviceTrustService provides methods to manage, enroll and authenticate
// trusted devices.
//
// A trusted device is a device that is registered and enrolled with Teleport,
// thus allowing the system to provide some guarantees about its provenance and
// state.
//
// Managing devices requires the corresponding CRUD "device" permission.
// Additionally, creating enrollment tokens requires the "create_enroll_token"
// permission and enrolling devices requires the "enroll" permission. See
// CreateDevice, CreateDeviceEnrollToken and EnrollDevice for reference.
//
// An authenticated, trusted device allows its user to perform device-aware
// actions. Such actions include accessing an SSH node, managing sensitive
// resources via `tctl`, etc. The enforcement mode is defined via cluster-wide
// and/or per-role toggles. Device authentication is automatic for enrolled
// devices communicating with Enterprise clusters. See AuthenticateDevice for
// reference.
//
// Device Trust is a Teleport Enterprise feature. Open Source Teleport clusters
// treat all Device RPCs as unimplemented (which, in fact, they are for OSS.)
var DeviceTrustServiceService = exports.DeviceTrustServiceService = {
  // CreateDevice creates a device, effectively registering it on Teleport.
// Devices need to be registered before they can be enrolled.
//
// It is possible to create both a Device and a DeviceEnrollToken in a
// single invocation, see CreateDeviceRequest.create_enroll_token.
createDevice: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/CreateDevice',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest,
    responseType: teleport_devicetrust_v1_device_pb.Device,
    requestSerialize: serialize_teleport_devicetrust_v1_CreateDeviceRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_CreateDeviceRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_Device,
    responseDeserialize: deserialize_teleport_devicetrust_v1_Device,
  },
  // UpdateDevice is a masked device update.
//
// Only certain fields may be updated, see Device for details.
updateDevice: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/UpdateDevice',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest,
    responseType: teleport_devicetrust_v1_device_pb.Device,
    requestSerialize: serialize_teleport_devicetrust_v1_UpdateDeviceRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_UpdateDeviceRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_Device,
    responseDeserialize: deserialize_teleport_devicetrust_v1_Device,
  },
  // UpsertDevice creates or updates a device.
//
// UpsertDevice attempts a write of all mutable fields on updates, therefore
// reading a fresh copy of the device is recommended. Update semantics still
// apply.
upsertDevice: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/UpsertDevice',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest,
    responseType: teleport_devicetrust_v1_device_pb.Device,
    requestSerialize: serialize_teleport_devicetrust_v1_UpsertDeviceRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_UpsertDeviceRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_Device,
    responseDeserialize: deserialize_teleport_devicetrust_v1_Device,
  },
  // DeleteDevice hard-deletes a device, removing it and all collected data
// history from the system.
//
// Prefer locking the device instead (see the `tctl lock` command). Deleting a
// device doesn't invalidate existing device certificates, but does prevent
// new device authentication ceremonies from occurring.
//
// Use with caution.
deleteDevice: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/DeleteDevice',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_teleport_devicetrust_v1_DeleteDeviceRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_DeleteDeviceRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // FindDevices retrieves devices by device ID and/or asset tag.
//
// It provides an in-between search between fetching a device by ID and
// listing all devices.
//
// ID matches are guaranteed to be present in the response.
findDevices: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/FindDevices',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest,
    responseType: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse,
    requestSerialize: serialize_teleport_devicetrust_v1_FindDevicesRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_FindDevicesRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_FindDevicesResponse,
    responseDeserialize: deserialize_teleport_devicetrust_v1_FindDevicesResponse,
  },
  // GetDevice retrieves a device by ID.
getDevice: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/GetDevice',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest,
    responseType: teleport_devicetrust_v1_device_pb.Device,
    requestSerialize: serialize_teleport_devicetrust_v1_GetDeviceRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_GetDeviceRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_Device,
    responseDeserialize: deserialize_teleport_devicetrust_v1_Device,
  },
  // ListDevices lists all registered devices.
listDevices: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/ListDevices',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest,
    responseType: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse,
    requestSerialize: serialize_teleport_devicetrust_v1_ListDevicesRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_ListDevicesRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_ListDevicesResponse,
    responseDeserialize: deserialize_teleport_devicetrust_v1_ListDevicesResponse,
  },
  // BulkCreateDevices is a bulk variant of CreateDevice.
//
// Unlike CreateDevice, it does not support creation of enrollment tokens, as
// it is meant for bulk inventory registration.
bulkCreateDevices: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/BulkCreateDevices',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest,
    responseType: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse,
    requestSerialize: serialize_teleport_devicetrust_v1_BulkCreateDevicesRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_BulkCreateDevicesRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_BulkCreateDevicesResponse,
    responseDeserialize: deserialize_teleport_devicetrust_v1_BulkCreateDevicesResponse,
  },
  // CreateDeviceEnrollToken creates a DeviceEnrollToken for a Device.
// An enrollment token is required for the enrollment ceremony. See
// EnrollDevice.
createDeviceEnrollToken: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/CreateDeviceEnrollToken',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest,
    responseType: teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken,
    requestSerialize: serialize_teleport_devicetrust_v1_CreateDeviceEnrollTokenRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_CreateDeviceEnrollTokenRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_DeviceEnrollToken,
    responseDeserialize: deserialize_teleport_devicetrust_v1_DeviceEnrollToken,
  },
  // EnrollDevice performs the device enrollment ceremony.
//
// Enrollment requires a previously-registered Device and a DeviceEnrollToken,
// see CreateDevice and CreateDeviceEnrollToken.
//
// An enrolled device is allowed, via AuthenticateDevice, to acquire
// certificates containing device extensions, thus gaining access to
// device-aware actions.
//
// macOS enrollment flow:
// -> EnrollDeviceInit (client)
// <- MacOSEnrollChallenge (server)
// -> MacOSEnrollChallengeResponse
// <- EnrollDeviceSuccess
//
// TPM enrollment flow:
// -> EnrollDeviceInit (client)
// <- TPMEnrollChallenge (server)
// -> TPMEnrollChallengeResponse
// <- EnrollDeviceSuccess
enrollDevice: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/EnrollDevice',
    requestStream: true,
    responseStream: true,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest,
    responseType: teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse,
    requestSerialize: serialize_teleport_devicetrust_v1_EnrollDeviceRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_EnrollDeviceRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_EnrollDeviceResponse,
    responseDeserialize: deserialize_teleport_devicetrust_v1_EnrollDeviceResponse,
  },
  // AuthenticateDevice performs the device authentication ceremony.
//
// Device authentication exchanges existing user certificates without device
// extensions for certificates augmented with device extensions. The new
// certificates allow the user to perform device-aware actions.
//
// Only registered and enrolled devices may perform device authentication.
authenticateDevice: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/AuthenticateDevice',
    requestStream: true,
    responseStream: true,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest,
    responseType: teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse,
    requestSerialize: serialize_teleport_devicetrust_v1_AuthenticateDeviceRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_AuthenticateDeviceRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_AuthenticateDeviceResponse,
    responseDeserialize: deserialize_teleport_devicetrust_v1_AuthenticateDeviceResponse,
  },
  // Syncs device inventory from a source exterior to Teleport, for example an
// MDM.
// Allows both partial and full syncs; for the latter, devices missing from
// the external inventory are handled as specified.
// Authorized either by a valid MDM service certificate or the appropriate
// "device" permissions (create/update/delete).
syncInventory: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/SyncInventory',
    requestStream: true,
    responseStream: true,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest,
    responseType: teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse,
    requestSerialize: serialize_teleport_devicetrust_v1_SyncInventoryRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_SyncInventoryRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_SyncInventoryResponse,
    responseDeserialize: deserialize_teleport_devicetrust_v1_SyncInventoryResponse,
  },
  // Superseded by ResourceUsageService.GetUsage.
getDevicesUsage: {
    path: '/teleport.devicetrust.v1.DeviceTrustService/GetDevicesUsage',
    requestStream: false,
    responseStream: false,
    requestType: teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest,
    responseType: teleport_devicetrust_v1_usage_pb.DevicesUsage,
    requestSerialize: serialize_teleport_devicetrust_v1_GetDevicesUsageRequest,
    requestDeserialize: deserialize_teleport_devicetrust_v1_GetDevicesUsageRequest,
    responseSerialize: serialize_teleport_devicetrust_v1_DevicesUsage,
    responseDeserialize: deserialize_teleport_devicetrust_v1_DevicesUsage,
  },
};

exports.DeviceTrustServiceClient = grpc.makeGenericClientConstructor(DeviceTrustServiceService);
