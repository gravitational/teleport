// source: teleport/lib/teleterm/v1/service.proto
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

var teleport_accesslist_v1_accesslist_pb = require('../../../../teleport/accesslist/v1/accesslist_pb.js');
goog.object.extend(proto, teleport_accesslist_v1_accesslist_pb);
var teleport_lib_teleterm_v1_access_request_pb = require('../../../../teleport/lib/teleterm/v1/access_request_pb.js');
goog.object.extend(proto, teleport_lib_teleterm_v1_access_request_pb);
var teleport_lib_teleterm_v1_app_pb = require('../../../../teleport/lib/teleterm/v1/app_pb.js');
goog.object.extend(proto, teleport_lib_teleterm_v1_app_pb);
var teleport_lib_teleterm_v1_auth_settings_pb = require('../../../../teleport/lib/teleterm/v1/auth_settings_pb.js');
goog.object.extend(proto, teleport_lib_teleterm_v1_auth_settings_pb);
var teleport_lib_teleterm_v1_cluster_pb = require('../../../../teleport/lib/teleterm/v1/cluster_pb.js');
goog.object.extend(proto, teleport_lib_teleterm_v1_cluster_pb);
var teleport_lib_teleterm_v1_database_pb = require('../../../../teleport/lib/teleterm/v1/database_pb.js');
goog.object.extend(proto, teleport_lib_teleterm_v1_database_pb);
var teleport_lib_teleterm_v1_gateway_pb = require('../../../../teleport/lib/teleterm/v1/gateway_pb.js');
goog.object.extend(proto, teleport_lib_teleterm_v1_gateway_pb);
var teleport_lib_teleterm_v1_kube_pb = require('../../../../teleport/lib/teleterm/v1/kube_pb.js');
goog.object.extend(proto, teleport_lib_teleterm_v1_kube_pb);
var teleport_lib_teleterm_v1_server_pb = require('../../../../teleport/lib/teleterm/v1/server_pb.js');
goog.object.extend(proto, teleport_lib_teleterm_v1_server_pb);
var teleport_lib_teleterm_v1_usage_events_pb = require('../../../../teleport/lib/teleterm/v1/usage_events_pb.js');
goog.object.extend(proto, teleport_lib_teleterm_v1_usage_events_pb);
var teleport_userpreferences_v1_cluster_preferences_pb = require('../../../../teleport/userpreferences/v1/cluster_preferences_pb.js');
goog.object.extend(proto, teleport_userpreferences_v1_cluster_preferences_pb);
var teleport_userpreferences_v1_unified_resource_preferences_pb = require('../../../../teleport/userpreferences/v1/unified_resource_preferences_pb.js');
goog.object.extend(proto, teleport_userpreferences_v1_unified_resource_preferences_pb);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.AddClusterRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.AssumeRoleRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.CreateGatewayRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.CredentialInfo', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.EmptyResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.FileTransferDirection', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.FileTransferProgress', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.FileTransferRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetAccessRequestRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetAccessRequestResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetAppsRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetAppsResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetClusterRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetDatabasesRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetDatabasesResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetKubesRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetKubesResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetServersRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetServersResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.HeadlessAuthenticationState', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ListClustersRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ListClustersResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ListGatewaysRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ListGatewaysResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ListLeafClustersRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.RequestCase', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginRequest.ParamsCase', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.LogoutRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.PaginatedResource', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.PaginatedResource.ResourceCase', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.PasswordlessPrompt', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.RemoveClusterRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.RemoveGatewayRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.SortBy', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.UserPreferences', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest', null, global);
goog.exportSymbol('proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse', null, global);
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
proto.teleport.lib.teleterm.v1.EmptyResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.EmptyResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.EmptyResponse.displayName = 'proto.teleport.lib.teleterm.v1.EmptyResponse';
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
proto.teleport.lib.teleterm.v1.RemoveClusterRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.RemoveClusterRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.RemoveClusterRequest.displayName = 'proto.teleport.lib.teleterm.v1.RemoveClusterRequest';
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
proto.teleport.lib.teleterm.v1.GetClusterRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetClusterRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetClusterRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetClusterRequest';
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
proto.teleport.lib.teleterm.v1.LogoutRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.LogoutRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.LogoutRequest.displayName = 'proto.teleport.lib.teleterm.v1.LogoutRequest';
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
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetAccessRequestRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetAccessRequestRequest';
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
proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest';
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
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetAccessRequestResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetAccessRequestResponse';
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
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse';
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
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.displayName = 'proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest';
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
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.displayName = 'proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest';
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
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.displayName = 'proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse';
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
proto.teleport.lib.teleterm.v1.AssumeRoleRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.AssumeRoleRequest.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.AssumeRoleRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.AssumeRoleRequest.displayName = 'proto.teleport.lib.teleterm.v1.AssumeRoleRequest';
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
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest';
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
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse';
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
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.displayName = 'proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest';
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
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.displayName = 'proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse';
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
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.displayName = 'proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest';
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
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.displayName = 'proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse';
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
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest';
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
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse';
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
proto.teleport.lib.teleterm.v1.CredentialInfo = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.CredentialInfo, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.CredentialInfo.displayName = 'proto.teleport.lib.teleterm.v1.CredentialInfo';
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.displayName = 'proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse';
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.oneofGroups_);
};
goog.inherits(proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.displayName = 'proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest';
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.displayName = 'proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit';
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.displayName = 'proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse';
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.displayName = 'proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse';
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
proto.teleport.lib.teleterm.v1.FileTransferRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.FileTransferRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.FileTransferRequest.displayName = 'proto.teleport.lib.teleterm.v1.FileTransferRequest';
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
proto.teleport.lib.teleterm.v1.FileTransferProgress = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.FileTransferProgress, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.FileTransferProgress.displayName = 'proto.teleport.lib.teleterm.v1.FileTransferProgress';
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
proto.teleport.lib.teleterm.v1.LoginRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.lib.teleterm.v1.LoginRequest.oneofGroups_);
};
goog.inherits(proto.teleport.lib.teleterm.v1.LoginRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.LoginRequest.displayName = 'proto.teleport.lib.teleterm.v1.LoginRequest';
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
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.displayName = 'proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams';
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
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.displayName = 'proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams';
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
proto.teleport.lib.teleterm.v1.AddClusterRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.AddClusterRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.AddClusterRequest.displayName = 'proto.teleport.lib.teleterm.v1.AddClusterRequest';
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
proto.teleport.lib.teleterm.v1.ListClustersRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ListClustersRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ListClustersRequest.displayName = 'proto.teleport.lib.teleterm.v1.ListClustersRequest';
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
proto.teleport.lib.teleterm.v1.ListClustersResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.ListClustersResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ListClustersResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ListClustersResponse.displayName = 'proto.teleport.lib.teleterm.v1.ListClustersResponse';
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
proto.teleport.lib.teleterm.v1.GetDatabasesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetDatabasesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetDatabasesRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetDatabasesRequest';
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
proto.teleport.lib.teleterm.v1.ListLeafClustersRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ListLeafClustersRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.displayName = 'proto.teleport.lib.teleterm.v1.ListLeafClustersRequest';
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
proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.displayName = 'proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest';
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
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.displayName = 'proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse';
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
proto.teleport.lib.teleterm.v1.CreateGatewayRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.CreateGatewayRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.CreateGatewayRequest.displayName = 'proto.teleport.lib.teleterm.v1.CreateGatewayRequest';
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
proto.teleport.lib.teleterm.v1.ListGatewaysRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ListGatewaysRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ListGatewaysRequest.displayName = 'proto.teleport.lib.teleterm.v1.ListGatewaysRequest';
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
proto.teleport.lib.teleterm.v1.ListGatewaysResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.ListGatewaysResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ListGatewaysResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ListGatewaysResponse.displayName = 'proto.teleport.lib.teleterm.v1.ListGatewaysResponse';
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
proto.teleport.lib.teleterm.v1.RemoveGatewayRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.RemoveGatewayRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.displayName = 'proto.teleport.lib.teleterm.v1.RemoveGatewayRequest';
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
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.displayName = 'proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest';
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
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.displayName = 'proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest';
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
proto.teleport.lib.teleterm.v1.GetServersRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetServersRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetServersRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetServersRequest';
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
proto.teleport.lib.teleterm.v1.GetServersResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.GetServersResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetServersResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetServersResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetServersResponse';
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
proto.teleport.lib.teleterm.v1.GetDatabasesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.GetDatabasesResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetDatabasesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetDatabasesResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetDatabasesResponse';
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
proto.teleport.lib.teleterm.v1.GetKubesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetKubesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetKubesRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetKubesRequest';
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
proto.teleport.lib.teleterm.v1.GetKubesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.GetKubesResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetKubesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetKubesResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetKubesResponse';
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
proto.teleport.lib.teleterm.v1.GetAppsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetAppsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetAppsRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetAppsRequest';
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
proto.teleport.lib.teleterm.v1.GetAppsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.GetAppsResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetAppsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetAppsResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetAppsResponse';
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
proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest';
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
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.displayName = 'proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest';
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
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.displayName = 'proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse';
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
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.displayName = 'proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest';
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
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.displayName = 'proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse';
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.displayName = 'proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest';
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.displayName = 'proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse';
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.displayName = 'proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest';
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.displayName = 'proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse';
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.displayName = 'proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest';
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.displayName = 'proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse';
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
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.displayName = 'proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest';
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
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.displayName = 'proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse';
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.displayName = 'proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest';
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.displayName = 'proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse';
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
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest';
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
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse';
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
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.displayName = 'proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest';
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
proto.teleport.lib.teleterm.v1.SortBy = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.SortBy, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.SortBy.displayName = 'proto.teleport.lib.teleterm.v1.SortBy';
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
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.repeatedFields_, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.displayName = 'proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse';
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
proto.teleport.lib.teleterm.v1.PaginatedResource = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.teleport.lib.teleterm.v1.PaginatedResource.oneofGroups_);
};
goog.inherits(proto.teleport.lib.teleterm.v1.PaginatedResource, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.PaginatedResource.displayName = 'proto.teleport.lib.teleterm.v1.PaginatedResource';
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
proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.displayName = 'proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest';
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
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.displayName = 'proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse';
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
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.displayName = 'proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest';
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
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.displayName = 'proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse';
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
proto.teleport.lib.teleterm.v1.UserPreferences = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.teleport.lib.teleterm.v1.UserPreferences, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.teleport.lib.teleterm.v1.UserPreferences.displayName = 'proto.teleport.lib.teleterm.v1.UserPreferences';
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
proto.teleport.lib.teleterm.v1.EmptyResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.EmptyResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.EmptyResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.EmptyResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.teleport.lib.teleterm.v1.EmptyResponse}
 */
proto.teleport.lib.teleterm.v1.EmptyResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.EmptyResponse;
  return proto.teleport.lib.teleterm.v1.EmptyResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.EmptyResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.EmptyResponse}
 */
proto.teleport.lib.teleterm.v1.EmptyResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.teleport.lib.teleterm.v1.EmptyResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.EmptyResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.EmptyResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.EmptyResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
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
proto.teleport.lib.teleterm.v1.RemoveClusterRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.RemoveClusterRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.RemoveClusterRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.RemoveClusterRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.RemoveClusterRequest}
 */
proto.teleport.lib.teleterm.v1.RemoveClusterRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.RemoveClusterRequest;
  return proto.teleport.lib.teleterm.v1.RemoveClusterRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.RemoveClusterRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.RemoveClusterRequest}
 */
proto.teleport.lib.teleterm.v1.RemoveClusterRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
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
proto.teleport.lib.teleterm.v1.RemoveClusterRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.RemoveClusterRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.RemoveClusterRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.RemoveClusterRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.RemoveClusterRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.RemoveClusterRequest} returns this
 */
proto.teleport.lib.teleterm.v1.RemoveClusterRequest.prototype.setClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.GetClusterRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetClusterRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetClusterRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetClusterRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetClusterRequest}
 */
proto.teleport.lib.teleterm.v1.GetClusterRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetClusterRequest;
  return proto.teleport.lib.teleterm.v1.GetClusterRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetClusterRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetClusterRequest}
 */
proto.teleport.lib.teleterm.v1.GetClusterRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
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
proto.teleport.lib.teleterm.v1.GetClusterRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetClusterRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetClusterRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetClusterRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetClusterRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetClusterRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetClusterRequest.prototype.setClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.LogoutRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.LogoutRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.LogoutRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LogoutRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.LogoutRequest}
 */
proto.teleport.lib.teleterm.v1.LogoutRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.LogoutRequest;
  return proto.teleport.lib.teleterm.v1.LogoutRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.LogoutRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.LogoutRequest}
 */
proto.teleport.lib.teleterm.v1.LogoutRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
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
proto.teleport.lib.teleterm.v1.LogoutRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.LogoutRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.LogoutRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LogoutRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.LogoutRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.LogoutRequest} returns this
 */
proto.teleport.lib.teleterm.v1.LogoutRequest.prototype.setClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accessRequestId: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetAccessRequestRequest;
  return proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccessRequestId(value);
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
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccessRequestId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.prototype.setClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string access_request_id = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.prototype.getAccessRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestRequest.prototype.setAccessRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
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
proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest;
  return proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
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
proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsRequest.prototype.setClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    request: (f = msg.getRequest()) && teleport_lib_teleterm_v1_access_request_pb.AccessRequest.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestResponse}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetAccessRequestResponse;
  return proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestResponse}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_access_request_pb.AccessRequest;
      reader.readMessage(value,teleport_lib_teleterm_v1_access_request_pb.AccessRequest.deserializeBinaryFromReader);
      msg.setRequest(value);
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
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequest();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_lib_teleterm_v1_access_request_pb.AccessRequest.serializeBinaryToWriter
    );
  }
};


/**
 * optional AccessRequest request = 1;
 * @return {?proto.teleport.lib.teleterm.v1.AccessRequest}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.prototype.getRequest = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.AccessRequest} */ (
    jspb.Message.getWrapperField(this, teleport_lib_teleterm_v1_access_request_pb.AccessRequest, 1));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.AccessRequest|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestResponse} returns this
*/
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.prototype.setRequest = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.prototype.clearRequest = function() {
  return this.setRequest(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestResponse.prototype.hasRequest = function() {
  return jspb.Message.getField(this, 1) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    requestsList: jspb.Message.toObjectList(msg.getRequestsList(),
    teleport_lib_teleterm_v1_access_request_pb.AccessRequest.toObject, includeInstance)
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
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse;
  return proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_access_request_pb.AccessRequest;
      reader.readMessage(value,teleport_lib_teleterm_v1_access_request_pb.AccessRequest.deserializeBinaryFromReader);
      msg.addRequests(value);
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
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequestsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_lib_teleterm_v1_access_request_pb.AccessRequest.serializeBinaryToWriter
    );
  }
};


/**
 * repeated AccessRequest requests = 1;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.AccessRequest>}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.prototype.getRequestsList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.AccessRequest>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_lib_teleterm_v1_access_request_pb.AccessRequest, 1));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.AccessRequest>} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse} returns this
*/
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.prototype.setRequestsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.AccessRequest=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.AccessRequest}
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.prototype.addRequests = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.lib.teleterm.v1.AccessRequest, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetAccessRequestsResponse.prototype.clearRequestsList = function() {
  return this.setRequestsList([]);
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
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accessRequestId: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest;
  return proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccessRequestId(value);
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
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccessRequestId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.prototype.setRootClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string access_request_id = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.prototype.getAccessRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.DeleteAccessRequestRequest.prototype.setAccessRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.repeatedFields_ = [3,4,5];



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
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    reason: jspb.Message.getFieldWithDefault(msg, 2, ""),
    rolesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
    suggestedReviewersList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
    resourceIdsList: jspb.Message.toObjectList(msg.getResourceIdsList(),
    teleport_lib_teleterm_v1_access_request_pb.ResourceID.toObject, includeInstance)
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
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest;
  return proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addRoles(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addSuggestedReviewers(value);
      break;
    case 5:
      var value = new teleport_lib_teleterm_v1_access_request_pb.ResourceID;
      reader.readMessage(value,teleport_lib_teleterm_v1_access_request_pb.ResourceID.deserializeBinaryFromReader);
      msg.addResourceIds(value);
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
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getRolesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getSuggestedReviewersList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getResourceIdsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      5,
      f,
      teleport_lib_teleterm_v1_access_request_pb.ResourceID.serializeBinaryToWriter
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.setRootClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string reason = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string roles = 3;
 * @return {!Array<string>}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.getRolesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.setRolesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.addRoles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.clearRolesList = function() {
  return this.setRolesList([]);
};


/**
 * repeated string suggested_reviewers = 4;
 * @return {!Array<string>}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.getSuggestedReviewersList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.setSuggestedReviewersList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.addSuggestedReviewers = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.clearSuggestedReviewersList = function() {
  return this.setSuggestedReviewersList([]);
};


/**
 * repeated ResourceID resource_ids = 5;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.ResourceID>}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.getResourceIdsList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.ResourceID>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_lib_teleterm_v1_access_request_pb.ResourceID, 5));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.ResourceID>} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
*/
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.setResourceIdsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 5, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.ResourceID=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.ResourceID}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.addResourceIds = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 5, opt_value, proto.teleport.lib.teleterm.v1.ResourceID, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestRequest.prototype.clearResourceIdsList = function() {
  return this.setResourceIdsList([]);
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
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    request: (f = msg.getRequest()) && teleport_lib_teleterm_v1_access_request_pb.AccessRequest.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse;
  return proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_access_request_pb.AccessRequest;
      reader.readMessage(value,teleport_lib_teleterm_v1_access_request_pb.AccessRequest.deserializeBinaryFromReader);
      msg.setRequest(value);
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
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequest();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_lib_teleterm_v1_access_request_pb.AccessRequest.serializeBinaryToWriter
    );
  }
};


/**
 * optional AccessRequest request = 1;
 * @return {?proto.teleport.lib.teleterm.v1.AccessRequest}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.prototype.getRequest = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.AccessRequest} */ (
    jspb.Message.getWrapperField(this, teleport_lib_teleterm_v1_access_request_pb.AccessRequest, 1));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.AccessRequest|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse} returns this
*/
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.prototype.setRequest = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse} returns this
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.prototype.clearRequest = function() {
  return this.setRequest(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.CreateAccessRequestResponse.prototype.hasRequest = function() {
  return jspb.Message.getField(this, 1) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.repeatedFields_ = [2,3];



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
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.AssumeRoleRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accessRequestIdsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f,
    dropRequestIdsList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f
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
 * @return {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest}
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.AssumeRoleRequest;
  return proto.teleport.lib.teleterm.v1.AssumeRoleRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest}
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addAccessRequestIds(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addDropRequestIds(value);
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
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.AssumeRoleRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccessRequestIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
  f = message.getDropRequestIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} returns this
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.setRootClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string access_request_ids = 2;
 * @return {!Array<string>}
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.getAccessRequestIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} returns this
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.setAccessRequestIdsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} returns this
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.addAccessRequestIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} returns this
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.clearAccessRequestIdsList = function() {
  return this.setAccessRequestIdsList([]);
};


/**
 * repeated string drop_request_ids = 3;
 * @return {!Array<string>}
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.getDropRequestIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} returns this
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.setDropRequestIdsList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} returns this
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.addDropRequestIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.AssumeRoleRequest} returns this
 */
proto.teleport.lib.teleterm.v1.AssumeRoleRequest.prototype.clearDropRequestIdsList = function() {
  return this.setDropRequestIdsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.repeatedFields_ = [2];



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
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    resourceIdsList: jspb.Message.toObjectList(msg.getResourceIdsList(),
    teleport_lib_teleterm_v1_access_request_pb.ResourceID.toObject, includeInstance)
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
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest}
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest;
  return proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest}
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
      break;
    case 2:
      var value = new teleport_lib_teleterm_v1_access_request_pb.ResourceID;
      reader.readMessage(value,teleport_lib_teleterm_v1_access_request_pb.ResourceID.deserializeBinaryFromReader);
      msg.addResourceIds(value);
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
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getResourceIdsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      teleport_lib_teleterm_v1_access_request_pb.ResourceID.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.prototype.setClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated ResourceID resource_ids = 2;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.ResourceID>}
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.prototype.getResourceIdsList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.ResourceID>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_lib_teleterm_v1_access_request_pb.ResourceID, 2));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.ResourceID>} value
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest} returns this
*/
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.prototype.setResourceIdsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.ResourceID=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.ResourceID}
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.prototype.addResourceIds = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.teleport.lib.teleterm.v1.ResourceID, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesRequest.prototype.clearResourceIdsList = function() {
  return this.setResourceIdsList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.repeatedFields_ = [1,2];



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
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    rolesList: (f = jspb.Message.getRepeatedField(msg, 1)) == null ? undefined : f,
    applicableRolesList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
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
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse}
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse;
  return proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse}
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addRoles(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addApplicableRoles(value);
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
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRolesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
  f = message.getApplicableRolesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * repeated string roles = 1;
 * @return {!Array<string>}
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.getRolesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.setRolesList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.addRoles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.clearRolesList = function() {
  return this.setRolesList([]);
};


/**
 * repeated string applicable_roles = 2;
 * @return {!Array<string>}
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.getApplicableRolesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.setApplicableRolesList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.addApplicableRoles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetRequestableRolesResponse.prototype.clearApplicableRolesList = function() {
  return this.setApplicableRolesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.repeatedFields_ = [4];



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
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    state: jspb.Message.getFieldWithDefault(msg, 2, ""),
    reason: jspb.Message.getFieldWithDefault(msg, 3, ""),
    rolesList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f,
    accessRequestId: jspb.Message.getFieldWithDefault(msg, 5, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest;
  return proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setState(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.addRoles(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccessRequestId(value);
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
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getState();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getRolesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      4,
      f
    );
  }
  f = message.getAccessRequestId();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.setRootClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string state = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.getState = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.setState = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string reason = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * repeated string roles = 4;
 * @return {!Array<string>}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.getRolesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.setRolesList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.addRoles = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.clearRolesList = function() {
  return this.setRolesList([]);
};


/**
 * optional string access_request_id = 5;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.getAccessRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestRequest.prototype.setAccessRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
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
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    request: (f = msg.getRequest()) && teleport_lib_teleterm_v1_access_request_pb.AccessRequest.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse;
  return proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_access_request_pb.AccessRequest;
      reader.readMessage(value,teleport_lib_teleterm_v1_access_request_pb.AccessRequest.deserializeBinaryFromReader);
      msg.setRequest(value);
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
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequest();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_lib_teleterm_v1_access_request_pb.AccessRequest.serializeBinaryToWriter
    );
  }
};


/**
 * optional AccessRequest request = 1;
 * @return {?proto.teleport.lib.teleterm.v1.AccessRequest}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.prototype.getRequest = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.AccessRequest} */ (
    jspb.Message.getWrapperField(this, teleport_lib_teleterm_v1_access_request_pb.AccessRequest, 1));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.AccessRequest|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse} returns this
*/
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.prototype.setRequest = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse} returns this
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.prototype.clearRequest = function() {
  return this.setRequest(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.ReviewAccessRequestResponse.prototype.hasRequest = function() {
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
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accessListId: jspb.Message.getFieldWithDefault(msg, 2, ""),
    reason: jspb.Message.getFieldWithDefault(msg, 3, ""),
    accessRequestId: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest;
  return proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccessListId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setReason(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccessRequestId(value);
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
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccessListId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getReason();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getAccessRequestId();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.setRootClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string access_list_id = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.getAccessListId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.setAccessListId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string reason = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.getReason = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.setReason = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string access_request_id = 4;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.getAccessRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest} returns this
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestRequest.prototype.setAccessRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
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
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    request: (f = msg.getRequest()) && teleport_lib_teleterm_v1_access_request_pb.AccessRequest.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse;
  return proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_access_request_pb.AccessRequest;
      reader.readMessage(value,teleport_lib_teleterm_v1_access_request_pb.AccessRequest.deserializeBinaryFromReader);
      msg.setRequest(value);
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
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRequest();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_lib_teleterm_v1_access_request_pb.AccessRequest.serializeBinaryToWriter
    );
  }
};


/**
 * optional AccessRequest request = 1;
 * @return {?proto.teleport.lib.teleterm.v1.AccessRequest}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.prototype.getRequest = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.AccessRequest} */ (
    jspb.Message.getWrapperField(this, teleport_lib_teleterm_v1_access_request_pb.AccessRequest, 1));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.AccessRequest|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse} returns this
*/
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.prototype.setRequest = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse} returns this
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.prototype.clearRequest = function() {
  return this.setRequest(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.PromoteAccessRequestResponse.prototype.hasRequest = function() {
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
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    accessRequestId: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest}
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest;
  return proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest}
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccessRequestId(value);
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
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAccessRequestId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.prototype.setRootClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string access_request_id = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.prototype.getAccessRequestId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsRequest.prototype.setAccessRequestId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    accessListsList: jspb.Message.toObjectList(msg.getAccessListsList(),
    teleport_accesslist_v1_accesslist_pb.AccessList.toObject, includeInstance)
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
 * @return {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse}
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse;
  return proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse}
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_accesslist_v1_accesslist_pb.AccessList;
      reader.readMessage(value,teleport_accesslist_v1_accesslist_pb.AccessList.deserializeBinaryFromReader);
      msg.addAccessLists(value);
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
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAccessListsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_accesslist_v1_accesslist_pb.AccessList.serializeBinaryToWriter
    );
  }
};


/**
 * repeated teleport.accesslist.v1.AccessList access_lists = 1;
 * @return {!Array<!proto.teleport.accesslist.v1.AccessList>}
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.prototype.getAccessListsList = function() {
  return /** @type{!Array<!proto.teleport.accesslist.v1.AccessList>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_accesslist_v1_accesslist_pb.AccessList, 1));
};


/**
 * @param {!Array<!proto.teleport.accesslist.v1.AccessList>} value
 * @return {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse} returns this
*/
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.prototype.setAccessListsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.accesslist.v1.AccessList=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.accesslist.v1.AccessList}
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.prototype.addAccessLists = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.accesslist.v1.AccessList, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetSuggestedAccessListsResponse.prototype.clearAccessListsList = function() {
  return this.setAccessListsList([]);
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
proto.teleport.lib.teleterm.v1.CredentialInfo.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.CredentialInfo.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.CredentialInfo} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CredentialInfo.toObject = function(includeInstance, msg) {
  var f, obj = {
    username: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.CredentialInfo}
 */
proto.teleport.lib.teleterm.v1.CredentialInfo.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.CredentialInfo;
  return proto.teleport.lib.teleterm.v1.CredentialInfo.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.CredentialInfo} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.CredentialInfo}
 */
proto.teleport.lib.teleterm.v1.CredentialInfo.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUsername(value);
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
proto.teleport.lib.teleterm.v1.CredentialInfo.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.CredentialInfo.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.CredentialInfo} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CredentialInfo.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUsername();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string username = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CredentialInfo.prototype.getUsername = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CredentialInfo} returns this
 */
proto.teleport.lib.teleterm.v1.CredentialInfo.prototype.setUsername = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.repeatedFields_ = [2];



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
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    prompt: jspb.Message.getFieldWithDefault(msg, 1, 0),
    credentialsList: jspb.Message.toObjectList(msg.getCredentialsList(),
    proto.teleport.lib.teleterm.v1.CredentialInfo.toObject, includeInstance)
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
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse;
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.teleport.lib.teleterm.v1.PasswordlessPrompt} */ (reader.readEnum());
      msg.setPrompt(value);
      break;
    case 2:
      var value = new proto.teleport.lib.teleterm.v1.CredentialInfo;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.CredentialInfo.deserializeBinaryFromReader);
      msg.addCredentials(value);
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPrompt();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getCredentialsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      2,
      f,
      proto.teleport.lib.teleterm.v1.CredentialInfo.serializeBinaryToWriter
    );
  }
};


/**
 * optional PasswordlessPrompt prompt = 1;
 * @return {!proto.teleport.lib.teleterm.v1.PasswordlessPrompt}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.prototype.getPrompt = function() {
  return /** @type {!proto.teleport.lib.teleterm.v1.PasswordlessPrompt} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.PasswordlessPrompt} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse} returns this
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.prototype.setPrompt = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * repeated CredentialInfo credentials = 2;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.CredentialInfo>}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.prototype.getCredentialsList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.CredentialInfo>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.teleport.lib.teleterm.v1.CredentialInfo, 2));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.CredentialInfo>} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse} returns this
*/
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.prototype.setCredentialsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 2, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.CredentialInfo=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.CredentialInfo}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.prototype.addCredentials = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 2, opt_value, proto.teleport.lib.teleterm.v1.CredentialInfo, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse} returns this
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessResponse.prototype.clearCredentialsList = function() {
  return this.setCredentialsList([]);
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.oneofGroups_ = [[1,2,3]];

/**
 * @enum {number}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.RequestCase = {
  REQUEST_NOT_SET: 0,
  INIT: 1,
  PIN: 2,
  CREDENTIAL: 3
};

/**
 * @return {proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.RequestCase}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.getRequestCase = function() {
  return /** @type {proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.RequestCase} */(jspb.Message.computeOneofCase(this, proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.oneofGroups_[0]));
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    init: (f = msg.getInit()) && proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.toObject(includeInstance, f),
    pin: (f = msg.getPin()) && proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.toObject(includeInstance, f),
    credential: (f = msg.getCredential()) && proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest;
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.deserializeBinaryFromReader);
      msg.setInit(value);
      break;
    case 2:
      var value = new proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.deserializeBinaryFromReader);
      msg.setPin(value);
      break;
    case 3:
      var value = new proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.deserializeBinaryFromReader);
      msg.setCredential(value);
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getInit();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.serializeBinaryToWriter
    );
  }
  f = message.getPin();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.serializeBinaryToWriter
    );
  }
  f = message.getCredential();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.serializeBinaryToWriter
    );
  }
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit;
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit} returns this
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit.prototype.setClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    pin: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse;
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setPin(value);
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPin();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string pin = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.prototype.getPin = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse} returns this
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse.prototype.setPin = function(value) {
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    index: jspb.Message.getFieldWithDefault(msg, 1, 0)
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
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse;
  return proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setIndex(value);
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
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIndex();
  if (f !== 0) {
    writer.writeInt64(
      1,
      f
    );
  }
};


/**
 * optional int64 index = 1;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.prototype.getIndex = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse} returns this
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse.prototype.setIndex = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional LoginPasswordlessRequestInit init = 1;
 * @return {?proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.getInit = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit} */ (
    jspb.Message.getWrapperField(this, proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit, 1));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessRequestInit|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest} returns this
*/
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.setInit = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest} returns this
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.clearInit = function() {
  return this.setInit(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.hasInit = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional LoginPasswordlessPINResponse pin = 2;
 * @return {?proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.getPin = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse} */ (
    jspb.Message.getWrapperField(this, proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse, 2));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessPINResponse|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest} returns this
*/
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.setPin = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest} returns this
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.clearPin = function() {
  return this.setPin(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.hasPin = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional LoginPasswordlessCredentialResponse credential = 3;
 * @return {?proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.getCredential = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse} */ (
    jspb.Message.getWrapperField(this, proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse, 3));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest} returns this
*/
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.setCredential = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest} returns this
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.clearCredential = function() {
  return this.setCredential(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.LoginPasswordlessRequest.prototype.hasCredential = function() {
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
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.FileTransferRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.FileTransferRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    login: jspb.Message.getFieldWithDefault(msg, 2, ""),
    source: jspb.Message.getFieldWithDefault(msg, 4, ""),
    destination: jspb.Message.getFieldWithDefault(msg, 5, ""),
    direction: jspb.Message.getFieldWithDefault(msg, 6, 0),
    serverUri: jspb.Message.getFieldWithDefault(msg, 7, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferRequest}
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.FileTransferRequest;
  return proto.teleport.lib.teleterm.v1.FileTransferRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.FileTransferRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferRequest}
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setLogin(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSource(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setDestination(value);
      break;
    case 6:
      var value = /** @type {!proto.teleport.lib.teleterm.v1.FileTransferDirection} */ (reader.readEnum());
      msg.setDirection(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setServerUri(value);
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
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.FileTransferRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.FileTransferRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getLogin();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSource();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getDestination();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getDirection();
  if (f !== 0.0) {
    writer.writeEnum(
      6,
      f
    );
  }
  f = message.getServerUri();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
};


/**
 * optional string login = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.getLogin = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferRequest} returns this
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.setLogin = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string source = 4;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.getSource = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferRequest} returns this
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.setSource = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string destination = 5;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.getDestination = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferRequest} returns this
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.setDestination = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional FileTransferDirection direction = 6;
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferDirection}
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.getDirection = function() {
  return /** @type {!proto.teleport.lib.teleterm.v1.FileTransferDirection} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.FileTransferDirection} value
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferRequest} returns this
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.setDirection = function(value) {
  return jspb.Message.setProto3EnumField(this, 6, value);
};


/**
 * optional string server_uri = 7;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.getServerUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferRequest} returns this
 */
proto.teleport.lib.teleterm.v1.FileTransferRequest.prototype.setServerUri = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
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
proto.teleport.lib.teleterm.v1.FileTransferProgress.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.FileTransferProgress.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.FileTransferProgress} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.FileTransferProgress.toObject = function(includeInstance, msg) {
  var f, obj = {
    percentage: jspb.Message.getFieldWithDefault(msg, 1, 0)
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
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferProgress}
 */
proto.teleport.lib.teleterm.v1.FileTransferProgress.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.FileTransferProgress;
  return proto.teleport.lib.teleterm.v1.FileTransferProgress.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.FileTransferProgress} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferProgress}
 */
proto.teleport.lib.teleterm.v1.FileTransferProgress.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setPercentage(value);
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
proto.teleport.lib.teleterm.v1.FileTransferProgress.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.FileTransferProgress.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.FileTransferProgress} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.FileTransferProgress.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPercentage();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
};


/**
 * optional uint32 percentage = 1;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.FileTransferProgress.prototype.getPercentage = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.FileTransferProgress} returns this
 */
proto.teleport.lib.teleterm.v1.FileTransferProgress.prototype.setPercentage = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.lib.teleterm.v1.LoginRequest.oneofGroups_ = [[2,3]];

/**
 * @enum {number}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.ParamsCase = {
  PARAMS_NOT_SET: 0,
  LOCAL: 2,
  SSO: 3
};

/**
 * @return {proto.teleport.lib.teleterm.v1.LoginRequest.ParamsCase}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.getParamsCase = function() {
  return /** @type {proto.teleport.lib.teleterm.v1.LoginRequest.ParamsCase} */(jspb.Message.computeOneofCase(this, proto.teleport.lib.teleterm.v1.LoginRequest.oneofGroups_[0]));
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
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.LoginRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.LoginRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    local: (f = msg.getLocal()) && proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.toObject(includeInstance, f),
    sso: (f = msg.getSso()) && proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.LoginRequest;
  return proto.teleport.lib.teleterm.v1.LoginRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.LoginRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
      break;
    case 2:
      var value = new proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.deserializeBinaryFromReader);
      msg.setLocal(value);
      break;
    case 3:
      var value = new proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.deserializeBinaryFromReader);
      msg.setSso(value);
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
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.LoginRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.LoginRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLocal();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.serializeBinaryToWriter
    );
  }
  f = message.getSso();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.serializeBinaryToWriter
    );
  }
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
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.toObject = function(includeInstance, msg) {
  var f, obj = {
    user: jspb.Message.getFieldWithDefault(msg, 1, ""),
    password: jspb.Message.getFieldWithDefault(msg, 2, ""),
    token: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams;
  return proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUser(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setPassword(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setToken(value);
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
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUser();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPassword();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getToken();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string user = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.prototype.getUser = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams} returns this
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.prototype.setUser = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string password = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.prototype.getPassword = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams} returns this
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.prototype.setPassword = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string token = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.prototype.getToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams} returns this
 */
proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams.prototype.setToken = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
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
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.toObject = function(includeInstance, msg) {
  var f, obj = {
    providerType: jspb.Message.getFieldWithDefault(msg, 1, ""),
    providerName: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams;
  return proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setProviderType(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setProviderName(value);
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
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getProviderType();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getProviderName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string provider_type = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.prototype.getProviderType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams} returns this
 */
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.prototype.setProviderType = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string provider_name = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.prototype.getProviderName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams} returns this
 */
proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams.prototype.setProviderName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest} returns this
 */
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.setClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional LocalParams local = 2;
 * @return {?proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.getLocal = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams} */ (
    jspb.Message.getWrapperField(this, proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams, 2));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.LoginRequest.LocalParams|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest} returns this
*/
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.setLocal = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.teleport.lib.teleterm.v1.LoginRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest} returns this
 */
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.clearLocal = function() {
  return this.setLocal(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.hasLocal = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional SsoParams sso = 3;
 * @return {?proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.getSso = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams} */ (
    jspb.Message.getWrapperField(this, proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams, 3));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.LoginRequest.SsoParams|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest} returns this
*/
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.setSso = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.teleport.lib.teleterm.v1.LoginRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.LoginRequest} returns this
 */
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.clearSso = function() {
  return this.setSso(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.LoginRequest.prototype.hasSso = function() {
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
proto.teleport.lib.teleterm.v1.AddClusterRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.AddClusterRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.AddClusterRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.AddClusterRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.AddClusterRequest}
 */
proto.teleport.lib.teleterm.v1.AddClusterRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.AddClusterRequest;
  return proto.teleport.lib.teleterm.v1.AddClusterRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.AddClusterRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.AddClusterRequest}
 */
proto.teleport.lib.teleterm.v1.AddClusterRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
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
proto.teleport.lib.teleterm.v1.AddClusterRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.AddClusterRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.AddClusterRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.AddClusterRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.AddClusterRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.AddClusterRequest} returns this
 */
proto.teleport.lib.teleterm.v1.AddClusterRequest.prototype.setName = function(value) {
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
proto.teleport.lib.teleterm.v1.ListClustersRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ListClustersRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ListClustersRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListClustersRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.teleport.lib.teleterm.v1.ListClustersRequest}
 */
proto.teleport.lib.teleterm.v1.ListClustersRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ListClustersRequest;
  return proto.teleport.lib.teleterm.v1.ListClustersRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ListClustersRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ListClustersRequest}
 */
proto.teleport.lib.teleterm.v1.ListClustersRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.teleport.lib.teleterm.v1.ListClustersRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ListClustersRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ListClustersRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListClustersRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.ListClustersResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.ListClustersResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ListClustersResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ListClustersResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListClustersResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    clustersList: jspb.Message.toObjectList(msg.getClustersList(),
    teleport_lib_teleterm_v1_cluster_pb.Cluster.toObject, includeInstance)
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
 * @return {!proto.teleport.lib.teleterm.v1.ListClustersResponse}
 */
proto.teleport.lib.teleterm.v1.ListClustersResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ListClustersResponse;
  return proto.teleport.lib.teleterm.v1.ListClustersResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ListClustersResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ListClustersResponse}
 */
proto.teleport.lib.teleterm.v1.ListClustersResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_cluster_pb.Cluster;
      reader.readMessage(value,teleport_lib_teleterm_v1_cluster_pb.Cluster.deserializeBinaryFromReader);
      msg.addClusters(value);
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
proto.teleport.lib.teleterm.v1.ListClustersResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ListClustersResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ListClustersResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListClustersResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClustersList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_lib_teleterm_v1_cluster_pb.Cluster.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Cluster clusters = 1;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.Cluster>}
 */
proto.teleport.lib.teleterm.v1.ListClustersResponse.prototype.getClustersList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.Cluster>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_lib_teleterm_v1_cluster_pb.Cluster, 1));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.Cluster>} value
 * @return {!proto.teleport.lib.teleterm.v1.ListClustersResponse} returns this
*/
proto.teleport.lib.teleterm.v1.ListClustersResponse.prototype.setClustersList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.Cluster=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.Cluster}
 */
proto.teleport.lib.teleterm.v1.ListClustersResponse.prototype.addClusters = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.lib.teleterm.v1.Cluster, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.ListClustersResponse} returns this
 */
proto.teleport.lib.teleterm.v1.ListClustersResponse.prototype.clearClustersList = function() {
  return this.setClustersList([]);
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
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetDatabasesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    limit: jspb.Message.getFieldWithDefault(msg, 2, 0),
    startKey: jspb.Message.getFieldWithDefault(msg, 3, ""),
    search: jspb.Message.getFieldWithDefault(msg, 4, ""),
    query: jspb.Message.getFieldWithDefault(msg, 5, ""),
    sortBy: jspb.Message.getFieldWithDefault(msg, 6, ""),
    searchAsRoles: jspb.Message.getFieldWithDefault(msg, 7, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetDatabasesRequest;
  return proto.teleport.lib.teleterm.v1.GetDatabasesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStartKey(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSearch(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setQuery(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setSortBy(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setSearchAsRoles(value);
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
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetDatabasesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getStartKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSearch();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getSortBy();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getSearchAsRoles();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.setClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 limit = 2;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string start_key = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.getStartKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.setStartKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string search = 4;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.getSearch = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.setSearch = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string query = 5;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string sort_by = 6;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.getSortBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.setSortBy = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string search_as_roles = 7;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.getSearchAsRoles = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesRequest.prototype.setSearchAsRoles = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
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
proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ListLeafClustersRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.ListLeafClustersRequest}
 */
proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ListLeafClustersRequest;
  return proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ListLeafClustersRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ListLeafClustersRequest}
 */
proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
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
proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ListLeafClustersRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ListLeafClustersRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListLeafClustersRequest.prototype.setClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    dbUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest}
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest;
  return proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest}
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDbUri(value);
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
proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDbUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string db_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.prototype.getDbUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersRequest.prototype.setDbUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    usersList: (f = jspb.Message.getRepeatedField(msg, 1)) == null ? undefined : f
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
 * @return {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse}
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse;
  return proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse}
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.addUsers(value);
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
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUsersList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      1,
      f
    );
  }
};


/**
 * repeated string users = 1;
 * @return {!Array<string>}
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.prototype.getUsersList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 1));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse} returns this
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.prototype.setUsersList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse} returns this
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.prototype.addUsers = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse} returns this
 */
proto.teleport.lib.teleterm.v1.ListDatabaseUsersResponse.prototype.clearUsersList = function() {
  return this.setUsersList([]);
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
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.CreateGatewayRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.CreateGatewayRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    targetUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    targetUser: jspb.Message.getFieldWithDefault(msg, 2, ""),
    localPort: jspb.Message.getFieldWithDefault(msg, 3, ""),
    targetSubresourceName: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.CreateGatewayRequest}
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.CreateGatewayRequest;
  return proto.teleport.lib.teleterm.v1.CreateGatewayRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.CreateGatewayRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.CreateGatewayRequest}
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetUser(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setLocalPort(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetSubresourceName(value);
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
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.CreateGatewayRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.CreateGatewayRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTargetUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTargetUser();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getLocalPort();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getTargetSubresourceName();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string target_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.getTargetUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateGatewayRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.setTargetUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string target_user = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.getTargetUser = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateGatewayRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.setTargetUser = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string local_port = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.getLocalPort = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateGatewayRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.setLocalPort = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string target_subresource_name = 4;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.getTargetSubresourceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateGatewayRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateGatewayRequest.prototype.setTargetSubresourceName = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
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
proto.teleport.lib.teleterm.v1.ListGatewaysRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ListGatewaysRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ListGatewaysRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListGatewaysRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.teleport.lib.teleterm.v1.ListGatewaysRequest}
 */
proto.teleport.lib.teleterm.v1.ListGatewaysRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ListGatewaysRequest;
  return proto.teleport.lib.teleterm.v1.ListGatewaysRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ListGatewaysRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ListGatewaysRequest}
 */
proto.teleport.lib.teleterm.v1.ListGatewaysRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.teleport.lib.teleterm.v1.ListGatewaysRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ListGatewaysRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ListGatewaysRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListGatewaysRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ListGatewaysResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ListGatewaysResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    gatewaysList: jspb.Message.toObjectList(msg.getGatewaysList(),
    teleport_lib_teleterm_v1_gateway_pb.Gateway.toObject, includeInstance)
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
 * @return {!proto.teleport.lib.teleterm.v1.ListGatewaysResponse}
 */
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ListGatewaysResponse;
  return proto.teleport.lib.teleterm.v1.ListGatewaysResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ListGatewaysResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ListGatewaysResponse}
 */
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_gateway_pb.Gateway;
      reader.readMessage(value,teleport_lib_teleterm_v1_gateway_pb.Gateway.deserializeBinaryFromReader);
      msg.addGateways(value);
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
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ListGatewaysResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ListGatewaysResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGatewaysList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_lib_teleterm_v1_gateway_pb.Gateway.serializeBinaryToWriter
    );
  }
};


/**
 * repeated Gateway gateways = 1;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.Gateway>}
 */
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.prototype.getGatewaysList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.Gateway>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_lib_teleterm_v1_gateway_pb.Gateway, 1));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.Gateway>} value
 * @return {!proto.teleport.lib.teleterm.v1.ListGatewaysResponse} returns this
*/
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.prototype.setGatewaysList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.Gateway=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.Gateway}
 */
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.prototype.addGateways = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.lib.teleterm.v1.Gateway, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.ListGatewaysResponse} returns this
 */
proto.teleport.lib.teleterm.v1.ListGatewaysResponse.prototype.clearGatewaysList = function() {
  return this.setGatewaysList([]);
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
proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.RemoveGatewayRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    gatewayUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.RemoveGatewayRequest}
 */
proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.RemoveGatewayRequest;
  return proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.RemoveGatewayRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.RemoveGatewayRequest}
 */
proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setGatewayUri(value);
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
proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.RemoveGatewayRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGatewayUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string gateway_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.prototype.getGatewayUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.RemoveGatewayRequest} returns this
 */
proto.teleport.lib.teleterm.v1.RemoveGatewayRequest.prototype.setGatewayUri = function(value) {
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
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    gatewayUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    targetSubresourceName: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest}
 */
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest;
  return proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest}
 */
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setGatewayUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setTargetSubresourceName(value);
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
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGatewayUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTargetSubresourceName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string gateway_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.prototype.getGatewayUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest} returns this
 */
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.prototype.setGatewayUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string target_subresource_name = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.prototype.getTargetSubresourceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest} returns this
 */
proto.teleport.lib.teleterm.v1.SetGatewayTargetSubresourceNameRequest.prototype.setTargetSubresourceName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
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
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    gatewayUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    localPort: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest}
 */
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest;
  return proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest}
 */
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setGatewayUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setLocalPort(value);
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
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGatewayUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLocalPort();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string gateway_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.prototype.getGatewayUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest} returns this
 */
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.prototype.setGatewayUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string local_port = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.prototype.getLocalPort = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest} returns this
 */
proto.teleport.lib.teleterm.v1.SetGatewayLocalPortRequest.prototype.setLocalPort = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
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
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetServersRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetServersRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    limit: jspb.Message.getFieldWithDefault(msg, 2, 0),
    startKey: jspb.Message.getFieldWithDefault(msg, 3, ""),
    search: jspb.Message.getFieldWithDefault(msg, 4, ""),
    query: jspb.Message.getFieldWithDefault(msg, 5, ""),
    sortBy: jspb.Message.getFieldWithDefault(msg, 6, ""),
    searchAsRoles: jspb.Message.getFieldWithDefault(msg, 7, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetServersRequest}
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetServersRequest;
  return proto.teleport.lib.teleterm.v1.GetServersRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetServersRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetServersRequest}
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStartKey(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSearch(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setQuery(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setSortBy(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setSearchAsRoles(value);
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
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetServersRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetServersRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getStartKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSearch();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getSortBy();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getSearchAsRoles();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.setClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 limit = 2;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string start_key = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.getStartKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.setStartKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string search = 4;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.getSearch = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.setSearch = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string query = 5;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string sort_by = 6;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.getSortBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.setSortBy = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string search_as_roles = 7;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.getSearchAsRoles = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersRequest.prototype.setSearchAsRoles = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetServersResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetServersResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    agentsList: jspb.Message.toObjectList(msg.getAgentsList(),
    teleport_lib_teleterm_v1_server_pb.Server.toObject, includeInstance),
    totalCount: jspb.Message.getFieldWithDefault(msg, 2, 0),
    startKey: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetServersResponse}
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetServersResponse;
  return proto.teleport.lib.teleterm.v1.GetServersResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetServersResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetServersResponse}
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_server_pb.Server;
      reader.readMessage(value,teleport_lib_teleterm_v1_server_pb.Server.deserializeBinaryFromReader);
      msg.addAgents(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTotalCount(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStartKey(value);
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
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetServersResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetServersResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAgentsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_lib_teleterm_v1_server_pb.Server.serializeBinaryToWriter
    );
  }
  f = message.getTotalCount();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getStartKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * repeated Server agents = 1;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.Server>}
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.getAgentsList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.Server>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_lib_teleterm_v1_server_pb.Server, 1));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.Server>} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersResponse} returns this
*/
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.setAgentsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.Server=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.Server}
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.addAgents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.lib.teleterm.v1.Server, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.GetServersResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.clearAgentsList = function() {
  return this.setAgentsList([]);
};


/**
 * optional int32 total_count = 2;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.getTotalCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.setTotalCount = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string start_key = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.getStartKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetServersResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetServersResponse.prototype.setStartKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetDatabasesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetDatabasesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    agentsList: jspb.Message.toObjectList(msg.getAgentsList(),
    teleport_lib_teleterm_v1_database_pb.Database.toObject, includeInstance),
    totalCount: jspb.Message.getFieldWithDefault(msg, 2, 0),
    startKey: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesResponse}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetDatabasesResponse;
  return proto.teleport.lib.teleterm.v1.GetDatabasesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetDatabasesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesResponse}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_database_pb.Database;
      reader.readMessage(value,teleport_lib_teleterm_v1_database_pb.Database.deserializeBinaryFromReader);
      msg.addAgents(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTotalCount(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStartKey(value);
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
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetDatabasesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetDatabasesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAgentsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_lib_teleterm_v1_database_pb.Database.serializeBinaryToWriter
    );
  }
  f = message.getTotalCount();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getStartKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * repeated Database agents = 1;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.Database>}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.getAgentsList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.Database>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_lib_teleterm_v1_database_pb.Database, 1));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.Database>} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesResponse} returns this
*/
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.setAgentsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.Database=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.Database}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.addAgents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.lib.teleterm.v1.Database, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.clearAgentsList = function() {
  return this.setAgentsList([]);
};


/**
 * optional int32 total_count = 2;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.getTotalCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.setTotalCount = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string start_key = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.getStartKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetDatabasesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetDatabasesResponse.prototype.setStartKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
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
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetKubesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetKubesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    limit: jspb.Message.getFieldWithDefault(msg, 2, 0),
    startKey: jspb.Message.getFieldWithDefault(msg, 3, ""),
    search: jspb.Message.getFieldWithDefault(msg, 4, ""),
    query: jspb.Message.getFieldWithDefault(msg, 5, ""),
    sortBy: jspb.Message.getFieldWithDefault(msg, 6, ""),
    searchAsRoles: jspb.Message.getFieldWithDefault(msg, 7, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesRequest}
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetKubesRequest;
  return proto.teleport.lib.teleterm.v1.GetKubesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetKubesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesRequest}
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStartKey(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSearch(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setQuery(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setSortBy(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setSearchAsRoles(value);
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
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetKubesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetKubesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getStartKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSearch();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getSortBy();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getSearchAsRoles();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.setClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 limit = 2;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string start_key = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.getStartKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.setStartKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string search = 4;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.getSearch = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.setSearch = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string query = 5;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string sort_by = 6;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.getSortBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.setSortBy = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string search_as_roles = 7;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.getSearchAsRoles = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesRequest.prototype.setSearchAsRoles = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetKubesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetKubesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    agentsList: jspb.Message.toObjectList(msg.getAgentsList(),
    teleport_lib_teleterm_v1_kube_pb.Kube.toObject, includeInstance),
    totalCount: jspb.Message.getFieldWithDefault(msg, 2, 0),
    startKey: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesResponse}
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetKubesResponse;
  return proto.teleport.lib.teleterm.v1.GetKubesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetKubesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesResponse}
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_kube_pb.Kube;
      reader.readMessage(value,teleport_lib_teleterm_v1_kube_pb.Kube.deserializeBinaryFromReader);
      msg.addAgents(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTotalCount(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStartKey(value);
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
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetKubesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetKubesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAgentsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_lib_teleterm_v1_kube_pb.Kube.serializeBinaryToWriter
    );
  }
  f = message.getTotalCount();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getStartKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * repeated Kube agents = 1;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.Kube>}
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.getAgentsList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.Kube>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_lib_teleterm_v1_kube_pb.Kube, 1));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.Kube>} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesResponse} returns this
*/
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.setAgentsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.Kube=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.Kube}
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.addAgents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.lib.teleterm.v1.Kube, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.clearAgentsList = function() {
  return this.setAgentsList([]);
};


/**
 * optional int32 total_count = 2;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.getTotalCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.setTotalCount = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string start_key = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.getStartKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetKubesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetKubesResponse.prototype.setStartKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
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
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetAppsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetAppsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    limit: jspb.Message.getFieldWithDefault(msg, 2, 0),
    startKey: jspb.Message.getFieldWithDefault(msg, 3, ""),
    search: jspb.Message.getFieldWithDefault(msg, 4, ""),
    query: jspb.Message.getFieldWithDefault(msg, 5, ""),
    sortBy: jspb.Message.getFieldWithDefault(msg, 6, ""),
    searchAsRoles: jspb.Message.getFieldWithDefault(msg, 7, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsRequest}
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetAppsRequest;
  return proto.teleport.lib.teleterm.v1.GetAppsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetAppsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsRequest}
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStartKey(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setSearch(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setQuery(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setSortBy(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setSearchAsRoles(value);
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
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetAppsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetAppsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getStartKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getSearch();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getSortBy();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getSearchAsRoles();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.setClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 limit = 2;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string start_key = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.getStartKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.setStartKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string search = 4;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.getSearch = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.setSearch = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string query = 5;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string sort_by = 6;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.getSortBy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.setSortBy = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string search_as_roles = 7;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.getSearchAsRoles = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsRequest.prototype.setSearchAsRoles = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetAppsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetAppsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    agentsList: jspb.Message.toObjectList(msg.getAgentsList(),
    teleport_lib_teleterm_v1_app_pb.App.toObject, includeInstance),
    totalCount: jspb.Message.getFieldWithDefault(msg, 2, 0),
    startKey: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsResponse}
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetAppsResponse;
  return proto.teleport.lib.teleterm.v1.GetAppsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetAppsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsResponse}
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_app_pb.App;
      reader.readMessage(value,teleport_lib_teleterm_v1_app_pb.App.deserializeBinaryFromReader);
      msg.addAgents(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTotalCount(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStartKey(value);
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
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetAppsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetAppsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAgentsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      teleport_lib_teleterm_v1_app_pb.App.serializeBinaryToWriter
    );
  }
  f = message.getTotalCount();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getStartKey();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * repeated App agents = 1;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.App>}
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.getAgentsList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.App>} */ (
    jspb.Message.getRepeatedWrapperField(this, teleport_lib_teleterm_v1_app_pb.App, 1));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.App>} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsResponse} returns this
*/
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.setAgentsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.App=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.App}
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.addAgents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.lib.teleterm.v1.App, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.clearAgentsList = function() {
  return this.setAgentsList([]);
};


/**
 * optional int32 total_count = 2;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.getTotalCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.setTotalCount = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string start_key = 3;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.getStartKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAppsResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetAppsResponse.prototype.setStartKey = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
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
proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest}
 */
proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest;
  return proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest}
 */
proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
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
proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetAuthSettingsRequest.prototype.setClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    address: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest}
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest;
  return proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest}
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setAddress(value);
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
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAddress();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string address = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.prototype.getAddress = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest} returns this
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressRequest.prototype.setAddress = function(value) {
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
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse}
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse;
  return proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse}
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateTshdEventsServerAddressResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
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
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    headlessAuthenticationId: jspb.Message.getFieldWithDefault(msg, 2, ""),
    state: jspb.Message.getFieldWithDefault(msg, 3, 0)
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
 * @return {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest}
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest;
  return proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest}
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setHeadlessAuthenticationId(value);
      break;
    case 3:
      var value = /** @type {!proto.teleport.lib.teleterm.v1.HeadlessAuthenticationState} */ (reader.readEnum());
      msg.setState(value);
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
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getHeadlessAuthenticationId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getState();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest} returns this
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.prototype.setRootClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string headless_authentication_id = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.prototype.getHeadlessAuthenticationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest} returns this
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.prototype.setHeadlessAuthenticationId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional HeadlessAuthenticationState state = 3;
 * @return {!proto.teleport.lib.teleterm.v1.HeadlessAuthenticationState}
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.prototype.getState = function() {
  return /** @type {!proto.teleport.lib.teleterm.v1.HeadlessAuthenticationState} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.HeadlessAuthenticationState} value
 * @return {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest} returns this
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateRequest.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
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
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse}
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse;
  return proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse}
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateHeadlessAuthenticationStateResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest;
  return proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleRequest.prototype.setRootClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    certsReloaded: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
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
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse;
  return proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setCertsReloaded(value);
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCertsReloaded();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool certs_reloaded = 1;
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.prototype.getCertsReloaded = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse} returns this
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerRoleResponse.prototype.setCertsReloaded = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest;
  return proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest} returns this
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenRequest.prototype.setRootClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    token: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse;
  return proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getToken();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string token = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.prototype.getToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse} returns this
 */
proto.teleport.lib.teleterm.v1.CreateConnectMyComputerNodeTokenResponse.prototype.setToken = function(value) {
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    token: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest;
  return proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setToken(value);
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest} returns this
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.prototype.setRootClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string token = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.prototype.getToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest} returns this
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenRequest.prototype.setToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse;
  return proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerTokenResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
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
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest}
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest;
  return proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest}
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
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
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest} returns this
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinRequest.prototype.setRootClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    server: (f = msg.getServer()) && teleport_lib_teleterm_v1_server_pb.Server.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse}
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse;
  return proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse}
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_server_pb.Server;
      reader.readMessage(value,teleport_lib_teleterm_v1_server_pb.Server.deserializeBinaryFromReader);
      msg.setServer(value);
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
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServer();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_lib_teleterm_v1_server_pb.Server.serializeBinaryToWriter
    );
  }
};


/**
 * optional Server server = 1;
 * @return {?proto.teleport.lib.teleterm.v1.Server}
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.prototype.getServer = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.Server} */ (
    jspb.Message.getWrapperField(this, teleport_lib_teleterm_v1_server_pb.Server, 1));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.Server|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse} returns this
*/
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.prototype.setServer = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse} returns this
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.prototype.clearServer = function() {
  return this.setServer(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.WaitForConnectMyComputerNodeJoinResponse.prototype.hasServer = function() {
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest;
  return proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest} returns this
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeRequest.prototype.setRootClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse;
  return proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse}
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.DeleteConnectMyComputerNodeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
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
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    rootClusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest}
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest;
  return proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest}
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setRootClusterUri(value);
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
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getRootClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string root_cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.prototype.getRootClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameRequest.prototype.setRootClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse}
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse;
  return proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse}
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
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
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetConnectMyComputerNodeNameResponse.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.repeatedFields_ = [2];



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
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    kindsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f,
    limit: jspb.Message.getFieldWithDefault(msg, 3, 0),
    startKey: jspb.Message.getFieldWithDefault(msg, 4, ""),
    query: jspb.Message.getFieldWithDefault(msg, 5, ""),
    search: jspb.Message.getFieldWithDefault(msg, 6, ""),
    sortBy: (f = msg.getSortBy()) && proto.teleport.lib.teleterm.v1.SortBy.toObject(includeInstance, f),
    searchAsRoles: jspb.Message.getBooleanFieldWithDefault(msg, 8, false),
    pinnedOnly: jspb.Message.getBooleanFieldWithDefault(msg, 9, false)
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
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest;
  return proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.addKinds(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setLimit(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setStartKey(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setQuery(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setSearch(value);
      break;
    case 7:
      var value = new proto.teleport.lib.teleterm.v1.SortBy;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.SortBy.deserializeBinaryFromReader);
      msg.setSortBy(value);
      break;
    case 8:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setSearchAsRoles(value);
      break;
    case 9:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setPinnedOnly(value);
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
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getKindsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
  f = message.getLimit();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getStartKey();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getQuery();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getSearch();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getSortBy();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.teleport.lib.teleterm.v1.SortBy.serializeBinaryToWriter
    );
  }
  f = message.getSearchAsRoles();
  if (f) {
    writer.writeBool(
      8,
      f
    );
  }
  f = message.getPinnedOnly();
  if (f) {
    writer.writeBool(
      9,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.setClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string kinds = 2;
 * @return {!Array<string>}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.getKindsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.setKindsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.addKinds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.clearKindsList = function() {
  return this.setKindsList([]);
};


/**
 * optional int32 limit = 3;
 * @return {number}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.getLimit = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.setLimit = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional string start_key = 4;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.getStartKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.setStartKey = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string query = 5;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.getQuery = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.setQuery = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string search = 6;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.getSearch = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.setSearch = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional SortBy sort_by = 7;
 * @return {?proto.teleport.lib.teleterm.v1.SortBy}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.getSortBy = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.SortBy} */ (
    jspb.Message.getWrapperField(this, proto.teleport.lib.teleterm.v1.SortBy, 7));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.SortBy|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
*/
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.setSortBy = function(value) {
  return jspb.Message.setWrapperField(this, 7, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.clearSortBy = function() {
  return this.setSortBy(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.hasSortBy = function() {
  return jspb.Message.getField(this, 7) != null;
};


/**
 * optional bool search_as_roles = 8;
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.getSearchAsRoles = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 8, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.setSearchAsRoles = function(value) {
  return jspb.Message.setProto3BooleanField(this, 8, value);
};


/**
 * optional bool pinned_only = 9;
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.getPinnedOnly = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 9, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesRequest.prototype.setPinnedOnly = function(value) {
  return jspb.Message.setProto3BooleanField(this, 9, value);
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
proto.teleport.lib.teleterm.v1.SortBy.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.SortBy.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.SortBy} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.SortBy.toObject = function(includeInstance, msg) {
  var f, obj = {
    isDesc: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
    field: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.SortBy}
 */
proto.teleport.lib.teleterm.v1.SortBy.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.SortBy;
  return proto.teleport.lib.teleterm.v1.SortBy.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.SortBy} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.SortBy}
 */
proto.teleport.lib.teleterm.v1.SortBy.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIsDesc(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setField(value);
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
proto.teleport.lib.teleterm.v1.SortBy.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.SortBy.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.SortBy} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.SortBy.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIsDesc();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getField();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional bool is_desc = 1;
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.SortBy.prototype.getIsDesc = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.teleport.lib.teleterm.v1.SortBy} returns this
 */
proto.teleport.lib.teleterm.v1.SortBy.prototype.setIsDesc = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional string field = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.SortBy.prototype.getField = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.SortBy} returns this
 */
proto.teleport.lib.teleterm.v1.SortBy.prototype.setField = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.repeatedFields_ = [1];



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
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    resourcesList: jspb.Message.toObjectList(msg.getResourcesList(),
    proto.teleport.lib.teleterm.v1.PaginatedResource.toObject, includeInstance),
    nextKey: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse;
  return proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.lib.teleterm.v1.PaginatedResource;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.PaginatedResource.deserializeBinaryFromReader);
      msg.addResources(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNextKey(value);
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
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResourcesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.teleport.lib.teleterm.v1.PaginatedResource.serializeBinaryToWriter
    );
  }
  f = message.getNextKey();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated PaginatedResource resources = 1;
 * @return {!Array<!proto.teleport.lib.teleterm.v1.PaginatedResource>}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.prototype.getResourcesList = function() {
  return /** @type{!Array<!proto.teleport.lib.teleterm.v1.PaginatedResource>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.teleport.lib.teleterm.v1.PaginatedResource, 1));
};


/**
 * @param {!Array<!proto.teleport.lib.teleterm.v1.PaginatedResource>} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse} returns this
*/
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.prototype.setResourcesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.teleport.lib.teleterm.v1.PaginatedResource=} opt_value
 * @param {number=} opt_index
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.prototype.addResources = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.teleport.lib.teleterm.v1.PaginatedResource, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.prototype.clearResourcesList = function() {
  return this.setResourcesList([]);
};


/**
 * optional string next_key = 2;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.prototype.getNextKey = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.ListUnifiedResourcesResponse.prototype.setNextKey = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.oneofGroups_ = [[1,2,3,4]];

/**
 * @enum {number}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.ResourceCase = {
  RESOURCE_NOT_SET: 0,
  DATABASE: 1,
  SERVER: 2,
  KUBE: 3,
  APP: 4
};

/**
 * @return {proto.teleport.lib.teleterm.v1.PaginatedResource.ResourceCase}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.getResourceCase = function() {
  return /** @type {proto.teleport.lib.teleterm.v1.PaginatedResource.ResourceCase} */(jspb.Message.computeOneofCase(this, proto.teleport.lib.teleterm.v1.PaginatedResource.oneofGroups_[0]));
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
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.PaginatedResource.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.PaginatedResource} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.toObject = function(includeInstance, msg) {
  var f, obj = {
    database: (f = msg.getDatabase()) && teleport_lib_teleterm_v1_database_pb.Database.toObject(includeInstance, f),
    server: (f = msg.getServer()) && teleport_lib_teleterm_v1_server_pb.Server.toObject(includeInstance, f),
    kube: (f = msg.getKube()) && teleport_lib_teleterm_v1_kube_pb.Kube.toObject(includeInstance, f),
    app: (f = msg.getApp()) && teleport_lib_teleterm_v1_app_pb.App.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.PaginatedResource;
  return proto.teleport.lib.teleterm.v1.PaginatedResource.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.PaginatedResource} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_lib_teleterm_v1_database_pb.Database;
      reader.readMessage(value,teleport_lib_teleterm_v1_database_pb.Database.deserializeBinaryFromReader);
      msg.setDatabase(value);
      break;
    case 2:
      var value = new teleport_lib_teleterm_v1_server_pb.Server;
      reader.readMessage(value,teleport_lib_teleterm_v1_server_pb.Server.deserializeBinaryFromReader);
      msg.setServer(value);
      break;
    case 3:
      var value = new teleport_lib_teleterm_v1_kube_pb.Kube;
      reader.readMessage(value,teleport_lib_teleterm_v1_kube_pb.Kube.deserializeBinaryFromReader);
      msg.setKube(value);
      break;
    case 4:
      var value = new teleport_lib_teleterm_v1_app_pb.App;
      reader.readMessage(value,teleport_lib_teleterm_v1_app_pb.App.deserializeBinaryFromReader);
      msg.setApp(value);
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
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.PaginatedResource.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.PaginatedResource} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDatabase();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_lib_teleterm_v1_database_pb.Database.serializeBinaryToWriter
    );
  }
  f = message.getServer();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      teleport_lib_teleterm_v1_server_pb.Server.serializeBinaryToWriter
    );
  }
  f = message.getKube();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      teleport_lib_teleterm_v1_kube_pb.Kube.serializeBinaryToWriter
    );
  }
  f = message.getApp();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      teleport_lib_teleterm_v1_app_pb.App.serializeBinaryToWriter
    );
  }
};


/**
 * optional Database database = 1;
 * @return {?proto.teleport.lib.teleterm.v1.Database}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.getDatabase = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.Database} */ (
    jspb.Message.getWrapperField(this, teleport_lib_teleterm_v1_database_pb.Database, 1));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.Database|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource} returns this
*/
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.setDatabase = function(value) {
  return jspb.Message.setOneofWrapperField(this, 1, proto.teleport.lib.teleterm.v1.PaginatedResource.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource} returns this
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.clearDatabase = function() {
  return this.setDatabase(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.hasDatabase = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional Server server = 2;
 * @return {?proto.teleport.lib.teleterm.v1.Server}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.getServer = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.Server} */ (
    jspb.Message.getWrapperField(this, teleport_lib_teleterm_v1_server_pb.Server, 2));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.Server|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource} returns this
*/
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.setServer = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.teleport.lib.teleterm.v1.PaginatedResource.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource} returns this
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.clearServer = function() {
  return this.setServer(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.hasServer = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional Kube kube = 3;
 * @return {?proto.teleport.lib.teleterm.v1.Kube}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.getKube = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.Kube} */ (
    jspb.Message.getWrapperField(this, teleport_lib_teleterm_v1_kube_pb.Kube, 3));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.Kube|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource} returns this
*/
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.setKube = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.teleport.lib.teleterm.v1.PaginatedResource.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource} returns this
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.clearKube = function() {
  return this.setKube(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.hasKube = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional App app = 4;
 * @return {?proto.teleport.lib.teleterm.v1.App}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.getApp = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.App} */ (
    jspb.Message.getWrapperField(this, teleport_lib_teleterm_v1_app_pb.App, 4));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.App|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource} returns this
*/
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.setApp = function(value) {
  return jspb.Message.setOneofWrapperField(this, 4, proto.teleport.lib.teleterm.v1.PaginatedResource.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.PaginatedResource} returns this
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.clearApp = function() {
  return this.setApp(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.PaginatedResource.prototype.hasApp = function() {
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
proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest}
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest;
  return proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest}
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
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
proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesRequest.prototype.setClusterUri = function(value) {
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
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    userPreferences: (f = msg.getUserPreferences()) && proto.teleport.lib.teleterm.v1.UserPreferences.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse}
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse;
  return proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse}
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.lib.teleterm.v1.UserPreferences;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.UserPreferences.deserializeBinaryFromReader);
      msg.setUserPreferences(value);
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
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserPreferences();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.lib.teleterm.v1.UserPreferences.serializeBinaryToWriter
    );
  }
};


/**
 * optional UserPreferences user_preferences = 1;
 * @return {?proto.teleport.lib.teleterm.v1.UserPreferences}
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.prototype.getUserPreferences = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.UserPreferences} */ (
    jspb.Message.getWrapperField(this, proto.teleport.lib.teleterm.v1.UserPreferences, 1));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.UserPreferences|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse} returns this
*/
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.prototype.setUserPreferences = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.prototype.clearUserPreferences = function() {
  return this.setUserPreferences(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.GetUserPreferencesResponse.prototype.hasUserPreferences = function() {
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
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterUri: jspb.Message.getFieldWithDefault(msg, 1, ""),
    userPreferences: (f = msg.getUserPreferences()) && proto.teleport.lib.teleterm.v1.UserPreferences.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest}
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest;
  return proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest}
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterUri(value);
      break;
    case 2:
      var value = new proto.teleport.lib.teleterm.v1.UserPreferences;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.UserPreferences.deserializeBinaryFromReader);
      msg.setUserPreferences(value);
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
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterUri();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUserPreferences();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.teleport.lib.teleterm.v1.UserPreferences.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_uri = 1;
 * @return {string}
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.prototype.getClusterUri = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.prototype.setClusterUri = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional UserPreferences user_preferences = 2;
 * @return {?proto.teleport.lib.teleterm.v1.UserPreferences}
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.prototype.getUserPreferences = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.UserPreferences} */ (
    jspb.Message.getWrapperField(this, proto.teleport.lib.teleterm.v1.UserPreferences, 2));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.UserPreferences|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest} returns this
*/
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.prototype.setUserPreferences = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest} returns this
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.prototype.clearUserPreferences = function() {
  return this.setUserPreferences(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesRequest.prototype.hasUserPreferences = function() {
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
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    userPreferences: (f = msg.getUserPreferences()) && proto.teleport.lib.teleterm.v1.UserPreferences.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse}
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse;
  return proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse}
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.teleport.lib.teleterm.v1.UserPreferences;
      reader.readMessage(value,proto.teleport.lib.teleterm.v1.UserPreferences.deserializeBinaryFromReader);
      msg.setUserPreferences(value);
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
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserPreferences();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.teleport.lib.teleterm.v1.UserPreferences.serializeBinaryToWriter
    );
  }
};


/**
 * optional UserPreferences user_preferences = 1;
 * @return {?proto.teleport.lib.teleterm.v1.UserPreferences}
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.prototype.getUserPreferences = function() {
  return /** @type{?proto.teleport.lib.teleterm.v1.UserPreferences} */ (
    jspb.Message.getWrapperField(this, proto.teleport.lib.teleterm.v1.UserPreferences, 1));
};


/**
 * @param {?proto.teleport.lib.teleterm.v1.UserPreferences|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse} returns this
*/
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.prototype.setUserPreferences = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse} returns this
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.prototype.clearUserPreferences = function() {
  return this.setUserPreferences(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.UpdateUserPreferencesResponse.prototype.hasUserPreferences = function() {
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
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.toObject = function(opt_includeInstance) {
  return proto.teleport.lib.teleterm.v1.UserPreferences.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.teleport.lib.teleterm.v1.UserPreferences} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UserPreferences.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterPreferences: (f = msg.getClusterPreferences()) && teleport_userpreferences_v1_cluster_preferences_pb.ClusterUserPreferences.toObject(includeInstance, f),
    unifiedResourcePreferences: (f = msg.getUnifiedResourcePreferences()) && teleport_userpreferences_v1_unified_resource_preferences_pb.UnifiedResourcePreferences.toObject(includeInstance, f)
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
 * @return {!proto.teleport.lib.teleterm.v1.UserPreferences}
 */
proto.teleport.lib.teleterm.v1.UserPreferences.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.teleport.lib.teleterm.v1.UserPreferences;
  return proto.teleport.lib.teleterm.v1.UserPreferences.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.teleport.lib.teleterm.v1.UserPreferences} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.teleport.lib.teleterm.v1.UserPreferences}
 */
proto.teleport.lib.teleterm.v1.UserPreferences.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new teleport_userpreferences_v1_cluster_preferences_pb.ClusterUserPreferences;
      reader.readMessage(value,teleport_userpreferences_v1_cluster_preferences_pb.ClusterUserPreferences.deserializeBinaryFromReader);
      msg.setClusterPreferences(value);
      break;
    case 2:
      var value = new teleport_userpreferences_v1_unified_resource_preferences_pb.UnifiedResourcePreferences;
      reader.readMessage(value,teleport_userpreferences_v1_unified_resource_preferences_pb.UnifiedResourcePreferences.deserializeBinaryFromReader);
      msg.setUnifiedResourcePreferences(value);
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
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.teleport.lib.teleterm.v1.UserPreferences.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.teleport.lib.teleterm.v1.UserPreferences} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.teleport.lib.teleterm.v1.UserPreferences.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterPreferences();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      teleport_userpreferences_v1_cluster_preferences_pb.ClusterUserPreferences.serializeBinaryToWriter
    );
  }
  f = message.getUnifiedResourcePreferences();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      teleport_userpreferences_v1_unified_resource_preferences_pb.UnifiedResourcePreferences.serializeBinaryToWriter
    );
  }
};


/**
 * optional teleport.userpreferences.v1.ClusterUserPreferences cluster_preferences = 1;
 * @return {?proto.teleport.userpreferences.v1.ClusterUserPreferences}
 */
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.getClusterPreferences = function() {
  return /** @type{?proto.teleport.userpreferences.v1.ClusterUserPreferences} */ (
    jspb.Message.getWrapperField(this, teleport_userpreferences_v1_cluster_preferences_pb.ClusterUserPreferences, 1));
};


/**
 * @param {?proto.teleport.userpreferences.v1.ClusterUserPreferences|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.UserPreferences} returns this
*/
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.setClusterPreferences = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.UserPreferences} returns this
 */
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.clearClusterPreferences = function() {
  return this.setClusterPreferences(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.hasClusterPreferences = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional teleport.userpreferences.v1.UnifiedResourcePreferences unified_resource_preferences = 2;
 * @return {?proto.teleport.userpreferences.v1.UnifiedResourcePreferences}
 */
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.getUnifiedResourcePreferences = function() {
  return /** @type{?proto.teleport.userpreferences.v1.UnifiedResourcePreferences} */ (
    jspb.Message.getWrapperField(this, teleport_userpreferences_v1_unified_resource_preferences_pb.UnifiedResourcePreferences, 2));
};


/**
 * @param {?proto.teleport.userpreferences.v1.UnifiedResourcePreferences|undefined} value
 * @return {!proto.teleport.lib.teleterm.v1.UserPreferences} returns this
*/
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.setUnifiedResourcePreferences = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.teleport.lib.teleterm.v1.UserPreferences} returns this
 */
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.clearUnifiedResourcePreferences = function() {
  return this.setUnifiedResourcePreferences(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.teleport.lib.teleterm.v1.UserPreferences.prototype.hasUnifiedResourcePreferences = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * @enum {number}
 */
proto.teleport.lib.teleterm.v1.PasswordlessPrompt = {
  PASSWORDLESS_PROMPT_UNSPECIFIED: 0,
  PASSWORDLESS_PROMPT_PIN: 1,
  PASSWORDLESS_PROMPT_TAP: 2,
  PASSWORDLESS_PROMPT_CREDENTIAL: 3
};

/**
 * @enum {number}
 */
proto.teleport.lib.teleterm.v1.FileTransferDirection = {
  FILE_TRANSFER_DIRECTION_UNSPECIFIED: 0,
  FILE_TRANSFER_DIRECTION_DOWNLOAD: 1,
  FILE_TRANSFER_DIRECTION_UPLOAD: 2
};

/**
 * @enum {number}
 */
proto.teleport.lib.teleterm.v1.HeadlessAuthenticationState = {
  HEADLESS_AUTHENTICATION_STATE_UNSPECIFIED: 0,
  HEADLESS_AUTHENTICATION_STATE_PENDING: 1,
  HEADLESS_AUTHENTICATION_STATE_DENIED: 2,
  HEADLESS_AUTHENTICATION_STATE_APPROVED: 3
};

goog.object.extend(exports, proto.teleport.lib.teleterm.v1);
