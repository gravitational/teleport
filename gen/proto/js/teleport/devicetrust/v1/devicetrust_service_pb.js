// source: teleport/devicetrust/v1/devicetrust_service.proto
/**
 * @fileoverview
 * @enhanceable
 * @suppress {missingRequire} reports error on implicit type usages.
 * @suppress {messageConventions} JS Compiler reports an error if a variable or
 *     field starts with 'MSG_' and isn't a translatable message.
 * @public
 */
// GENERATED CODE -- DO NOT EDIT!
/* eslint-disable */
// @ts-nocheck

var jspb = require('google-protobuf');
var goog = jspb;
var global = (function() { return this || window || global || self || Function('return this')(); }).call(null);

var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
goog.object.extend(proto, google_protobuf_empty_pb);
var google_protobuf_field_mask_pb = require('google-protobuf/google/protobuf/field_mask_pb.js');
goog.object.extend(proto, google_protobuf_field_mask_pb);
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');
goog.object.extend(proto, google_protobuf_timestamp_pb);
var google_rpc_status_pb = require('../../../google/rpc/status_pb.js');
goog.object.extend(proto, google_rpc_status_pb);
var teleport_devicetrust_v1_device_pb = require('../../../teleport/devicetrust/v1/device_pb.js');
goog.object.extend(proto, teleport_devicetrust_v1_device_pb);
var teleport_devicetrust_v1_device_collected_data_pb = require('../../../teleport/devicetrust/v1/device_collected_data_pb.js');
goog.object.extend(proto, teleport_devicetrust_v1_device_collected_data_pb);
var teleport_devicetrust_v1_device_enroll_token_pb = require('../../../teleport/devicetrust/v1/device_enroll_token_pb.js');
goog.object.extend(proto, teleport_devicetrust_v1_device_enroll_token_pb);
var teleport_devicetrust_v1_device_source_pb = require('../../../teleport/devicetrust/v1/device_source_pb.js');
goog.object.extend(proto, teleport_devicetrust_v1_device_source_pb);
var teleport_devicetrust_v1_device_web_token_pb = require('../../../teleport/devicetrust/v1/device_web_token_pb.js');
goog.object.extend(proto, teleport_devicetrust_v1_device_web_token_pb);
var teleport_devicetrust_v1_tpm_pb = require('../../../teleport/devicetrust/v1/tpm_pb.js');
goog.object.extend(proto, teleport_devicetrust_v1_tpm_pb);
var teleport_devicetrust_v1_usage_pb = require('../../../teleport/devicetrust/v1/usage_pb.js');
goog.object.extend(proto, teleport_devicetrust_v1_usage_pb);
var teleport_devicetrust_v1_user_certificates_pb = require('../../../teleport/devicetrust/v1/user_certificates_pb.js');
goog.object.extend(proto, teleport_devicetrust_v1_user_certificates_pb);
goog.exportSymbol('proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.AuthenticateDeviceInit', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.AuthenticateDeviceRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.PayloadCase', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.AuthenticateDeviceResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.PayloadCase', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.BulkCreateDevicesRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.BulkCreateDevicesResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.CreateDeviceRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.DeleteDeviceRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.DeviceOrStatus', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.DeviceView', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.EnrollDeviceInit', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.EnrollDeviceRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.EnrollDeviceRequest.PayloadCase', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.EnrollDeviceResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.EnrollDeviceResponse.PayloadCase', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.EnrollDeviceSuccess', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.FindDevicesRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.FindDevicesResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.GetDeviceRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.GetDevicesUsageRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.ListDevicesRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.ListDevicesResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.MacOSEnrollChallenge', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.MacOSEnrollPayload', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryAck', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryDevices', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryEnd', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryMissingDevices', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryRequest.PayloadCase', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryResponse.PayloadCase', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryResult', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.SyncInventoryStart', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.TPMAttestationParameters', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.TPMEncryptedCredential', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.TPMEnrollChallenge', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.TPMEnrollPayload', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.TPMEnrollPayload.EkCase', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.UpdateDeviceRequest', null, global);
goog.exportSymbol('proto.teleport.devicetrust.v1.UpsertDeviceRequest', null, global);
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.CreateDeviceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.CreateDeviceRequest.displayName = 'proto.teleport.devicetrust.v1.CreateDeviceRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.UpdateDeviceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.UpdateDeviceRequest.displayName = 'proto.teleport.devicetrust.v1.UpdateDeviceRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.UpsertDeviceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.UpsertDeviceRequest.displayName = 'proto.teleport.devicetrust.v1.UpsertDeviceRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.DeleteDeviceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.DeleteDeviceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.DeleteDeviceRequest.displayName = 'proto.teleport.devicetrust.v1.DeleteDeviceRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.FindDevicesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.FindDevicesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.FindDevicesRequest.displayName = 'proto.teleport.devicetrust.v1.FindDevicesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.FindDevicesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.devicetrust.v1.FindDevicesResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.devicetrust.v1.FindDevicesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.FindDevicesResponse.displayName = 'proto.teleport.devicetrust.v1.FindDevicesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.GetDeviceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.GetDeviceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.GetDeviceRequest.displayName = 'proto.teleport.devicetrust.v1.GetDeviceRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.ListDevicesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.ListDevicesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.ListDevicesRequest.displayName = 'proto.teleport.devicetrust.v1.ListDevicesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.ListDevicesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.devicetrust.v1.ListDevicesResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.devicetrust.v1.ListDevicesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.ListDevicesResponse.displayName = 'proto.teleport.devicetrust.v1.ListDevicesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.repeatedFields_, null);
};
goog.inherits(proto.teleport.devicetrust.v1.BulkCreateDevicesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.displayName = 'proto.teleport.devicetrust.v1.BulkCreateDevicesRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.devicetrust.v1.BulkCreateDevicesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.displayName = 'proto.teleport.devicetrust.v1.BulkCreateDevicesResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.DeviceOrStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.DeviceOrStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.DeviceOrStatus.displayName = 'proto.teleport.devicetrust.v1.DeviceOrStatus';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.displayName = 'proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.devicetrust.v1.EnrollDeviceRequest.oneofGroups_);
};
goog.inherits(proto.teleport.devicetrust.v1.EnrollDeviceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.EnrollDeviceRequest.displayName = 'proto.teleport.devicetrust.v1.EnrollDeviceRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.devicetrust.v1.EnrollDeviceResponse.oneofGroups_);
};
goog.inherits(proto.teleport.devicetrust.v1.EnrollDeviceResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.EnrollDeviceResponse.displayName = 'proto.teleport.devicetrust.v1.EnrollDeviceResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.EnrollDeviceInit, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.EnrollDeviceInit.displayName = 'proto.teleport.devicetrust.v1.EnrollDeviceInit';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.EnrollDeviceSuccess, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.EnrollDeviceSuccess.displayName = 'proto.teleport.devicetrust.v1.EnrollDeviceSuccess';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.MacOSEnrollPayload, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.MacOSEnrollPayload.displayName = 'proto.teleport.devicetrust.v1.MacOSEnrollPayload';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.MacOSEnrollChallenge, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.MacOSEnrollChallenge.displayName = 'proto.teleport.devicetrust.v1.MacOSEnrollChallenge';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.displayName = 'proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.devicetrust.v1.TPMEnrollPayload.oneofGroups_);
};
goog.inherits(proto.teleport.devicetrust.v1.TPMEnrollPayload, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.TPMEnrollPayload.displayName = 'proto.teleport.devicetrust.v1.TPMEnrollPayload';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.TPMAttestationParameters, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.TPMAttestationParameters.displayName = 'proto.teleport.devicetrust.v1.TPMAttestationParameters';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.TPMEnrollChallenge, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.TPMEnrollChallenge.displayName = 'proto.teleport.devicetrust.v1.TPMEnrollChallenge';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.TPMEncryptedCredential, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.TPMEncryptedCredential.displayName = 'proto.teleport.devicetrust.v1.TPMEncryptedCredential';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.displayName = 'proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.oneofGroups_);
};
goog.inherits(proto.teleport.devicetrust.v1.AuthenticateDeviceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.displayName = 'proto.teleport.devicetrust.v1.AuthenticateDeviceRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.oneofGroups_);
};
goog.inherits(proto.teleport.devicetrust.v1.AuthenticateDeviceResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.displayName = 'proto.teleport.devicetrust.v1.AuthenticateDeviceResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.AuthenticateDeviceInit, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.AuthenticateDeviceInit.displayName = 'proto.teleport.devicetrust.v1.AuthenticateDeviceInit';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.displayName = 'proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.displayName = 'proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.displayName = 'proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.displayName = 'proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.devicetrust.v1.SyncInventoryRequest.oneofGroups_);
};
goog.inherits(proto.teleport.devicetrust.v1.SyncInventoryRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.SyncInventoryRequest.displayName = 'proto.teleport.devicetrust.v1.SyncInventoryRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.devicetrust.v1.SyncInventoryResponse.oneofGroups_);
};
goog.inherits(proto.teleport.devicetrust.v1.SyncInventoryResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.SyncInventoryResponse.displayName = 'proto.teleport.devicetrust.v1.SyncInventoryResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.SyncInventoryStart = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.SyncInventoryStart, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.SyncInventoryStart.displayName = 'proto.teleport.devicetrust.v1.SyncInventoryStart';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.SyncInventoryEnd = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.SyncInventoryEnd, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.SyncInventoryEnd.displayName = 'proto.teleport.devicetrust.v1.SyncInventoryEnd';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.devicetrust.v1.SyncInventoryDevices.repeatedFields_, null);
};
goog.inherits(proto.teleport.devicetrust.v1.SyncInventoryDevices, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.SyncInventoryDevices.displayName = 'proto.teleport.devicetrust.v1.SyncInventoryDevices';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.SyncInventoryAck = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.SyncInventoryAck, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.SyncInventoryAck.displayName = 'proto.teleport.devicetrust.v1.SyncInventoryAck';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.SyncInventoryResult = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.devicetrust.v1.SyncInventoryResult.repeatedFields_, null);
};
goog.inherits(proto.teleport.devicetrust.v1.SyncInventoryResult, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.SyncInventoryResult.displayName = 'proto.teleport.devicetrust.v1.SyncInventoryResult';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.repeatedFields_, null);
};
goog.inherits(proto.teleport.devicetrust.v1.SyncInventoryMissingDevices, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.displayName = 'proto.teleport.devicetrust.v1.SyncInventoryMissingDevices';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.teleport.devicetrust.v1.GetDevicesUsageRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.devicetrust.v1.GetDevicesUsageRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.devicetrust.v1.GetDevicesUsageRequest.displayName = 'proto.teleport.devicetrust.v1.GetDevicesUsageRequest';
}



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.CreateDeviceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.CreateDeviceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    device: (f = msg.getDevice()) && teleport_devicetrust_v1_device_pb.Device.toObject(includeInstance, f),
    createEnrollToken: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
    createAsResource: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
    enrollTokenExpireTime: (f = msg.getEnrollTokenExpireTime()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceRequest}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.CreateDeviceRequest;
  return proto.teleport.devicetrust.v1.CreateDeviceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.CreateDeviceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceRequest}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_pb.Device;
      reader.readMessage(value,teleport_devicetrust_v1_device_pb.Device.deserializeBinaryFromReader);
      msg.setDevice(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCreateEnrollToken(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCreateAsResource(value);
      break;
    case 4:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setEnrollTokenExpireTime(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.CreateDeviceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.CreateDeviceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevice();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_devicetrust_v1_device_pb.Device.serializeBinaryToWriter
    );
  }
  f = message.getCreateEnrollToken();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getCreateAsResource();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getEnrollTokenExpireTime();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional Device device = 1;
 * @return {?proto.teleport.devicetrust.v1.Device}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.getDevice = function() {
  return /** @type{?proto.teleport.devicetrust.v1.Device} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_device_pb.Device, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.Device|undefined} value
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.setDevice = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.clearDevice = function() {
  return this.setDevice(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.hasDevice = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bool create_enroll_token = 2;
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.getCreateEnrollToken = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.setCreateEnrollToken = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional bool create_as_resource = 3;
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.getCreateAsResource = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.setCreateAsResource = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional google.protobuf.Timestamp enroll_token_expire_time = 4;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.getEnrollTokenExpireTime = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 4));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.setEnrollTokenExpireTime = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.clearEnrollTokenExpireTime = function() {
  return this.setEnrollTokenExpireTime(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.CreateDeviceRequest.prototype.hasEnrollTokenExpireTime = function() {
  return jspb.Message.getField(this, 4) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.UpdateDeviceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.UpdateDeviceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    device: (f = msg.getDevice()) && teleport_devicetrust_v1_device_pb.Device.toObject(includeInstance, f),
    updateMask: (f = msg.getUpdateMask()) && google_protobuf_field_mask_pb.FieldMask.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.UpdateDeviceRequest}
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.UpdateDeviceRequest;
  return proto.teleport.devicetrust.v1.UpdateDeviceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.UpdateDeviceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.UpdateDeviceRequest}
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_pb.Device;
      reader.readMessage(value,teleport_devicetrust_v1_device_pb.Device.deserializeBinaryFromReader);
      msg.setDevice(value);
      break;
    case 2:
      var value = new google_protobuf_field_mask_pb.FieldMask;
      reader.readMessage(value,google_protobuf_field_mask_pb.FieldMask.deserializeBinaryFromReader);
      msg.setUpdateMask(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.UpdateDeviceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.UpdateDeviceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevice();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_devicetrust_v1_device_pb.Device.serializeBinaryToWriter
    );
  }
  f = message.getUpdateMask();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_protobuf_field_mask_pb.FieldMask.serializeBinaryToWriter
    );
  }
};


/**
 * optional Device device = 1;
 * @return {?proto.teleport.devicetrust.v1.Device}
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.getDevice = function() {
  return /** @type{?proto.teleport.devicetrust.v1.Device} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_device_pb.Device, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.Device|undefined} value
 * @return {!proto.teleport.devicetrust.v1.UpdateDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.setDevice = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.UpdateDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.clearDevice = function() {
  return this.setDevice(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.hasDevice = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional google.protobuf.FieldMask update_mask = 2;
 * @return {?proto.google.protobuf.FieldMask}
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.getUpdateMask = function() {
  return /** @type{?proto.google.protobuf.FieldMask} */ (
    jspb.Message.getWrapperField(this, google_protobuf_field_mask_pb.FieldMask, 2));
};


/**
 * @param {?proto.google.protobuf.FieldMask|undefined} value
 * @return {!proto.teleport.devicetrust.v1.UpdateDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.setUpdateMask = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.UpdateDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.clearUpdateMask = function() {
  return this.setUpdateMask(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.UpdateDeviceRequest.prototype.hasUpdateMask = function() {
  return jspb.Message.getField(this, 2) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.UpsertDeviceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.UpsertDeviceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    device: (f = msg.getDevice()) && teleport_devicetrust_v1_device_pb.Device.toObject(includeInstance, f),
    createAsResource: jspb.Message.getBooleanFieldWithDefault(msg, 2, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.UpsertDeviceRequest}
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.UpsertDeviceRequest;
  return proto.teleport.devicetrust.v1.UpsertDeviceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.UpsertDeviceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.UpsertDeviceRequest}
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_pb.Device;
      reader.readMessage(value,teleport_devicetrust_v1_device_pb.Device.deserializeBinaryFromReader);
      msg.setDevice(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCreateAsResource(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.UpsertDeviceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.UpsertDeviceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevice();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_devicetrust_v1_device_pb.Device.serializeBinaryToWriter
    );
  }
  f = message.getCreateAsResource();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
};


/**
 * optional Device device = 1;
 * @return {?proto.teleport.devicetrust.v1.Device}
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.prototype.getDevice = function() {
  return /** @type{?proto.teleport.devicetrust.v1.Device} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_device_pb.Device, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.Device|undefined} value
 * @return {!proto.teleport.devicetrust.v1.UpsertDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.UpsertDeviceRequest.prototype.setDevice = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.UpsertDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.prototype.clearDevice = function() {
  return this.setDevice(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.prototype.hasDevice = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bool create_as_resource = 2;
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.prototype.getCreateAsResource = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.devicetrust.v1.UpsertDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.UpsertDeviceRequest.prototype.setCreateAsResource = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.DeleteDeviceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.DeleteDeviceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.DeleteDeviceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.DeleteDeviceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    deviceId: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.DeleteDeviceRequest}
 */
proto.teleport.devicetrust.v1.DeleteDeviceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.DeleteDeviceRequest;
  return proto.teleport.devicetrust.v1.DeleteDeviceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.DeleteDeviceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.DeleteDeviceRequest}
 */
proto.teleport.devicetrust.v1.DeleteDeviceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDeviceId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.DeleteDeviceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.DeleteDeviceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.DeleteDeviceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.DeleteDeviceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDeviceId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string device_id = 1;
 * @return {string}
 */
proto.teleport.devicetrust.v1.DeleteDeviceRequest.prototype.getDeviceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.DeleteDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.DeleteDeviceRequest.prototype.setDeviceId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.FindDevicesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.FindDevicesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.FindDevicesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.FindDevicesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    idOrTag: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.FindDevicesRequest}
 */
proto.teleport.devicetrust.v1.FindDevicesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.FindDevicesRequest;
  return proto.teleport.devicetrust.v1.FindDevicesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.FindDevicesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.FindDevicesRequest}
 */
proto.teleport.devicetrust.v1.FindDevicesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setIdOrTag(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.FindDevicesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.FindDevicesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.FindDevicesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.FindDevicesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIdOrTag();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string id_or_tag = 1;
 * @return {string}
 */
proto.teleport.devicetrust.v1.FindDevicesRequest.prototype.getIdOrTag = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.FindDevicesRequest} returns this
 */
proto.teleport.devicetrust.v1.FindDevicesRequest.prototype.setIdOrTag = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.FindDevicesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.FindDevicesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    devicesList: jspb.Message.toObjectList(msg.getDevicesList(),
    teleport_devicetrust_v1_device_pb.Device.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.FindDevicesResponse}
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.FindDevicesResponse;
  return proto.teleport.devicetrust.v1.FindDevicesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.FindDevicesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.FindDevicesResponse}
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_pb.Device;
      reader.readMessage(value,teleport_devicetrust_v1_device_pb.Device.deserializeBinaryFromReader);
      msg.addDevices(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.FindDevicesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.FindDevicesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_devicetrust_v1_device_pb.Device.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Device devices = 1;
 * @return {!Array<!proto.teleport.devicetrust.v1.Device>}
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.prototype.getDevicesList = function() {
  return /** @type{!Array<!proto.teleport.devicetrust.v1.Device>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_devicetrust_v1_device_pb.Device, 1));
};


/**
 * @param {!Array<!proto.teleport.devicetrust.v1.Device>} value
 * @return {!proto.teleport.devicetrust.v1.FindDevicesResponse} returns this
*/
proto.teleport.devicetrust.v1.FindDevicesResponse.prototype.setDevicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.devicetrust.v1.Device=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.devicetrust.v1.Device}
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.prototype.addDevices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.devicetrust.v1.Device, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.devicetrust.v1.FindDevicesResponse} returns this
 */
proto.teleport.devicetrust.v1.FindDevicesResponse.prototype.clearDevicesList = function() {
  return this.setDevicesList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.GetDeviceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.GetDeviceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.GetDeviceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.GetDeviceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    deviceId: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.GetDeviceRequest}
 */
proto.teleport.devicetrust.v1.GetDeviceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.GetDeviceRequest;
  return proto.teleport.devicetrust.v1.GetDeviceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.GetDeviceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.GetDeviceRequest}
 */
proto.teleport.devicetrust.v1.GetDeviceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDeviceId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.GetDeviceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.GetDeviceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.GetDeviceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.GetDeviceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDeviceId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string device_id = 1;
 * @return {string}
 */
proto.teleport.devicetrust.v1.GetDeviceRequest.prototype.getDeviceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.GetDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.GetDeviceRequest.prototype.setDeviceId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.ListDevicesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.ListDevicesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    pageSize: jspb.Message.getFieldWithDefault(msg, 1, 0),
    pageToken: jspb.Message.getFieldWithDefault(msg, 2, ""),
    view: jspb.Message.getFieldWithDefault(msg, 3, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.ListDevicesRequest}
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.ListDevicesRequest;
  return proto.teleport.devicetrust.v1.ListDevicesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.ListDevicesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.ListDevicesRequest}
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPageSize(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPageToken(value);
      break;
    case 3:
      var value = /** @type {!proto.teleport.devicetrust.v1.DeviceView} */ (reader.readEnum());
      msg.setView(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.ListDevicesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.ListDevicesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPageSize();
  if (f !== 0) {
    writer.writeInt32(
      1,
      f
    );
  }
  f = message.getPageToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getView();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
};


/**
 * optional int32 page_size = 1;
 * @return {number}
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.prototype.getPageSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.devicetrust.v1.ListDevicesRequest} returns this
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.prototype.setPageSize = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional string page_token = 2;
 * @return {string}
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.prototype.getPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.ListDevicesRequest} returns this
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.prototype.setPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional DeviceView view = 3;
 * @return {!proto.teleport.devicetrust.v1.DeviceView}
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.prototype.getView = function() {
  return /** @type {!proto.teleport.devicetrust.v1.DeviceView} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.teleport.devicetrust.v1.DeviceView} value
 * @return {!proto.teleport.devicetrust.v1.ListDevicesRequest} returns this
 */
proto.teleport.devicetrust.v1.ListDevicesRequest.prototype.setView = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.ListDevicesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.ListDevicesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    devicesList: jspb.Message.toObjectList(msg.getDevicesList(),
    teleport_devicetrust_v1_device_pb.Device.toObject, includeInstance),
    nextPageToken: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.ListDevicesResponse}
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.ListDevicesResponse;
  return proto.teleport.devicetrust.v1.ListDevicesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.ListDevicesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.ListDevicesResponse}
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_pb.Device;
      reader.readMessage(value,teleport_devicetrust_v1_device_pb.Device.deserializeBinaryFromReader);
      msg.addDevices(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNextPageToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.ListDevicesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.ListDevicesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_devicetrust_v1_device_pb.Device.serializeBinaryToWriter
    );
  }
  f = message.getNextPageToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated Device devices = 1;
 * @return {!Array<!proto.teleport.devicetrust.v1.Device>}
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.prototype.getDevicesList = function() {
  return /** @type{!Array<!proto.teleport.devicetrust.v1.Device>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_devicetrust_v1_device_pb.Device, 1));
};


/**
 * @param {!Array<!proto.teleport.devicetrust.v1.Device>} value
 * @return {!proto.teleport.devicetrust.v1.ListDevicesResponse} returns this
*/
proto.teleport.devicetrust.v1.ListDevicesResponse.prototype.setDevicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.devicetrust.v1.Device=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.devicetrust.v1.Device}
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.prototype.addDevices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.devicetrust.v1.Device, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.devicetrust.v1.ListDevicesResponse} returns this
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.prototype.clearDevicesList = function() {
  return this.setDevicesList([]);
};


/**
 * optional string next_page_token = 2;
 * @return {string}
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.prototype.getNextPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.ListDevicesResponse} returns this
 */
proto.teleport.devicetrust.v1.ListDevicesResponse.prototype.setNextPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.BulkCreateDevicesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    devicesList: jspb.Message.toObjectList(msg.getDevicesList(),
    teleport_devicetrust_v1_device_pb.Device.toObject, includeInstance),
    createAsResource: jspb.Message.getBooleanFieldWithDefault(msg, 2, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.BulkCreateDevicesRequest}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.BulkCreateDevicesRequest;
  return proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.BulkCreateDevicesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.BulkCreateDevicesRequest}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_pb.Device;
      reader.readMessage(value,teleport_devicetrust_v1_device_pb.Device.deserializeBinaryFromReader);
      msg.addDevices(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCreateAsResource(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.BulkCreateDevicesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_devicetrust_v1_device_pb.Device.serializeBinaryToWriter
    );
  }
  f = message.getCreateAsResource();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
};


/**
 * repeated Device devices = 1;
 * @return {!Array<!proto.teleport.devicetrust.v1.Device>}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.prototype.getDevicesList = function() {
  return /** @type{!Array<!proto.teleport.devicetrust.v1.Device>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_devicetrust_v1_device_pb.Device, 1));
};


/**
 * @param {!Array<!proto.teleport.devicetrust.v1.Device>} value
 * @return {!proto.teleport.devicetrust.v1.BulkCreateDevicesRequest} returns this
*/
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.prototype.setDevicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.devicetrust.v1.Device=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.devicetrust.v1.Device}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.prototype.addDevices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.devicetrust.v1.Device, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.devicetrust.v1.BulkCreateDevicesRequest} returns this
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.prototype.clearDevicesList = function() {
  return this.setDevicesList([]);
};


/**
 * optional bool create_as_resource = 2;
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.prototype.getCreateAsResource = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.devicetrust.v1.BulkCreateDevicesRequest} returns this
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesRequest.prototype.setCreateAsResource = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.BulkCreateDevicesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    devicesList: jspb.Message.toObjectList(msg.getDevicesList(),
    proto.teleport.devicetrust.v1.DeviceOrStatus.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.BulkCreateDevicesResponse}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.BulkCreateDevicesResponse;
  return proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.BulkCreateDevicesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.BulkCreateDevicesResponse}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.devicetrust.v1.DeviceOrStatus;
      reader.readMessage(value,proto.teleport.devicetrust.v1.DeviceOrStatus.deserializeBinaryFromReader);
      msg.addDevices(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.BulkCreateDevicesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.teleport.devicetrust.v1.DeviceOrStatus.serializeBinaryToWriter
    );
  }
};


/**
 * repeated DeviceOrStatus devices = 1;
 * @return {!Array<!proto.teleport.devicetrust.v1.DeviceOrStatus>}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.prototype.getDevicesList = function() {
  return /** @type{!Array<!proto.teleport.devicetrust.v1.DeviceOrStatus>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.teleport.devicetrust.v1.DeviceOrStatus, 1));
};


/**
 * @param {!Array<!proto.teleport.devicetrust.v1.DeviceOrStatus>} value
 * @return {!proto.teleport.devicetrust.v1.BulkCreateDevicesResponse} returns this
*/
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.prototype.setDevicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.devicetrust.v1.DeviceOrStatus=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.devicetrust.v1.DeviceOrStatus}
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.prototype.addDevices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.devicetrust.v1.DeviceOrStatus, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.devicetrust.v1.BulkCreateDevicesResponse} returns this
 */
proto.teleport.devicetrust.v1.BulkCreateDevicesResponse.prototype.clearDevicesList = function() {
  return this.setDevicesList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.DeviceOrStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.DeviceOrStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
    status: (f = msg.getStatus()) && google_rpc_status_pb.Status.toObject(includeInstance, f),
    id: jspb.Message.getFieldWithDefault(msg, 2, ""),
    deleted: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.DeviceOrStatus}
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.DeviceOrStatus;
  return proto.teleport.devicetrust.v1.DeviceOrStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.DeviceOrStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.DeviceOrStatus}
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_rpc_status_pb.Status;
      reader.readMessage(value,google_rpc_status_pb.Status.deserializeBinaryFromReader);
      msg.setStatus(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setDeleted(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.DeviceOrStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.DeviceOrStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      google_rpc_status_pb.Status.serializeBinaryToWriter
    );
  }
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDeleted();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional google.rpc.Status status = 1;
 * @return {?proto.google.rpc.Status}
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.getStatus = function() {
  return /** @type{?proto.google.rpc.Status} */ (
    jspb.Message.getWrapperField(this, google_rpc_status_pb.Status, 1));
};


/**
 * @param {?proto.google.rpc.Status|undefined} value
 * @return {!proto.teleport.devicetrust.v1.DeviceOrStatus} returns this
*/
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.DeviceOrStatus} returns this
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.hasStatus = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string id = 2;
 * @return {string}
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.DeviceOrStatus} returns this
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool deleted = 3;
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.getDeleted = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.devicetrust.v1.DeviceOrStatus} returns this
 */
proto.teleport.devicetrust.v1.DeviceOrStatus.prototype.setDeleted = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    deviceId: jspb.Message.getFieldWithDefault(msg, 1, ""),
    deviceData: (f = msg.getDeviceData()) && teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.toObject(includeInstance, f),
    expireTime: (f = msg.getExpireTime()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest}
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest;
  return proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest}
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDeviceId(value);
      break;
    case 2:
      var value = new teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData;
      reader.readMessage(value,teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.deserializeBinaryFromReader);
      msg.setDeviceData(value);
      break;
    case 3:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setExpireTime(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDeviceId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDeviceData();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.serializeBinaryToWriter
    );
  }
  f = message.getExpireTime();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
};


/**
 * optional string device_id = 1;
 * @return {string}
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.getDeviceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest} returns this
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.setDeviceId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional DeviceCollectedData device_data = 2;
 * @return {?proto.teleport.devicetrust.v1.DeviceCollectedData}
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.getDeviceData = function() {
  return /** @type{?proto.teleport.devicetrust.v1.DeviceCollectedData} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData, 2));
};


/**
 * @param {?proto.teleport.devicetrust.v1.DeviceCollectedData|undefined} value
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest} returns this
*/
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.setDeviceData = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest} returns this
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.clearDeviceData = function() {
  return this.setDeviceData(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.hasDeviceData = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional google.protobuf.Timestamp expire_time = 3;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.getExpireTime = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 3));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest} returns this
*/
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.setExpireTime = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest} returns this
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.clearExpireTime = function() {
  return this.setExpireTime(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.CreateDeviceEnrollTokenRequest.prototype.hasExpireTime = function() {
  return jspb.Message.getField(this, 3) != null;
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.oneofGroups_ = [[1,2,3]];

/**
 * @enum {number}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.PayloadCase = {
  PAYLOAD_NOT_SET: 0,
  INIT: 1,
  MACOS_CHALLENGE_RESPONSE: 2,
  TPM_CHALLENGE_RESPONSE: 3
};

/**
 * @return {proto.teleport.devicetrust.v1.EnrollDeviceRequest.PayloadCase}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.getPayloadCase = function() {
  return /** @type {proto.teleport.devicetrust.v1.EnrollDeviceRequest.PayloadCase} */(jspb.Message.computeOneofCase(this, proto.teleport.devicetrust.v1.EnrollDeviceRequest.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.EnrollDeviceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    init: (f = msg.getInit()) && proto.teleport.devicetrust.v1.EnrollDeviceInit.toObject(includeInstance, f),
    macosChallengeResponse: (f = msg.getMacosChallengeResponse()) && proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.toObject(includeInstance, f),
    tpmChallengeResponse: (f = msg.getTpmChallengeResponse()) && proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceRequest}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.EnrollDeviceRequest;
  return proto.teleport.devicetrust.v1.EnrollDeviceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceRequest}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.devicetrust.v1.EnrollDeviceInit;
      reader.readMessage(value,proto.teleport.devicetrust.v1.EnrollDeviceInit.deserializeBinaryFromReader);
      msg.setInit(value);
      break;
    case 2:
      var value = new proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse;
      reader.readMessage(value,proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.deserializeBinaryFromReader);
      msg.setMacosChallengeResponse(value);
      break;
    case 3:
      var value = new proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse;
      reader.readMessage(value,proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.deserializeBinaryFromReader);
      msg.setTpmChallengeResponse(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.EnrollDeviceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getInit();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.devicetrust.v1.EnrollDeviceInit.serializeBinaryToWriter
    );
  }
  f = message.getMacosChallengeResponse();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.serializeBinaryToWriter
    );
  }
  f = message.getTpmChallengeResponse();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.serializeBinaryToWriter
    );
  }
};


/**
 * optional EnrollDeviceInit init = 1;
 * @return {?proto.teleport.devicetrust.v1.EnrollDeviceInit}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.getInit = function() {
  return /** @type{?proto.teleport.devicetrust.v1.EnrollDeviceInit} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.EnrollDeviceInit, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.EnrollDeviceInit|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.setInit = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.teleport.devicetrust.v1.EnrollDeviceRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.clearInit = function() {
  return this.setInit(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.hasInit = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional MacOSEnrollChallengeResponse macos_challenge_response = 2;
 * @return {?proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.getMacosChallengeResponse = function() {
  return /** @type{?proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse, 2));
};


/**
 * @param {?proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.setMacosChallengeResponse = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.teleport.devicetrust.v1.EnrollDeviceRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.clearMacosChallengeResponse = function() {
  return this.setMacosChallengeResponse(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.hasMacosChallengeResponse = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional TPMEnrollChallengeResponse tpm_challenge_response = 3;
 * @return {?proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.getTpmChallengeResponse = function() {
  return /** @type{?proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse, 3));
};


/**
 * @param {?proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.setTpmChallengeResponse = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.teleport.devicetrust.v1.EnrollDeviceRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.clearTpmChallengeResponse = function() {
  return this.setTpmChallengeResponse(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceRequest.prototype.hasTpmChallengeResponse = function() {
  return jspb.Message.getField(this, 3) != null;
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.oneofGroups_ = [[1,2,3]];

/**
 * @enum {number}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.PayloadCase = {
  PAYLOAD_NOT_SET: 0,
  SUCCESS: 1,
  MACOS_CHALLENGE: 2,
  TPM_CHALLENGE: 3
};

/**
 * @return {proto.teleport.devicetrust.v1.EnrollDeviceResponse.PayloadCase}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.getPayloadCase = function() {
  return /** @type {proto.teleport.devicetrust.v1.EnrollDeviceResponse.PayloadCase} */(jspb.Message.computeOneofCase(this, proto.teleport.devicetrust.v1.EnrollDeviceResponse.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.EnrollDeviceResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    success: (f = msg.getSuccess()) && proto.teleport.devicetrust.v1.EnrollDeviceSuccess.toObject(includeInstance, f),
    macosChallenge: (f = msg.getMacosChallenge()) && proto.teleport.devicetrust.v1.MacOSEnrollChallenge.toObject(includeInstance, f),
    tpmChallenge: (f = msg.getTpmChallenge()) && proto.teleport.devicetrust.v1.TPMEnrollChallenge.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceResponse}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.EnrollDeviceResponse;
  return proto.teleport.devicetrust.v1.EnrollDeviceResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceResponse}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.devicetrust.v1.EnrollDeviceSuccess;
      reader.readMessage(value,proto.teleport.devicetrust.v1.EnrollDeviceSuccess.deserializeBinaryFromReader);
      msg.setSuccess(value);
      break;
    case 2:
      var value = new proto.teleport.devicetrust.v1.MacOSEnrollChallenge;
      reader.readMessage(value,proto.teleport.devicetrust.v1.MacOSEnrollChallenge.deserializeBinaryFromReader);
      msg.setMacosChallenge(value);
      break;
    case 3:
      var value = new proto.teleport.devicetrust.v1.TPMEnrollChallenge;
      reader.readMessage(value,proto.teleport.devicetrust.v1.TPMEnrollChallenge.deserializeBinaryFromReader);
      msg.setTpmChallenge(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.EnrollDeviceResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSuccess();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.devicetrust.v1.EnrollDeviceSuccess.serializeBinaryToWriter
    );
  }
  f = message.getMacosChallenge();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.teleport.devicetrust.v1.MacOSEnrollChallenge.serializeBinaryToWriter
    );
  }
  f = message.getTpmChallenge();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.teleport.devicetrust.v1.TPMEnrollChallenge.serializeBinaryToWriter
    );
  }
};


/**
 * optional EnrollDeviceSuccess success = 1;
 * @return {?proto.teleport.devicetrust.v1.EnrollDeviceSuccess}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.getSuccess = function() {
  return /** @type{?proto.teleport.devicetrust.v1.EnrollDeviceSuccess} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.EnrollDeviceSuccess, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.EnrollDeviceSuccess|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceResponse} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.setSuccess = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.teleport.devicetrust.v1.EnrollDeviceResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceResponse} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.clearSuccess = function() {
  return this.setSuccess(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.hasSuccess = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional MacOSEnrollChallenge macos_challenge = 2;
 * @return {?proto.teleport.devicetrust.v1.MacOSEnrollChallenge}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.getMacosChallenge = function() {
  return /** @type{?proto.teleport.devicetrust.v1.MacOSEnrollChallenge} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.MacOSEnrollChallenge, 2));
};


/**
 * @param {?proto.teleport.devicetrust.v1.MacOSEnrollChallenge|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceResponse} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.setMacosChallenge = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.teleport.devicetrust.v1.EnrollDeviceResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceResponse} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.clearMacosChallenge = function() {
  return this.setMacosChallenge(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.hasMacosChallenge = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional TPMEnrollChallenge tpm_challenge = 3;
 * @return {?proto.teleport.devicetrust.v1.TPMEnrollChallenge}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.getTpmChallenge = function() {
  return /** @type{?proto.teleport.devicetrust.v1.TPMEnrollChallenge} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.TPMEnrollChallenge, 3));
};


/**
 * @param {?proto.teleport.devicetrust.v1.TPMEnrollChallenge|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceResponse} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.setTpmChallenge = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.teleport.devicetrust.v1.EnrollDeviceResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceResponse} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.clearTpmChallenge = function() {
  return this.setTpmChallenge(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceResponse.prototype.hasTpmChallenge = function() {
  return jspb.Message.getField(this, 3) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.EnrollDeviceInit.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceInit} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.toObject = function(includeInstance, msg) {
  var f, obj = {
    token: jspb.Message.getFieldWithDefault(msg, 1, ""),
    credentialId: jspb.Message.getFieldWithDefault(msg, 2, ""),
    deviceData: (f = msg.getDeviceData()) && teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.toObject(includeInstance, f),
    macos: (f = msg.getMacos()) && proto.teleport.devicetrust.v1.MacOSEnrollPayload.toObject(includeInstance, f),
    tpm: (f = msg.getTpm()) && proto.teleport.devicetrust.v1.TPMEnrollPayload.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.EnrollDeviceInit;
  return proto.teleport.devicetrust.v1.EnrollDeviceInit.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceInit} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setToken(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setCredentialId(value);
      break;
    case 3:
      var value = new teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData;
      reader.readMessage(value,teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.deserializeBinaryFromReader);
      msg.setDeviceData(value);
      break;
    case 4:
      var value = new proto.teleport.devicetrust.v1.MacOSEnrollPayload;
      reader.readMessage(value,proto.teleport.devicetrust.v1.MacOSEnrollPayload.deserializeBinaryFromReader);
      msg.setMacos(value);
      break;
    case 5:
      var value = new proto.teleport.devicetrust.v1.TPMEnrollPayload;
      reader.readMessage(value,proto.teleport.devicetrust.v1.TPMEnrollPayload.deserializeBinaryFromReader);
      msg.setTpm(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.EnrollDeviceInit.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceInit} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getToken();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCredentialId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDeviceData();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.serializeBinaryToWriter
    );
  }
  f = message.getMacos();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.teleport.devicetrust.v1.MacOSEnrollPayload.serializeBinaryToWriter
    );
  }
  f = message.getTpm();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      proto.teleport.devicetrust.v1.TPMEnrollPayload.serializeBinaryToWriter
    );
  }
};


/**
 * optional string token = 1;
 * @return {string}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.getToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.setToken = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string credential_id = 2;
 * @return {string}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.getCredentialId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.setCredentialId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional DeviceCollectedData device_data = 3;
 * @return {?proto.teleport.devicetrust.v1.DeviceCollectedData}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.getDeviceData = function() {
  return /** @type{?proto.teleport.devicetrust.v1.DeviceCollectedData} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData, 3));
};


/**
 * @param {?proto.teleport.devicetrust.v1.DeviceCollectedData|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.setDeviceData = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.clearDeviceData = function() {
  return this.setDeviceData(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.hasDeviceData = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional MacOSEnrollPayload macos = 4;
 * @return {?proto.teleport.devicetrust.v1.MacOSEnrollPayload}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.getMacos = function() {
  return /** @type{?proto.teleport.devicetrust.v1.MacOSEnrollPayload} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.MacOSEnrollPayload, 4));
};


/**
 * @param {?proto.teleport.devicetrust.v1.MacOSEnrollPayload|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.setMacos = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.clearMacos = function() {
  return this.setMacos(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.hasMacos = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional TPMEnrollPayload tpm = 5;
 * @return {?proto.teleport.devicetrust.v1.TPMEnrollPayload}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.getTpm = function() {
  return /** @type{?proto.teleport.devicetrust.v1.TPMEnrollPayload} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.TPMEnrollPayload, 5));
};


/**
 * @param {?proto.teleport.devicetrust.v1.TPMEnrollPayload|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.setTpm = function(value) {
  return jspb.Message.setWrapperField(this, 5, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceInit} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.clearTpm = function() {
  return this.setTpm(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceInit.prototype.hasTpm = function() {
  return jspb.Message.getField(this, 5) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.EnrollDeviceSuccess.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceSuccess} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.toObject = function(includeInstance, msg) {
  var f, obj = {
    device: (f = msg.getDevice()) && teleport_devicetrust_v1_device_pb.Device.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceSuccess}
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.EnrollDeviceSuccess;
  return proto.teleport.devicetrust.v1.EnrollDeviceSuccess.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceSuccess} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceSuccess}
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_pb.Device;
      reader.readMessage(value,teleport_devicetrust_v1_device_pb.Device.deserializeBinaryFromReader);
      msg.setDevice(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.EnrollDeviceSuccess.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.EnrollDeviceSuccess} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevice();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_devicetrust_v1_device_pb.Device.serializeBinaryToWriter
    );
  }
};


/**
 * optional Device device = 1;
 * @return {?proto.teleport.devicetrust.v1.Device}
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.prototype.getDevice = function() {
  return /** @type{?proto.teleport.devicetrust.v1.Device} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_device_pb.Device, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.Device|undefined} value
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceSuccess} returns this
*/
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.prototype.setDevice = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.EnrollDeviceSuccess} returns this
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.prototype.clearDevice = function() {
  return this.setDevice(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.EnrollDeviceSuccess.prototype.hasDevice = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.MacOSEnrollPayload.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.MacOSEnrollPayload} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.toObject = function(includeInstance, msg) {
  var f, obj = {
    publicKeyDer: msg.getPublicKeyDer_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.MacOSEnrollPayload}
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.MacOSEnrollPayload;
  return proto.teleport.devicetrust.v1.MacOSEnrollPayload.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.MacOSEnrollPayload} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.MacOSEnrollPayload}
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPublicKeyDer(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.MacOSEnrollPayload.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.MacOSEnrollPayload} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublicKeyDer_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
};


/**
 * optional bytes public_key_der = 2;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.prototype.getPublicKeyDer = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes public_key_der = 2;
 * This is a type-conversion wrapper around `getPublicKeyDer()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.prototype.getPublicKeyDer_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPublicKeyDer()));
};


/**
 * optional bytes public_key_der = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPublicKeyDer()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.prototype.getPublicKeyDer_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPublicKeyDer()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.MacOSEnrollPayload} returns this
 */
proto.teleport.devicetrust.v1.MacOSEnrollPayload.prototype.setPublicKeyDer = function(value) {
  return jspb.Message.setProto3BytesField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.MacOSEnrollChallenge.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.MacOSEnrollChallenge} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.toObject = function(includeInstance, msg) {
  var f, obj = {
    challenge: msg.getChallenge_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.MacOSEnrollChallenge}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.MacOSEnrollChallenge;
  return proto.teleport.devicetrust.v1.MacOSEnrollChallenge.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.MacOSEnrollChallenge} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.MacOSEnrollChallenge}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setChallenge(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.MacOSEnrollChallenge.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.MacOSEnrollChallenge} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getChallenge_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes challenge = 1;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.prototype.getChallenge = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes challenge = 1;
 * This is a type-conversion wrapper around `getChallenge()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.prototype.getChallenge_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getChallenge()));
};


/**
 * optional bytes challenge = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getChallenge()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.prototype.getChallenge_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getChallenge()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.MacOSEnrollChallenge} returns this
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallenge.prototype.setChallenge = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    signature: msg.getSignature_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse;
  return proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSignature(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSignature_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
};


/**
 * optional bytes signature = 2;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.prototype.getSignature = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes signature = 2;
 * This is a type-conversion wrapper around `getSignature()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.prototype.getSignature_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSignature()));
};


/**
 * optional bytes signature = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSignature()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.prototype.getSignature_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSignature()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse} returns this
 */
proto.teleport.devicetrust.v1.MacOSEnrollChallengeResponse.prototype.setSignature = function(value) {
  return jspb.Message.setProto3BytesField(this, 2, value);
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.oneofGroups_ = [[1,2]];

/**
 * @enum {number}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.EkCase = {
  EK_NOT_SET: 0,
  EK_CERT: 1,
  EK_KEY: 2
};

/**
 * @return {proto.teleport.devicetrust.v1.TPMEnrollPayload.EkCase}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.getEkCase = function() {
  return /** @type {proto.teleport.devicetrust.v1.TPMEnrollPayload.EkCase} */(jspb.Message.computeOneofCase(this, proto.teleport.devicetrust.v1.TPMEnrollPayload.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.TPMEnrollPayload.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.TPMEnrollPayload} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.toObject = function(includeInstance, msg) {
  var f, obj = {
    ekCert: msg.getEkCert_asB64(),
    ekKey: msg.getEkKey_asB64(),
    attestationParameters: (f = msg.getAttestationParameters()) && proto.teleport.devicetrust.v1.TPMAttestationParameters.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollPayload}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.TPMEnrollPayload;
  return proto.teleport.devicetrust.v1.TPMEnrollPayload.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.TPMEnrollPayload} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollPayload}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setEkCert(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setEkKey(value);
      break;
    case 3:
      var value = new proto.teleport.devicetrust.v1.TPMAttestationParameters;
      reader.readMessage(value,proto.teleport.devicetrust.v1.TPMAttestationParameters.deserializeBinaryFromReader);
      msg.setAttestationParameters(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.TPMEnrollPayload.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.TPMEnrollPayload} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 1));
  if (f != null) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = /** @type {!(string|Uint8Array)} */ (jspb.Message.getField(message, 2));
  if (f != null) {
    writer.writeBytes(
      2,
      f
    );
  }
  f = message.getAttestationParameters();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.teleport.devicetrust.v1.TPMAttestationParameters.serializeBinaryToWriter
    );
  }
};


/**
 * optional bytes ek_cert = 1;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.getEkCert = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes ek_cert = 1;
 * This is a type-conversion wrapper around `getEkCert()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.getEkCert_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getEkCert()));
};


/**
 * optional bytes ek_cert = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getEkCert()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.getEkCert_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getEkCert()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollPayload} returns this
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.setEkCert = function(value) {
  return jspb.Message.setOneofField(this, 1, proto.teleport.devicetrust.v1.TPMEnrollPayload.oneofGroups_[0], value);
};


/**
 * Clears the field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollPayload} returns this
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.clearEkCert = function() {
  return jspb.Message.setOneofField(this, 1, proto.teleport.devicetrust.v1.TPMEnrollPayload.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.hasEkCert = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bytes ek_key = 2;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.getEkKey = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes ek_key = 2;
 * This is a type-conversion wrapper around `getEkKey()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.getEkKey_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getEkKey()));
};


/**
 * optional bytes ek_key = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getEkKey()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.getEkKey_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getEkKey()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollPayload} returns this
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.setEkKey = function(value) {
  return jspb.Message.setOneofField(this, 2, proto.teleport.devicetrust.v1.TPMEnrollPayload.oneofGroups_[0], value);
};


/**
 * Clears the field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollPayload} returns this
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.clearEkKey = function() {
  return jspb.Message.setOneofField(this, 2, proto.teleport.devicetrust.v1.TPMEnrollPayload.oneofGroups_[0], undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.hasEkKey = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional TPMAttestationParameters attestation_parameters = 3;
 * @return {?proto.teleport.devicetrust.v1.TPMAttestationParameters}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.getAttestationParameters = function() {
  return /** @type{?proto.teleport.devicetrust.v1.TPMAttestationParameters} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.TPMAttestationParameters, 3));
};


/**
 * @param {?proto.teleport.devicetrust.v1.TPMAttestationParameters|undefined} value
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollPayload} returns this
*/
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.setAttestationParameters = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollPayload} returns this
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.clearAttestationParameters = function() {
  return this.setAttestationParameters(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.TPMEnrollPayload.prototype.hasAttestationParameters = function() {
  return jspb.Message.getField(this, 3) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.TPMAttestationParameters.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.TPMAttestationParameters} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.toObject = function(includeInstance, msg) {
  var f, obj = {
    pb_public: msg.getPublic_asB64(),
    createData: msg.getCreateData_asB64(),
    createAttestation: msg.getCreateAttestation_asB64(),
    createSignature: msg.getCreateSignature_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.TPMAttestationParameters}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.TPMAttestationParameters;
  return proto.teleport.devicetrust.v1.TPMAttestationParameters.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.TPMAttestationParameters} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.TPMAttestationParameters}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setPublic(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setCreateData(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setCreateAttestation(value);
      break;
    case 4:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setCreateSignature(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.TPMAttestationParameters.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.TPMAttestationParameters} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPublic_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getCreateData_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
  f = message.getCreateAttestation_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
  f = message.getCreateSignature_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      4,
      f
    );
  }
};


/**
 * optional bytes public = 1;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getPublic = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes public = 1;
 * This is a type-conversion wrapper around `getPublic()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getPublic_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getPublic()));
};


/**
 * optional bytes public = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getPublic()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getPublic_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getPublic()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMAttestationParameters} returns this
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.setPublic = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional bytes create_data = 2;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getCreateData = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes create_data = 2;
 * This is a type-conversion wrapper around `getCreateData()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getCreateData_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getCreateData()));
};


/**
 * optional bytes create_data = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getCreateData()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getCreateData_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getCreateData()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMAttestationParameters} returns this
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.setCreateData = function(value) {
  return jspb.Message.setProto3BytesField(this, 2, value);
};


/**
 * optional bytes create_attestation = 3;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getCreateAttestation = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes create_attestation = 3;
 * This is a type-conversion wrapper around `getCreateAttestation()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getCreateAttestation_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getCreateAttestation()));
};


/**
 * optional bytes create_attestation = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getCreateAttestation()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getCreateAttestation_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getCreateAttestation()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMAttestationParameters} returns this
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.setCreateAttestation = function(value) {
  return jspb.Message.setProto3BytesField(this, 3, value);
};


/**
 * optional bytes create_signature = 4;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getCreateSignature = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * optional bytes create_signature = 4;
 * This is a type-conversion wrapper around `getCreateSignature()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getCreateSignature_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getCreateSignature()));
};


/**
 * optional bytes create_signature = 4;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getCreateSignature()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.getCreateSignature_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getCreateSignature()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMAttestationParameters} returns this
 */
proto.teleport.devicetrust.v1.TPMAttestationParameters.prototype.setCreateSignature = function(value) {
  return jspb.Message.setProto3BytesField(this, 4, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.TPMEnrollChallenge.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.TPMEnrollChallenge} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.toObject = function(includeInstance, msg) {
  var f, obj = {
    encryptedCredential: (f = msg.getEncryptedCredential()) && proto.teleport.devicetrust.v1.TPMEncryptedCredential.toObject(includeInstance, f),
    attestationNonce: msg.getAttestationNonce_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallenge}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.TPMEnrollChallenge;
  return proto.teleport.devicetrust.v1.TPMEnrollChallenge.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.TPMEnrollChallenge} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallenge}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.devicetrust.v1.TPMEncryptedCredential;
      reader.readMessage(value,proto.teleport.devicetrust.v1.TPMEncryptedCredential.deserializeBinaryFromReader);
      msg.setEncryptedCredential(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setAttestationNonce(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.TPMEnrollChallenge.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.TPMEnrollChallenge} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getEncryptedCredential();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.devicetrust.v1.TPMEncryptedCredential.serializeBinaryToWriter
    );
  }
  f = message.getAttestationNonce_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
};


/**
 * optional TPMEncryptedCredential encrypted_credential = 1;
 * @return {?proto.teleport.devicetrust.v1.TPMEncryptedCredential}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.getEncryptedCredential = function() {
  return /** @type{?proto.teleport.devicetrust.v1.TPMEncryptedCredential} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.TPMEncryptedCredential, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.TPMEncryptedCredential|undefined} value
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallenge} returns this
*/
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.setEncryptedCredential = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallenge} returns this
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.clearEncryptedCredential = function() {
  return this.setEncryptedCredential(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.hasEncryptedCredential = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bytes attestation_nonce = 2;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.getAttestationNonce = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes attestation_nonce = 2;
 * This is a type-conversion wrapper around `getAttestationNonce()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.getAttestationNonce_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getAttestationNonce()));
};


/**
 * optional bytes attestation_nonce = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getAttestationNonce()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.getAttestationNonce_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getAttestationNonce()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallenge} returns this
 */
proto.teleport.devicetrust.v1.TPMEnrollChallenge.prototype.setAttestationNonce = function(value) {
  return jspb.Message.setProto3BytesField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.TPMEncryptedCredential.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.TPMEncryptedCredential} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.toObject = function(includeInstance, msg) {
  var f, obj = {
    credentialBlob: msg.getCredentialBlob_asB64(),
    secret: msg.getSecret_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.TPMEncryptedCredential}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.TPMEncryptedCredential;
  return proto.teleport.devicetrust.v1.TPMEncryptedCredential.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.TPMEncryptedCredential} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.TPMEncryptedCredential}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setCredentialBlob(value);
      break;
    case 2:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSecret(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.TPMEncryptedCredential.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.TPMEncryptedCredential} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCredentialBlob_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getSecret_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      2,
      f
    );
  }
};


/**
 * optional bytes credential_blob = 1;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.getCredentialBlob = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes credential_blob = 1;
 * This is a type-conversion wrapper around `getCredentialBlob()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.getCredentialBlob_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getCredentialBlob()));
};


/**
 * optional bytes credential_blob = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getCredentialBlob()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.getCredentialBlob_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getCredentialBlob()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMEncryptedCredential} returns this
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.setCredentialBlob = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional bytes secret = 2;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.getSecret = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * optional bytes secret = 2;
 * This is a type-conversion wrapper around `getSecret()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.getSecret_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSecret()));
};


/**
 * optional bytes secret = 2;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSecret()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.getSecret_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSecret()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMEncryptedCredential} returns this
 */
proto.teleport.devicetrust.v1.TPMEncryptedCredential.prototype.setSecret = function(value) {
  return jspb.Message.setProto3BytesField(this, 2, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    solution: msg.getSolution_asB64(),
    platformParameters: (f = msg.getPlatformParameters()) && teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse;
  return proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSolution(value);
      break;
    case 2:
      var value = new teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters;
      reader.readMessage(value,teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters.deserializeBinaryFromReader);
      msg.setPlatformParameters(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSolution_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getPlatformParameters();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters.serializeBinaryToWriter
    );
  }
};


/**
 * optional bytes solution = 1;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.getSolution = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes solution = 1;
 * This is a type-conversion wrapper around `getSolution()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.getSolution_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSolution()));
};


/**
 * optional bytes solution = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSolution()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.getSolution_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSolution()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse} returns this
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.setSolution = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional TPMPlatformParameters platform_parameters = 2;
 * @return {?proto.teleport.devicetrust.v1.TPMPlatformParameters}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.getPlatformParameters = function() {
  return /** @type{?proto.teleport.devicetrust.v1.TPMPlatformParameters} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters, 2));
};


/**
 * @param {?proto.teleport.devicetrust.v1.TPMPlatformParameters|undefined} value
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse} returns this
*/
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.setPlatformParameters = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse} returns this
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.clearPlatformParameters = function() {
  return this.setPlatformParameters(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.TPMEnrollChallengeResponse.prototype.hasPlatformParameters = function() {
  return jspb.Message.getField(this, 2) != null;
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.oneofGroups_ = [[1,2,3]];

/**
 * @enum {number}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.PayloadCase = {
  PAYLOAD_NOT_SET: 0,
  INIT: 1,
  CHALLENGE_RESPONSE: 2,
  TPM_CHALLENGE_RESPONSE: 3
};

/**
 * @return {proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.PayloadCase}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.getPayloadCase = function() {
  return /** @type {proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.PayloadCase} */(jspb.Message.computeOneofCase(this, proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    init: (f = msg.getInit()) && proto.teleport.devicetrust.v1.AuthenticateDeviceInit.toObject(includeInstance, f),
    challengeResponse: (f = msg.getChallengeResponse()) && proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.toObject(includeInstance, f),
    tpmChallengeResponse: (f = msg.getTpmChallengeResponse()) && proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.AuthenticateDeviceRequest;
  return proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.devicetrust.v1.AuthenticateDeviceInit;
      reader.readMessage(value,proto.teleport.devicetrust.v1.AuthenticateDeviceInit.deserializeBinaryFromReader);
      msg.setInit(value);
      break;
    case 2:
      var value = new proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse;
      reader.readMessage(value,proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.deserializeBinaryFromReader);
      msg.setChallengeResponse(value);
      break;
    case 3:
      var value = new proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse;
      reader.readMessage(value,proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.deserializeBinaryFromReader);
      msg.setTpmChallengeResponse(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getInit();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.devicetrust.v1.AuthenticateDeviceInit.serializeBinaryToWriter
    );
  }
  f = message.getChallengeResponse();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.serializeBinaryToWriter
    );
  }
  f = message.getTpmChallengeResponse();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.serializeBinaryToWriter
    );
  }
};


/**
 * optional AuthenticateDeviceInit init = 1;
 * @return {?proto.teleport.devicetrust.v1.AuthenticateDeviceInit}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.getInit = function() {
  return /** @type{?proto.teleport.devicetrust.v1.AuthenticateDeviceInit} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.AuthenticateDeviceInit, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.AuthenticateDeviceInit|undefined} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.setInit = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.clearInit = function() {
  return this.setInit(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.hasInit = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional AuthenticateDeviceChallengeResponse challenge_response = 2;
 * @return {?proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.getChallengeResponse = function() {
  return /** @type{?proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse, 2));
};


/**
 * @param {?proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse|undefined} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.setChallengeResponse = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.clearChallengeResponse = function() {
  return this.setChallengeResponse(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.hasChallengeResponse = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional TPMAuthenticateDeviceChallengeResponse tpm_challenge_response = 3;
 * @return {?proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.getTpmChallengeResponse = function() {
  return /** @type{?proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse, 3));
};


/**
 * @param {?proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse|undefined} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest} returns this
*/
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.setTpmChallengeResponse = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceRequest} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.clearTpmChallengeResponse = function() {
  return this.setTpmChallengeResponse(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceRequest.prototype.hasTpmChallengeResponse = function() {
  return jspb.Message.getField(this, 3) != null;
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.oneofGroups_ = [[1,2,3]];

/**
 * @enum {number}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.PayloadCase = {
  PAYLOAD_NOT_SET: 0,
  CHALLENGE: 1,
  USER_CERTIFICATES: 2,
  TPM_CHALLENGE: 3
};

/**
 * @return {proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.PayloadCase}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.getPayloadCase = function() {
  return /** @type {proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.PayloadCase} */(jspb.Message.computeOneofCase(this, proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    challenge: (f = msg.getChallenge()) && proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.toObject(includeInstance, f),
    userCertificates: (f = msg.getUserCertificates()) && teleport_devicetrust_v1_user_certificates_pb.UserCertificates.toObject(includeInstance, f),
    tpmChallenge: (f = msg.getTpmChallenge()) && proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.AuthenticateDeviceResponse;
  return proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge;
      reader.readMessage(value,proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.deserializeBinaryFromReader);
      msg.setChallenge(value);
      break;
    case 2:
      var value = new teleport_devicetrust_v1_user_certificates_pb.UserCertificates;
      reader.readMessage(value,teleport_devicetrust_v1_user_certificates_pb.UserCertificates.deserializeBinaryFromReader);
      msg.setUserCertificates(value);
      break;
    case 3:
      var value = new proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge;
      reader.readMessage(value,proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.deserializeBinaryFromReader);
      msg.setTpmChallenge(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getChallenge();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.serializeBinaryToWriter
    );
  }
  f = message.getUserCertificates();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      teleport_devicetrust_v1_user_certificates_pb.UserCertificates.serializeBinaryToWriter
    );
  }
  f = message.getTpmChallenge();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.serializeBinaryToWriter
    );
  }
};


/**
 * optional AuthenticateDeviceChallenge challenge = 1;
 * @return {?proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.getChallenge = function() {
  return /** @type{?proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge|undefined} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse} returns this
*/
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.setChallenge = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.clearChallenge = function() {
  return this.setChallenge(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.hasChallenge = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional UserCertificates user_certificates = 2;
 * @return {?proto.teleport.devicetrust.v1.UserCertificates}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.getUserCertificates = function() {
  return /** @type{?proto.teleport.devicetrust.v1.UserCertificates} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_user_certificates_pb.UserCertificates, 2));
};


/**
 * @param {?proto.teleport.devicetrust.v1.UserCertificates|undefined} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse} returns this
*/
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.setUserCertificates = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.clearUserCertificates = function() {
  return this.setUserCertificates(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.hasUserCertificates = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional TPMAuthenticateDeviceChallenge tpm_challenge = 3;
 * @return {?proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.getTpmChallenge = function() {
  return /** @type{?proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge, 3));
};


/**
 * @param {?proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge|undefined} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse} returns this
*/
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.setTpmChallenge = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceResponse} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.clearTpmChallenge = function() {
  return this.setTpmChallenge(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceResponse.prototype.hasTpmChallenge = function() {
  return jspb.Message.getField(this, 3) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.AuthenticateDeviceInit.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.toObject = function(includeInstance, msg) {
  var f, obj = {
    userCertificates: (f = msg.getUserCertificates()) && teleport_devicetrust_v1_user_certificates_pb.UserCertificates.toObject(includeInstance, f),
    credentialId: jspb.Message.getFieldWithDefault(msg, 2, ""),
    deviceData: (f = msg.getDeviceData()) && teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.toObject(includeInstance, f),
    deviceWebToken: (f = msg.getDeviceWebToken()) && teleport_devicetrust_v1_device_web_token_pb.DeviceWebToken.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.AuthenticateDeviceInit;
  return proto.teleport.devicetrust.v1.AuthenticateDeviceInit.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_user_certificates_pb.UserCertificates;
      reader.readMessage(value,teleport_devicetrust_v1_user_certificates_pb.UserCertificates.deserializeBinaryFromReader);
      msg.setUserCertificates(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setCredentialId(value);
      break;
    case 3:
      var value = new teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData;
      reader.readMessage(value,teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.deserializeBinaryFromReader);
      msg.setDeviceData(value);
      break;
    case 4:
      var value = new teleport_devicetrust_v1_device_web_token_pb.DeviceWebToken;
      reader.readMessage(value,teleport_devicetrust_v1_device_web_token_pb.DeviceWebToken.deserializeBinaryFromReader);
      msg.setDeviceWebToken(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.AuthenticateDeviceInit.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserCertificates();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_devicetrust_v1_user_certificates_pb.UserCertificates.serializeBinaryToWriter
    );
  }
  f = message.getCredentialId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDeviceData();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.serializeBinaryToWriter
    );
  }
  f = message.getDeviceWebToken();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      teleport_devicetrust_v1_device_web_token_pb.DeviceWebToken.serializeBinaryToWriter
    );
  }
};


/**
 * optional UserCertificates user_certificates = 1;
 * @return {?proto.teleport.devicetrust.v1.UserCertificates}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.getUserCertificates = function() {
  return /** @type{?proto.teleport.devicetrust.v1.UserCertificates} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_user_certificates_pb.UserCertificates, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.UserCertificates|undefined} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} returns this
*/
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.setUserCertificates = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.clearUserCertificates = function() {
  return this.setUserCertificates(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.hasUserCertificates = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string credential_id = 2;
 * @return {string}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.getCredentialId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.setCredentialId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional DeviceCollectedData device_data = 3;
 * @return {?proto.teleport.devicetrust.v1.DeviceCollectedData}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.getDeviceData = function() {
  return /** @type{?proto.teleport.devicetrust.v1.DeviceCollectedData} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData, 3));
};


/**
 * @param {?proto.teleport.devicetrust.v1.DeviceCollectedData|undefined} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} returns this
*/
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.setDeviceData = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.clearDeviceData = function() {
  return this.setDeviceData(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.hasDeviceData = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional DeviceWebToken device_web_token = 4;
 * @return {?proto.teleport.devicetrust.v1.DeviceWebToken}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.getDeviceWebToken = function() {
  return /** @type{?proto.teleport.devicetrust.v1.DeviceWebToken} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_device_web_token_pb.DeviceWebToken, 4));
};


/**
 * @param {?proto.teleport.devicetrust.v1.DeviceWebToken|undefined} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} returns this
*/
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.setDeviceWebToken = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceInit} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.clearDeviceWebToken = function() {
  return this.setDeviceWebToken(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceInit.prototype.hasDeviceWebToken = function() {
  return jspb.Message.getField(this, 4) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.toObject = function(includeInstance, msg) {
  var f, obj = {
    attestationNonce: msg.getAttestationNonce_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge;
  return proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setAttestationNonce(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAttestationNonce_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes attestation_nonce = 1;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.prototype.getAttestationNonce = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes attestation_nonce = 1;
 * This is a type-conversion wrapper around `getAttestationNonce()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.prototype.getAttestationNonce_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getAttestationNonce()));
};


/**
 * optional bytes attestation_nonce = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getAttestationNonce()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.prototype.getAttestationNonce_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getAttestationNonce()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge} returns this
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallenge.prototype.setAttestationNonce = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    platformParameters: (f = msg.getPlatformParameters()) && teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse;
  return proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters;
      reader.readMessage(value,teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters.deserializeBinaryFromReader);
      msg.setPlatformParameters(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPlatformParameters();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters.serializeBinaryToWriter
    );
  }
};


/**
 * optional TPMPlatformParameters platform_parameters = 1;
 * @return {?proto.teleport.devicetrust.v1.TPMPlatformParameters}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.prototype.getPlatformParameters = function() {
  return /** @type{?proto.teleport.devicetrust.v1.TPMPlatformParameters} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.TPMPlatformParameters|undefined} value
 * @return {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse} returns this
*/
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.prototype.setPlatformParameters = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse} returns this
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.prototype.clearPlatformParameters = function() {
  return this.setPlatformParameters(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.TPMAuthenticateDeviceChallengeResponse.prototype.hasPlatformParameters = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.toObject = function(includeInstance, msg) {
  var f, obj = {
    challenge: msg.getChallenge_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge;
  return proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setChallenge(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getChallenge_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes challenge = 1;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.prototype.getChallenge = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes challenge = 1;
 * This is a type-conversion wrapper around `getChallenge()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.prototype.getChallenge_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getChallenge()));
};


/**
 * optional bytes challenge = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getChallenge()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.prototype.getChallenge_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getChallenge()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallenge.prototype.setChallenge = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    signature: msg.getSignature_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse;
  return proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setSignature(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSignature_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
};


/**
 * optional bytes signature = 1;
 * @return {!(string|Uint8Array)}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.prototype.getSignature = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes signature = 1;
 * This is a type-conversion wrapper around `getSignature()`
 * @return {string}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.prototype.getSignature_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getSignature()));
};


/**
 * optional bytes signature = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getSignature()`
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.prototype.getSignature_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getSignature()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse} returns this
 */
proto.teleport.devicetrust.v1.AuthenticateDeviceChallengeResponse.prototype.setSignature = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.oneofGroups_ = [[1,2,3,4]];

/**
 * @enum {number}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.PayloadCase = {
  PAYLOAD_NOT_SET: 0,
  START: 1,
  END: 2,
  DEVICES_TO_UPSERT: 3,
  DEVICES_TO_REMOVE: 4
};

/**
 * @return {proto.teleport.devicetrust.v1.SyncInventoryRequest.PayloadCase}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.getPayloadCase = function() {
  return /** @type {proto.teleport.devicetrust.v1.SyncInventoryRequest.PayloadCase} */(jspb.Message.computeOneofCase(this, proto.teleport.devicetrust.v1.SyncInventoryRequest.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.SyncInventoryRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    start: (f = msg.getStart()) && proto.teleport.devicetrust.v1.SyncInventoryStart.toObject(includeInstance, f),
    end: (f = msg.getEnd()) && proto.teleport.devicetrust.v1.SyncInventoryEnd.toObject(includeInstance, f),
    devicesToUpsert: (f = msg.getDevicesToUpsert()) && proto.teleport.devicetrust.v1.SyncInventoryDevices.toObject(includeInstance, f),
    devicesToRemove: (f = msg.getDevicesToRemove()) && proto.teleport.devicetrust.v1.SyncInventoryDevices.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.SyncInventoryRequest;
  return proto.teleport.devicetrust.v1.SyncInventoryRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.devicetrust.v1.SyncInventoryStart;
      reader.readMessage(value,proto.teleport.devicetrust.v1.SyncInventoryStart.deserializeBinaryFromReader);
      msg.setStart(value);
      break;
    case 2:
      var value = new proto.teleport.devicetrust.v1.SyncInventoryEnd;
      reader.readMessage(value,proto.teleport.devicetrust.v1.SyncInventoryEnd.deserializeBinaryFromReader);
      msg.setEnd(value);
      break;
    case 3:
      var value = new proto.teleport.devicetrust.v1.SyncInventoryDevices;
      reader.readMessage(value,proto.teleport.devicetrust.v1.SyncInventoryDevices.deserializeBinaryFromReader);
      msg.setDevicesToUpsert(value);
      break;
    case 4:
      var value = new proto.teleport.devicetrust.v1.SyncInventoryDevices;
      reader.readMessage(value,proto.teleport.devicetrust.v1.SyncInventoryDevices.deserializeBinaryFromReader);
      msg.setDevicesToRemove(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.SyncInventoryRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStart();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.devicetrust.v1.SyncInventoryStart.serializeBinaryToWriter
    );
  }
  f = message.getEnd();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.teleport.devicetrust.v1.SyncInventoryEnd.serializeBinaryToWriter
    );
  }
  f = message.getDevicesToUpsert();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.teleport.devicetrust.v1.SyncInventoryDevices.serializeBinaryToWriter
    );
  }
  f = message.getDevicesToRemove();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.teleport.devicetrust.v1.SyncInventoryDevices.serializeBinaryToWriter
    );
  }
};


/**
 * optional SyncInventoryStart start = 1;
 * @return {?proto.teleport.devicetrust.v1.SyncInventoryStart}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.getStart = function() {
  return /** @type{?proto.teleport.devicetrust.v1.SyncInventoryStart} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.SyncInventoryStart, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.SyncInventoryStart|undefined} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.setStart = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.teleport.devicetrust.v1.SyncInventoryRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.clearStart = function() {
  return this.setStart(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.hasStart = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional SyncInventoryEnd end = 2;
 * @return {?proto.teleport.devicetrust.v1.SyncInventoryEnd}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.getEnd = function() {
  return /** @type{?proto.teleport.devicetrust.v1.SyncInventoryEnd} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.SyncInventoryEnd, 2));
};


/**
 * @param {?proto.teleport.devicetrust.v1.SyncInventoryEnd|undefined} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.setEnd = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.teleport.devicetrust.v1.SyncInventoryRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.clearEnd = function() {
  return this.setEnd(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.hasEnd = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional SyncInventoryDevices devices_to_upsert = 3;
 * @return {?proto.teleport.devicetrust.v1.SyncInventoryDevices}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.getDevicesToUpsert = function() {
  return /** @type{?proto.teleport.devicetrust.v1.SyncInventoryDevices} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.SyncInventoryDevices, 3));
};


/**
 * @param {?proto.teleport.devicetrust.v1.SyncInventoryDevices|undefined} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.setDevicesToUpsert = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.teleport.devicetrust.v1.SyncInventoryRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.clearDevicesToUpsert = function() {
  return this.setDevicesToUpsert(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.hasDevicesToUpsert = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional SyncInventoryDevices devices_to_remove = 4;
 * @return {?proto.teleport.devicetrust.v1.SyncInventoryDevices}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.getDevicesToRemove = function() {
  return /** @type{?proto.teleport.devicetrust.v1.SyncInventoryDevices} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.SyncInventoryDevices, 4));
};


/**
 * @param {?proto.teleport.devicetrust.v1.SyncInventoryDevices|undefined} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.setDevicesToRemove = function(value) {
  return jspb.Message.setOneofWrapperField(this, 4, proto.teleport.devicetrust.v1.SyncInventoryRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryRequest} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.clearDevicesToRemove = function() {
  return this.setDevicesToRemove(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.SyncInventoryRequest.prototype.hasDevicesToRemove = function() {
  return jspb.Message.getField(this, 4) != null;
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.oneofGroups_ = [[1,2,3]];

/**
 * @enum {number}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.PayloadCase = {
  PAYLOAD_NOT_SET: 0,
  ACK: 1,
  RESULT: 2,
  MISSING_DEVICES: 3
};

/**
 * @return {proto.teleport.devicetrust.v1.SyncInventoryResponse.PayloadCase}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.getPayloadCase = function() {
  return /** @type {proto.teleport.devicetrust.v1.SyncInventoryResponse.PayloadCase} */(jspb.Message.computeOneofCase(this, proto.teleport.devicetrust.v1.SyncInventoryResponse.oneofGroups_[0]));
};



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.SyncInventoryResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    ack: (f = msg.getAck()) && proto.teleport.devicetrust.v1.SyncInventoryAck.toObject(includeInstance, f),
    result: (f = msg.getResult()) && proto.teleport.devicetrust.v1.SyncInventoryResult.toObject(includeInstance, f),
    missingDevices: (f = msg.getMissingDevices()) && proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResponse}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.SyncInventoryResponse;
  return proto.teleport.devicetrust.v1.SyncInventoryResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResponse}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.devicetrust.v1.SyncInventoryAck;
      reader.readMessage(value,proto.teleport.devicetrust.v1.SyncInventoryAck.deserializeBinaryFromReader);
      msg.setAck(value);
      break;
    case 2:
      var value = new proto.teleport.devicetrust.v1.SyncInventoryResult;
      reader.readMessage(value,proto.teleport.devicetrust.v1.SyncInventoryResult.deserializeBinaryFromReader);
      msg.setResult(value);
      break;
    case 3:
      var value = new proto.teleport.devicetrust.v1.SyncInventoryMissingDevices;
      reader.readMessage(value,proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.deserializeBinaryFromReader);
      msg.setMissingDevices(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.SyncInventoryResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAck();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.devicetrust.v1.SyncInventoryAck.serializeBinaryToWriter
    );
  }
  f = message.getResult();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.teleport.devicetrust.v1.SyncInventoryResult.serializeBinaryToWriter
    );
  }
  f = message.getMissingDevices();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.serializeBinaryToWriter
    );
  }
};


/**
 * optional SyncInventoryAck ack = 1;
 * @return {?proto.teleport.devicetrust.v1.SyncInventoryAck}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.getAck = function() {
  return /** @type{?proto.teleport.devicetrust.v1.SyncInventoryAck} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.SyncInventoryAck, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.SyncInventoryAck|undefined} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResponse} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.setAck = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.teleport.devicetrust.v1.SyncInventoryResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResponse} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.clearAck = function() {
  return this.setAck(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.hasAck = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional SyncInventoryResult result = 2;
 * @return {?proto.teleport.devicetrust.v1.SyncInventoryResult}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.getResult = function() {
  return /** @type{?proto.teleport.devicetrust.v1.SyncInventoryResult} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.SyncInventoryResult, 2));
};


/**
 * @param {?proto.teleport.devicetrust.v1.SyncInventoryResult|undefined} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResponse} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.setResult = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.teleport.devicetrust.v1.SyncInventoryResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResponse} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.clearResult = function() {
  return this.setResult(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.hasResult = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional SyncInventoryMissingDevices missing_devices = 3;
 * @return {?proto.teleport.devicetrust.v1.SyncInventoryMissingDevices}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.getMissingDevices = function() {
  return /** @type{?proto.teleport.devicetrust.v1.SyncInventoryMissingDevices} */ (
    jspb.Message.getWrapperField(this, proto.teleport.devicetrust.v1.SyncInventoryMissingDevices, 3));
};


/**
 * @param {?proto.teleport.devicetrust.v1.SyncInventoryMissingDevices|undefined} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResponse} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.setMissingDevices = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.teleport.devicetrust.v1.SyncInventoryResponse.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResponse} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.clearMissingDevices = function() {
  return this.setMissingDevices(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.SyncInventoryResponse.prototype.hasMissingDevices = function() {
  return jspb.Message.getField(this, 3) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.SyncInventoryStart.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryStart} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.toObject = function(includeInstance, msg) {
  var f, obj = {
    source: (f = msg.getSource()) && teleport_devicetrust_v1_device_source_pb.DeviceSource.toObject(includeInstance, f),
    trackMissingDevices: jspb.Message.getBooleanFieldWithDefault(msg, 4, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryStart}
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.SyncInventoryStart;
  return proto.teleport.devicetrust.v1.SyncInventoryStart.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryStart} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryStart}
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_source_pb.DeviceSource;
      reader.readMessage(value,teleport_devicetrust_v1_device_source_pb.DeviceSource.deserializeBinaryFromReader);
      msg.setSource(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setTrackMissingDevices(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.SyncInventoryStart.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryStart} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSource();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_devicetrust_v1_device_source_pb.DeviceSource.serializeBinaryToWriter
    );
  }
  f = message.getTrackMissingDevices();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
};


/**
 * optional DeviceSource source = 1;
 * @return {?proto.teleport.devicetrust.v1.DeviceSource}
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.prototype.getSource = function() {
  return /** @type{?proto.teleport.devicetrust.v1.DeviceSource} */ (
    jspb.Message.getWrapperField(this, teleport_devicetrust_v1_device_source_pb.DeviceSource, 1));
};


/**
 * @param {?proto.teleport.devicetrust.v1.DeviceSource|undefined} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryStart} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryStart.prototype.setSource = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryStart} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.prototype.clearSource = function() {
  return this.setSource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.prototype.hasSource = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional bool track_missing_devices = 4;
 * @return {boolean}
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.prototype.getTrackMissingDevices = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryStart} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryStart.prototype.setTrackMissingDevices = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.SyncInventoryEnd.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.SyncInventoryEnd.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryEnd} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryEnd.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryEnd}
 */
proto.teleport.devicetrust.v1.SyncInventoryEnd.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.SyncInventoryEnd;
  return proto.teleport.devicetrust.v1.SyncInventoryEnd.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryEnd} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryEnd}
 */
proto.teleport.devicetrust.v1.SyncInventoryEnd.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.SyncInventoryEnd.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.SyncInventoryEnd.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryEnd} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryEnd.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.SyncInventoryDevices.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryDevices} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.toObject = function(includeInstance, msg) {
  var f, obj = {
    devicesList: jspb.Message.toObjectList(msg.getDevicesList(),
    teleport_devicetrust_v1_device_pb.Device.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryDevices}
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.SyncInventoryDevices;
  return proto.teleport.devicetrust.v1.SyncInventoryDevices.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryDevices} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryDevices}
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_pb.Device;
      reader.readMessage(value,teleport_devicetrust_v1_device_pb.Device.deserializeBinaryFromReader);
      msg.addDevices(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.SyncInventoryDevices.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryDevices} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_devicetrust_v1_device_pb.Device.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Device devices = 1;
 * @return {!Array<!proto.teleport.devicetrust.v1.Device>}
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.prototype.getDevicesList = function() {
  return /** @type{!Array<!proto.teleport.devicetrust.v1.Device>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_devicetrust_v1_device_pb.Device, 1));
};


/**
 * @param {!Array<!proto.teleport.devicetrust.v1.Device>} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryDevices} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryDevices.prototype.setDevicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.devicetrust.v1.Device=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.devicetrust.v1.Device}
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.prototype.addDevices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.devicetrust.v1.Device, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryDevices} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryDevices.prototype.clearDevicesList = function() {
  return this.setDevicesList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.SyncInventoryAck.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.SyncInventoryAck.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryAck} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryAck.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryAck}
 */
proto.teleport.devicetrust.v1.SyncInventoryAck.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.SyncInventoryAck;
  return proto.teleport.devicetrust.v1.SyncInventoryAck.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryAck} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryAck}
 */
proto.teleport.devicetrust.v1.SyncInventoryAck.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.SyncInventoryAck.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.SyncInventoryAck.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryAck} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryAck.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.SyncInventoryResult.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryResult} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.toObject = function(includeInstance, msg) {
  var f, obj = {
    devicesList: jspb.Message.toObjectList(msg.getDevicesList(),
    proto.teleport.devicetrust.v1.DeviceOrStatus.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResult}
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.SyncInventoryResult;
  return proto.teleport.devicetrust.v1.SyncInventoryResult.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryResult} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResult}
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.devicetrust.v1.DeviceOrStatus;
      reader.readMessage(value,proto.teleport.devicetrust.v1.DeviceOrStatus.deserializeBinaryFromReader);
      msg.addDevices(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.SyncInventoryResult.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryResult} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.teleport.devicetrust.v1.DeviceOrStatus.serializeBinaryToWriter
    );
  }
};


/**
 * repeated DeviceOrStatus devices = 1;
 * @return {!Array<!proto.teleport.devicetrust.v1.DeviceOrStatus>}
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.prototype.getDevicesList = function() {
  return /** @type{!Array<!proto.teleport.devicetrust.v1.DeviceOrStatus>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.teleport.devicetrust.v1.DeviceOrStatus, 1));
};


/**
 * @param {!Array<!proto.teleport.devicetrust.v1.DeviceOrStatus>} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResult} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryResult.prototype.setDevicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.devicetrust.v1.DeviceOrStatus=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.devicetrust.v1.DeviceOrStatus}
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.prototype.addDevices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.devicetrust.v1.DeviceOrStatus, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryResult} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryResult.prototype.clearDevicesList = function() {
  return this.setDevicesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryMissingDevices} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.toObject = function(includeInstance, msg) {
  var f, obj = {
    devicesList: jspb.Message.toObjectList(msg.getDevicesList(),
    teleport_devicetrust_v1_device_pb.Device.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryMissingDevices}
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.SyncInventoryMissingDevices;
  return proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryMissingDevices} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryMissingDevices}
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_devicetrust_v1_device_pb.Device;
      reader.readMessage(value,teleport_devicetrust_v1_device_pb.Device.deserializeBinaryFromReader);
      msg.addDevices(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.SyncInventoryMissingDevices} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDevicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_devicetrust_v1_device_pb.Device.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Device devices = 1;
 * @return {!Array<!proto.teleport.devicetrust.v1.Device>}
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.prototype.getDevicesList = function() {
  return /** @type{!Array<!proto.teleport.devicetrust.v1.Device>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_devicetrust_v1_device_pb.Device, 1));
};


/**
 * @param {!Array<!proto.teleport.devicetrust.v1.Device>} value
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryMissingDevices} returns this
*/
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.prototype.setDevicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.devicetrust.v1.Device=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.devicetrust.v1.Device}
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.prototype.addDevices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.devicetrust.v1.Device, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.devicetrust.v1.SyncInventoryMissingDevices} returns this
 */
proto.teleport.devicetrust.v1.SyncInventoryMissingDevices.prototype.clearDevicesList = function() {
  return this.setDevicesList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.teleport.devicetrust.v1.GetDevicesUsageRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.devicetrust.v1.GetDevicesUsageRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.devicetrust.v1.GetDevicesUsageRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.GetDevicesUsageRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.teleport.devicetrust.v1.GetDevicesUsageRequest}
 */
proto.teleport.devicetrust.v1.GetDevicesUsageRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.devicetrust.v1.GetDevicesUsageRequest;
  return proto.teleport.devicetrust.v1.GetDevicesUsageRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.devicetrust.v1.GetDevicesUsageRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.devicetrust.v1.GetDevicesUsageRequest}
 */
proto.teleport.devicetrust.v1.GetDevicesUsageRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.teleport.devicetrust.v1.GetDevicesUsageRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.devicetrust.v1.GetDevicesUsageRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.devicetrust.v1.GetDevicesUsageRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.devicetrust.v1.GetDevicesUsageRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};


/**
 * @enum {number}
 */
proto.teleport.devicetrust.v1.DeviceView = {
  DEVICE_VIEW_UNSPECIFIED: 0,
  DEVICE_VIEW_LIST: 1,
  DEVICE_VIEW_RESOURCE: 2
};

goog.object.extend(exports, proto.teleport.devicetrust.v1);
