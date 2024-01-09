// source: prehog/v1alpha/teleport.proto
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

var google_protobuf_duration_pb = require('google-protobuf/google/protobuf/duration_pb.js');
goog.object.extend(proto, google_protobuf_duration_pb);
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');
goog.object.extend(proto, google_protobuf_timestamp_pb);
goog.exportSymbol('proto.prehog.v1alpha.AccessListCreateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListDeleteEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListGrantsToUserEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListMemberCreateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListMemberDeleteEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListMemberUpdateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListMetadata', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListReviewComplianceEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListReviewCreateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListReviewDeleteEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AccessListUpdateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AgentMetadataEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AssistAccessRequestEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AssistActionEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AssistCompletionEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AssistExecutionEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AssistNewConversationEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.AuditQueryRunEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.BotCreateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.BotJoinEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.CTA', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DesktopClipboardEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DesktopDirectoryShareEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DeviceAuthenticateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DeviceEnrollEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DiscoverMetadata', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DiscoverResource', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DiscoverResourceMetadata', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DiscoverStatus', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DiscoverStepStatus', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DiscoveredDatabaseMetadata', null, global);
goog.exportSymbol('proto.prehog.v1alpha.DiscoveryFetchEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.EditorChangeEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.EditorChangeStatus', null, global);
goog.exportSymbol('proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.Feature', null, global);
goog.exportSymbol('proto.prehog.v1alpha.FeatureRecommendationEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.FeatureRecommendationStatus', null, global);
goog.exportSymbol('proto.prehog.v1alpha.HelloTeleportRequest', null, global);
goog.exportSymbol('proto.prehog.v1alpha.HelloTeleportResponse', null, global);
goog.exportSymbol('proto.prehog.v1alpha.IntegrationEnrollKind', null, global);
goog.exportSymbol('proto.prehog.v1alpha.IntegrationEnrollMetadata', null, global);
goog.exportSymbol('proto.prehog.v1alpha.KubeRequestEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.LicenseLimit', null, global);
goog.exportSymbol('proto.prehog.v1alpha.LicenseLimitEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.ResourceCreateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.ResourceHeartbeatEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.ResourceKind', null, global);
goog.exportSymbol('proto.prehog.v1alpha.RoleCreateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SFTPEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SSOCreateEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SecurityReportGetResultEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SessionStartDatabaseMetadata', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SessionStartDesktopMetadata', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SessionStartEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SubmitEventRequest', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SubmitEventRequest.EventCase', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SubmitEventResponse', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SubmitEventsRequest', null, global);
goog.exportSymbol('proto.prehog.v1alpha.SubmitEventsResponse', null, global);
goog.exportSymbol('proto.prehog.v1alpha.TAGExecuteQueryEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIBannerClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UICallToActionClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UICreateNewRoleClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverCompletedEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverCreateNodeEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDeployEICEEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDeployServiceEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployMethod', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployType', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverStartedEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIDiscoverTestConnectionEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIIntegrationEnrollStartEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UserCertificateIssuedEvent', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UserKind', null, global);
goog.exportSymbol('proto.prehog.v1alpha.UserLoginEvent', null, global);
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
proto.prehog.v1alpha.UserLoginEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UserLoginEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UserLoginEvent.displayName = 'proto.prehog.v1alpha.UserLoginEvent';
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
proto.prehog.v1alpha.SSOCreateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.SSOCreateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SSOCreateEvent.displayName = 'proto.prehog.v1alpha.SSOCreateEvent';
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
proto.prehog.v1alpha.ResourceCreateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.ResourceCreateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.ResourceCreateEvent.displayName = 'proto.prehog.v1alpha.ResourceCreateEvent';
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
proto.prehog.v1alpha.DiscoveredDatabaseMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.DiscoveredDatabaseMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.DiscoveredDatabaseMetadata.displayName = 'proto.prehog.v1alpha.DiscoveredDatabaseMetadata';
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
proto.prehog.v1alpha.ResourceHeartbeatEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.ResourceHeartbeatEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.ResourceHeartbeatEvent.displayName = 'proto.prehog.v1alpha.ResourceHeartbeatEvent';
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
proto.prehog.v1alpha.SessionStartEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.SessionStartEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SessionStartEvent.displayName = 'proto.prehog.v1alpha.SessionStartEvent';
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
proto.prehog.v1alpha.SessionStartDatabaseMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.SessionStartDatabaseMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SessionStartDatabaseMetadata.displayName = 'proto.prehog.v1alpha.SessionStartDatabaseMetadata';
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
proto.prehog.v1alpha.SessionStartDesktopMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.SessionStartDesktopMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SessionStartDesktopMetadata.displayName = 'proto.prehog.v1alpha.SessionStartDesktopMetadata';
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
proto.prehog.v1alpha.UserCertificateIssuedEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UserCertificateIssuedEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UserCertificateIssuedEvent.displayName = 'proto.prehog.v1alpha.UserCertificateIssuedEvent';
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
proto.prehog.v1alpha.UIBannerClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIBannerClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIBannerClickEvent.displayName = 'proto.prehog.v1alpha.UIBannerClickEvent';
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
proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.displayName = 'proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent';
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
proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.displayName = 'proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent';
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
proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.displayName = 'proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent';
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
proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.displayName = 'proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent';
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
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.displayName = 'proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent';
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
proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.displayName = 'proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent';
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
proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.displayName = 'proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent';
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
proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.displayName = 'proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent';
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
proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.displayName = 'proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent';
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
proto.prehog.v1alpha.DiscoverMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.DiscoverMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.DiscoverMetadata.displayName = 'proto.prehog.v1alpha.DiscoverMetadata';
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
proto.prehog.v1alpha.DiscoverResourceMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.DiscoverResourceMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.DiscoverResourceMetadata.displayName = 'proto.prehog.v1alpha.DiscoverResourceMetadata';
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
proto.prehog.v1alpha.DiscoverStepStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.DiscoverStepStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.DiscoverStepStatus.displayName = 'proto.prehog.v1alpha.DiscoverStepStatus';
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
proto.prehog.v1alpha.UIDiscoverStartedEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverStartedEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverStartedEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverStartedEvent';
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
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent';
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
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent';
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
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent';
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
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverDeployServiceEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverDeployServiceEvent';
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
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent';
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
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent';
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
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent';
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
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent';
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
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent';
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
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent';
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
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverDeployEICEEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverDeployEICEEvent';
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
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverCreateNodeEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverCreateNodeEvent';
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
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent';
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
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent';
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
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverTestConnectionEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverTestConnectionEvent';
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
proto.prehog.v1alpha.UIDiscoverCompletedEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIDiscoverCompletedEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIDiscoverCompletedEvent.displayName = 'proto.prehog.v1alpha.UIDiscoverCompletedEvent';
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
proto.prehog.v1alpha.RoleCreateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.RoleCreateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.RoleCreateEvent.displayName = 'proto.prehog.v1alpha.RoleCreateEvent';
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
proto.prehog.v1alpha.BotCreateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.BotCreateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.BotCreateEvent.displayName = 'proto.prehog.v1alpha.BotCreateEvent';
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
proto.prehog.v1alpha.BotJoinEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.BotJoinEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.BotJoinEvent.displayName = 'proto.prehog.v1alpha.BotJoinEvent';
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
proto.prehog.v1alpha.UICreateNewRoleClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UICreateNewRoleClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UICreateNewRoleClickEvent.displayName = 'proto.prehog.v1alpha.UICreateNewRoleClickEvent';
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
proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.displayName = 'proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent';
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
proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.displayName = 'proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent';
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
proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.displayName = 'proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent';
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
proto.prehog.v1alpha.UICallToActionClickEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UICallToActionClickEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UICallToActionClickEvent.displayName = 'proto.prehog.v1alpha.UICallToActionClickEvent';
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
proto.prehog.v1alpha.KubeRequestEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.KubeRequestEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.KubeRequestEvent.displayName = 'proto.prehog.v1alpha.KubeRequestEvent';
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
proto.prehog.v1alpha.SFTPEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.SFTPEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SFTPEvent.displayName = 'proto.prehog.v1alpha.SFTPEvent';
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
proto.prehog.v1alpha.AgentMetadataEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.prehog.v1alpha.AgentMetadataEvent.repeatedFields_, null);
};
goog.inherits(proto.prehog.v1alpha.AgentMetadataEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AgentMetadataEvent.displayName = 'proto.prehog.v1alpha.AgentMetadataEvent';
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
proto.prehog.v1alpha.AssistCompletionEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AssistCompletionEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AssistCompletionEvent.displayName = 'proto.prehog.v1alpha.AssistCompletionEvent';
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
proto.prehog.v1alpha.AssistExecutionEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AssistExecutionEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AssistExecutionEvent.displayName = 'proto.prehog.v1alpha.AssistExecutionEvent';
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
proto.prehog.v1alpha.AssistNewConversationEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AssistNewConversationEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AssistNewConversationEvent.displayName = 'proto.prehog.v1alpha.AssistNewConversationEvent';
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
proto.prehog.v1alpha.AssistAccessRequestEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AssistAccessRequestEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AssistAccessRequestEvent.displayName = 'proto.prehog.v1alpha.AssistAccessRequestEvent';
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
proto.prehog.v1alpha.AssistActionEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AssistActionEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AssistActionEvent.displayName = 'proto.prehog.v1alpha.AssistActionEvent';
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
proto.prehog.v1alpha.AccessListMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListMetadata.displayName = 'proto.prehog.v1alpha.AccessListMetadata';
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
proto.prehog.v1alpha.AccessListCreateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListCreateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListCreateEvent.displayName = 'proto.prehog.v1alpha.AccessListCreateEvent';
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
proto.prehog.v1alpha.AccessListUpdateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListUpdateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListUpdateEvent.displayName = 'proto.prehog.v1alpha.AccessListUpdateEvent';
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
proto.prehog.v1alpha.AccessListDeleteEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListDeleteEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListDeleteEvent.displayName = 'proto.prehog.v1alpha.AccessListDeleteEvent';
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
proto.prehog.v1alpha.AccessListMemberCreateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListMemberCreateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListMemberCreateEvent.displayName = 'proto.prehog.v1alpha.AccessListMemberCreateEvent';
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
proto.prehog.v1alpha.AccessListMemberUpdateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListMemberUpdateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListMemberUpdateEvent.displayName = 'proto.prehog.v1alpha.AccessListMemberUpdateEvent';
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
proto.prehog.v1alpha.AccessListMemberDeleteEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListMemberDeleteEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListMemberDeleteEvent.displayName = 'proto.prehog.v1alpha.AccessListMemberDeleteEvent';
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
proto.prehog.v1alpha.AccessListGrantsToUserEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListGrantsToUserEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListGrantsToUserEvent.displayName = 'proto.prehog.v1alpha.AccessListGrantsToUserEvent';
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
proto.prehog.v1alpha.AccessListReviewCreateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListReviewCreateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListReviewCreateEvent.displayName = 'proto.prehog.v1alpha.AccessListReviewCreateEvent';
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
proto.prehog.v1alpha.AccessListReviewDeleteEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListReviewDeleteEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListReviewDeleteEvent.displayName = 'proto.prehog.v1alpha.AccessListReviewDeleteEvent';
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
proto.prehog.v1alpha.AccessListReviewComplianceEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AccessListReviewComplianceEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AccessListReviewComplianceEvent.displayName = 'proto.prehog.v1alpha.AccessListReviewComplianceEvent';
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
proto.prehog.v1alpha.IntegrationEnrollMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.IntegrationEnrollMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.IntegrationEnrollMetadata.displayName = 'proto.prehog.v1alpha.IntegrationEnrollMetadata';
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
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIIntegrationEnrollStartEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.displayName = 'proto.prehog.v1alpha.UIIntegrationEnrollStartEvent';
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
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.displayName = 'proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent';
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
proto.prehog.v1alpha.EditorChangeEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.EditorChangeEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.EditorChangeEvent.displayName = 'proto.prehog.v1alpha.EditorChangeEvent';
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
proto.prehog.v1alpha.DeviceAuthenticateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.DeviceAuthenticateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.DeviceAuthenticateEvent.displayName = 'proto.prehog.v1alpha.DeviceAuthenticateEvent';
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
proto.prehog.v1alpha.DeviceEnrollEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.DeviceEnrollEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.DeviceEnrollEvent.displayName = 'proto.prehog.v1alpha.DeviceEnrollEvent';
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
proto.prehog.v1alpha.FeatureRecommendationEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.FeatureRecommendationEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.FeatureRecommendationEvent.displayName = 'proto.prehog.v1alpha.FeatureRecommendationEvent';
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
proto.prehog.v1alpha.LicenseLimitEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.LicenseLimitEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.LicenseLimitEvent.displayName = 'proto.prehog.v1alpha.LicenseLimitEvent';
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
proto.prehog.v1alpha.DesktopDirectoryShareEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.DesktopDirectoryShareEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.DesktopDirectoryShareEvent.displayName = 'proto.prehog.v1alpha.DesktopDirectoryShareEvent';
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
proto.prehog.v1alpha.DesktopClipboardEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.DesktopClipboardEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.DesktopClipboardEvent.displayName = 'proto.prehog.v1alpha.DesktopClipboardEvent';
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
proto.prehog.v1alpha.TAGExecuteQueryEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.TAGExecuteQueryEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.TAGExecuteQueryEvent.displayName = 'proto.prehog.v1alpha.TAGExecuteQueryEvent';
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
proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.displayName = 'proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent';
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
proto.prehog.v1alpha.SecurityReportGetResultEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.SecurityReportGetResultEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SecurityReportGetResultEvent.displayName = 'proto.prehog.v1alpha.SecurityReportGetResultEvent';
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
proto.prehog.v1alpha.AuditQueryRunEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.AuditQueryRunEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.AuditQueryRunEvent.displayName = 'proto.prehog.v1alpha.AuditQueryRunEvent';
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
proto.prehog.v1alpha.DiscoveryFetchEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.DiscoveryFetchEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.DiscoveryFetchEvent.displayName = 'proto.prehog.v1alpha.DiscoveryFetchEvent';
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
proto.prehog.v1alpha.SubmitEventRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_);
};
goog.inherits(proto.prehog.v1alpha.SubmitEventRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SubmitEventRequest.displayName = 'proto.prehog.v1alpha.SubmitEventRequest';
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
proto.prehog.v1alpha.SubmitEventResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.SubmitEventResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SubmitEventResponse.displayName = 'proto.prehog.v1alpha.SubmitEventResponse';
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
proto.prehog.v1alpha.SubmitEventsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.prehog.v1alpha.SubmitEventsRequest.repeatedFields_, null);
};
goog.inherits(proto.prehog.v1alpha.SubmitEventsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SubmitEventsRequest.displayName = 'proto.prehog.v1alpha.SubmitEventsRequest';
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
proto.prehog.v1alpha.SubmitEventsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.SubmitEventsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.SubmitEventsResponse.displayName = 'proto.prehog.v1alpha.SubmitEventsResponse';
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
proto.prehog.v1alpha.HelloTeleportRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.HelloTeleportRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.HelloTeleportRequest.displayName = 'proto.prehog.v1alpha.HelloTeleportRequest';
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
proto.prehog.v1alpha.HelloTeleportResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.prehog.v1alpha.HelloTeleportResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.prehog.v1alpha.HelloTeleportResponse.displayName = 'proto.prehog.v1alpha.HelloTeleportResponse';
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
proto.prehog.v1alpha.UserLoginEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UserLoginEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UserLoginEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UserLoginEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    connectorType: jspb.Message.getFieldWithDefault(msg, 2, ""),
    deviceId: jspb.Message.getFieldWithDefault(msg, 3, ""),
    requiredPrivateKeyPolicy: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.prehog.v1alpha.UserLoginEvent}
 */
proto.prehog.v1alpha.UserLoginEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UserLoginEvent;
  return proto.prehog.v1alpha.UserLoginEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UserLoginEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UserLoginEvent}
 */
proto.prehog.v1alpha.UserLoginEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setConnectorType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDeviceId(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setRequiredPrivateKeyPolicy(value);
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
proto.prehog.v1alpha.UserLoginEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UserLoginEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UserLoginEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UserLoginEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getConnectorType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDeviceId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getRequiredPrivateKeyPolicy();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UserLoginEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UserLoginEvent} returns this
 */
proto.prehog.v1alpha.UserLoginEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string connector_type = 2;
 * @return {string}
 */
proto.prehog.v1alpha.UserLoginEvent.prototype.getConnectorType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UserLoginEvent} returns this
 */
proto.prehog.v1alpha.UserLoginEvent.prototype.setConnectorType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string device_id = 3;
 * @return {string}
 */
proto.prehog.v1alpha.UserLoginEvent.prototype.getDeviceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UserLoginEvent} returns this
 */
proto.prehog.v1alpha.UserLoginEvent.prototype.setDeviceId = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string required_private_key_policy = 4;
 * @return {string}
 */
proto.prehog.v1alpha.UserLoginEvent.prototype.getRequiredPrivateKeyPolicy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UserLoginEvent} returns this
 */
proto.prehog.v1alpha.UserLoginEvent.prototype.setRequiredPrivateKeyPolicy = function(value) {
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
proto.prehog.v1alpha.SSOCreateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SSOCreateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SSOCreateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SSOCreateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    connectorType: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.SSOCreateEvent}
 */
proto.prehog.v1alpha.SSOCreateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SSOCreateEvent;
  return proto.prehog.v1alpha.SSOCreateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SSOCreateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SSOCreateEvent}
 */
proto.prehog.v1alpha.SSOCreateEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setConnectorType(value);
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
proto.prehog.v1alpha.SSOCreateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SSOCreateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SSOCreateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SSOCreateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getConnectorType();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string connector_type = 1;
 * @return {string}
 */
proto.prehog.v1alpha.SSOCreateEvent.prototype.getConnectorType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SSOCreateEvent} returns this
 */
proto.prehog.v1alpha.SSOCreateEvent.prototype.setConnectorType = function(value) {
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
proto.prehog.v1alpha.ResourceCreateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.ResourceCreateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.ResourceCreateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.ResourceCreateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    resourceType: jspb.Message.getFieldWithDefault(msg, 1, ""),
    resourceOrigin: jspb.Message.getFieldWithDefault(msg, 2, ""),
    cloudProvider: jspb.Message.getFieldWithDefault(msg, 3, ""),
    database: (f = msg.getDatabase()) && proto.prehog.v1alpha.DiscoveredDatabaseMetadata.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.ResourceCreateEvent}
 */
proto.prehog.v1alpha.ResourceCreateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.ResourceCreateEvent;
  return proto.prehog.v1alpha.ResourceCreateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.ResourceCreateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.ResourceCreateEvent}
 */
proto.prehog.v1alpha.ResourceCreateEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceType(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceOrigin(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setCloudProvider(value);
      break;
    case 4:
      var value = new proto.prehog.v1alpha.DiscoveredDatabaseMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoveredDatabaseMetadata.deserializeBinaryFromReader);
      msg.setDatabase(value);
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
proto.prehog.v1alpha.ResourceCreateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.ResourceCreateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.ResourceCreateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.ResourceCreateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResourceType();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getResourceOrigin();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getCloudProvider();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getDatabase();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.prehog.v1alpha.DiscoveredDatabaseMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional string resource_type = 1;
 * @return {string}
 */
proto.prehog.v1alpha.ResourceCreateEvent.prototype.getResourceType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.ResourceCreateEvent} returns this
 */
proto.prehog.v1alpha.ResourceCreateEvent.prototype.setResourceType = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string resource_origin = 2;
 * @return {string}
 */
proto.prehog.v1alpha.ResourceCreateEvent.prototype.getResourceOrigin = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.ResourceCreateEvent} returns this
 */
proto.prehog.v1alpha.ResourceCreateEvent.prototype.setResourceOrigin = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string cloud_provider = 3;
 * @return {string}
 */
proto.prehog.v1alpha.ResourceCreateEvent.prototype.getCloudProvider = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.ResourceCreateEvent} returns this
 */
proto.prehog.v1alpha.ResourceCreateEvent.prototype.setCloudProvider = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional DiscoveredDatabaseMetadata database = 4;
 * @return {?proto.prehog.v1alpha.DiscoveredDatabaseMetadata}
 */
proto.prehog.v1alpha.ResourceCreateEvent.prototype.getDatabase = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoveredDatabaseMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoveredDatabaseMetadata, 4));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoveredDatabaseMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.ResourceCreateEvent} returns this
*/
proto.prehog.v1alpha.ResourceCreateEvent.prototype.setDatabase = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.ResourceCreateEvent} returns this
 */
proto.prehog.v1alpha.ResourceCreateEvent.prototype.clearDatabase = function() {
  return this.setDatabase(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.ResourceCreateEvent.prototype.hasDatabase = function() {
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
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.DiscoveredDatabaseMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.DiscoveredDatabaseMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    dbType: jspb.Message.getFieldWithDefault(msg, 1, ""),
    dbProtocol: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.prehog.v1alpha.DiscoveredDatabaseMetadata}
 */
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.DiscoveredDatabaseMetadata;
  return proto.prehog.v1alpha.DiscoveredDatabaseMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.DiscoveredDatabaseMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.DiscoveredDatabaseMetadata}
 */
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDbType(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDbProtocol(value);
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
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.DiscoveredDatabaseMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.DiscoveredDatabaseMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDbType();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDbProtocol();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string db_type = 1;
 * @return {string}
 */
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.prototype.getDbType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DiscoveredDatabaseMetadata} returns this
 */
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.prototype.setDbType = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string db_protocol = 2;
 * @return {string}
 */
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.prototype.getDbProtocol = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DiscoveredDatabaseMetadata} returns this
 */
proto.prehog.v1alpha.DiscoveredDatabaseMetadata.prototype.setDbProtocol = function(value) {
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
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.ResourceHeartbeatEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.ResourceHeartbeatEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    resourceName: msg.getResourceName_asB64(),
    resourceKind: jspb.Message.getFieldWithDefault(msg, 2, 0),
    pb_static: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
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
 * @return {!proto.prehog.v1alpha.ResourceHeartbeatEvent}
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.ResourceHeartbeatEvent;
  return proto.prehog.v1alpha.ResourceHeartbeatEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.ResourceHeartbeatEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.ResourceHeartbeatEvent}
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setResourceName(value);
      break;
    case 2:
      var value = /** @type {!proto.prehog.v1alpha.ResourceKind} */ (reader.readEnum());
      msg.setResourceKind(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setStatic(value);
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
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.ResourceHeartbeatEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.ResourceHeartbeatEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResourceName_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      1,
      f
    );
  }
  f = message.getResourceKind();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getStatic();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional bytes resource_name = 1;
 * @return {!(string|Uint8Array)}
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.getResourceName = function() {
  return /** @type {!(string|Uint8Array)} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * optional bytes resource_name = 1;
 * This is a type-conversion wrapper around `getResourceName()`
 * @return {string}
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.getResourceName_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getResourceName()));
};


/**
 * optional bytes resource_name = 1;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getResourceName()`
 * @return {!Uint8Array}
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.getResourceName_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getResourceName()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.prehog.v1alpha.ResourceHeartbeatEvent} returns this
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.setResourceName = function(value) {
  return jspb.Message.setProto3BytesField(this, 1, value);
};


/**
 * optional ResourceKind resource_kind = 2;
 * @return {!proto.prehog.v1alpha.ResourceKind}
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.getResourceKind = function() {
  return /** @type {!proto.prehog.v1alpha.ResourceKind} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.prehog.v1alpha.ResourceKind} value
 * @return {!proto.prehog.v1alpha.ResourceHeartbeatEvent} returns this
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.setResourceKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional bool static = 3;
 * @return {boolean}
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.getStatic = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.ResourceHeartbeatEvent} returns this
 */
proto.prehog.v1alpha.ResourceHeartbeatEvent.prototype.setStatic = function(value) {
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
proto.prehog.v1alpha.SessionStartEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SessionStartEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SessionStartEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SessionStartEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    sessionType: jspb.Message.getFieldWithDefault(msg, 2, ""),
    database: (f = msg.getDatabase()) && proto.prehog.v1alpha.SessionStartDatabaseMetadata.toObject(includeInstance, f),
    desktop: (f = msg.getDesktop()) && proto.prehog.v1alpha.SessionStartDesktopMetadata.toObject(includeInstance, f),
    userKind: jspb.Message.getFieldWithDefault(msg, 5, 0)
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
 * @return {!proto.prehog.v1alpha.SessionStartEvent}
 */
proto.prehog.v1alpha.SessionStartEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SessionStartEvent;
  return proto.prehog.v1alpha.SessionStartEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SessionStartEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SessionStartEvent}
 */
proto.prehog.v1alpha.SessionStartEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setSessionType(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.SessionStartDatabaseMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.SessionStartDatabaseMetadata.deserializeBinaryFromReader);
      msg.setDatabase(value);
      break;
    case 4:
      var value = new proto.prehog.v1alpha.SessionStartDesktopMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.SessionStartDesktopMetadata.deserializeBinaryFromReader);
      msg.setDesktop(value);
      break;
    case 5:
      var value = /** @type {!proto.prehog.v1alpha.UserKind} */ (reader.readEnum());
      msg.setUserKind(value);
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
proto.prehog.v1alpha.SessionStartEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SessionStartEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SessionStartEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SessionStartEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getSessionType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDatabase();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.SessionStartDatabaseMetadata.serializeBinaryToWriter
    );
  }
  f = message.getDesktop();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.prehog.v1alpha.SessionStartDesktopMetadata.serializeBinaryToWriter
    );
  }
  f = message.getUserKind();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SessionStartEvent} returns this
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string session_type = 2;
 * @return {string}
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.getSessionType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SessionStartEvent} returns this
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.setSessionType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional SessionStartDatabaseMetadata database = 3;
 * @return {?proto.prehog.v1alpha.SessionStartDatabaseMetadata}
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.getDatabase = function() {
  return /** @type{?proto.prehog.v1alpha.SessionStartDatabaseMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.SessionStartDatabaseMetadata, 3));
};


/**
 * @param {?proto.prehog.v1alpha.SessionStartDatabaseMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.SessionStartEvent} returns this
*/
proto.prehog.v1alpha.SessionStartEvent.prototype.setDatabase = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SessionStartEvent} returns this
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.clearDatabase = function() {
  return this.setDatabase(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.hasDatabase = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional SessionStartDesktopMetadata desktop = 4;
 * @return {?proto.prehog.v1alpha.SessionStartDesktopMetadata}
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.getDesktop = function() {
  return /** @type{?proto.prehog.v1alpha.SessionStartDesktopMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.SessionStartDesktopMetadata, 4));
};


/**
 * @param {?proto.prehog.v1alpha.SessionStartDesktopMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.SessionStartEvent} returns this
*/
proto.prehog.v1alpha.SessionStartEvent.prototype.setDesktop = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SessionStartEvent} returns this
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.clearDesktop = function() {
  return this.setDesktop(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.hasDesktop = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional UserKind user_kind = 5;
 * @return {!proto.prehog.v1alpha.UserKind}
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.getUserKind = function() {
  return /** @type {!proto.prehog.v1alpha.UserKind} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.prehog.v1alpha.UserKind} value
 * @return {!proto.prehog.v1alpha.SessionStartEvent} returns this
 */
proto.prehog.v1alpha.SessionStartEvent.prototype.setUserKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
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
proto.prehog.v1alpha.SessionStartDatabaseMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SessionStartDatabaseMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SessionStartDatabaseMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    dbType: jspb.Message.getFieldWithDefault(msg, 1, ""),
    dbProtocol: jspb.Message.getFieldWithDefault(msg, 2, ""),
    dbOrigin: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.prehog.v1alpha.SessionStartDatabaseMetadata}
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SessionStartDatabaseMetadata;
  return proto.prehog.v1alpha.SessionStartDatabaseMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SessionStartDatabaseMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SessionStartDatabaseMetadata}
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDbType(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setDbProtocol(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDbOrigin(value);
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
proto.prehog.v1alpha.SessionStartDatabaseMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SessionStartDatabaseMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SessionStartDatabaseMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDbType();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDbProtocol();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDbOrigin();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string db_type = 1;
 * @return {string}
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.prototype.getDbType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SessionStartDatabaseMetadata} returns this
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.prototype.setDbType = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string db_protocol = 2;
 * @return {string}
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.prototype.getDbProtocol = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SessionStartDatabaseMetadata} returns this
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.prototype.setDbProtocol = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string db_origin = 3;
 * @return {string}
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.prototype.getDbOrigin = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SessionStartDatabaseMetadata} returns this
 */
proto.prehog.v1alpha.SessionStartDatabaseMetadata.prototype.setDbOrigin = function(value) {
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
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SessionStartDesktopMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SessionStartDesktopMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    desktopType: jspb.Message.getFieldWithDefault(msg, 1, ""),
    origin: jspb.Message.getFieldWithDefault(msg, 2, ""),
    windowsDomain: jspb.Message.getFieldWithDefault(msg, 3, ""),
    allowUserCreation: jspb.Message.getBooleanFieldWithDefault(msg, 4, false)
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
 * @return {!proto.prehog.v1alpha.SessionStartDesktopMetadata}
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SessionStartDesktopMetadata;
  return proto.prehog.v1alpha.SessionStartDesktopMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SessionStartDesktopMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SessionStartDesktopMetadata}
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDesktopType(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setOrigin(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setWindowsDomain(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setAllowUserCreation(value);
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
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SessionStartDesktopMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SessionStartDesktopMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDesktopType();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOrigin();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getWindowsDomain();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getAllowUserCreation();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
};


/**
 * optional string desktop_type = 1;
 * @return {string}
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.getDesktopType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SessionStartDesktopMetadata} returns this
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.setDesktopType = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string origin = 2;
 * @return {string}
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.getOrigin = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SessionStartDesktopMetadata} returns this
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.setOrigin = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string windows_domain = 3;
 * @return {string}
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.getWindowsDomain = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SessionStartDesktopMetadata} returns this
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.setWindowsDomain = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional bool allow_user_creation = 4;
 * @return {boolean}
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.getAllowUserCreation = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.SessionStartDesktopMetadata} returns this
 */
proto.prehog.v1alpha.SessionStartDesktopMetadata.prototype.setAllowUserCreation = function(value) {
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
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UserCertificateIssuedEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UserCertificateIssuedEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    ttl: (f = msg.getTtl()) && google_protobuf_duration_pb.Duration.toObject(includeInstance, f),
    isBot: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
    usageDatabase: jspb.Message.getBooleanFieldWithDefault(msg, 4, false),
    usageApp: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
    usageKubernetes: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
    usageDesktop: jspb.Message.getBooleanFieldWithDefault(msg, 7, false),
    privateKeyPolicy: jspb.Message.getFieldWithDefault(msg, 8, "")
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
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UserCertificateIssuedEvent;
  return proto.prehog.v1alpha.UserCertificateIssuedEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UserCertificateIssuedEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = new google_protobuf_duration_pb.Duration;
      reader.readMessage(value,google_protobuf_duration_pb.Duration.deserializeBinaryFromReader);
      msg.setTtl(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIsBot(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setUsageDatabase(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setUsageApp(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setUsageKubernetes(value);
      break;
    case 7:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setUsageDesktop(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.setPrivateKeyPolicy(value);
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
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UserCertificateIssuedEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UserCertificateIssuedEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTtl();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_protobuf_duration_pb.Duration.serializeBinaryToWriter
    );
  }
  f = message.getIsBot();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getUsageDatabase();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getUsageApp();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getUsageKubernetes();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getUsageDesktop();
  if (f) {
    writer.writeBool(
      7,
      f
    );
  }
  f = message.getPrivateKeyPolicy();
  if (f.length > 0) {
    writer.writeString(
      8,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent} returns this
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional google.protobuf.Duration ttl = 2;
 * @return {?proto.google.protobuf.Duration}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.getTtl = function() {
  return /** @type{?proto.google.protobuf.Duration} */ (
    jspb.Message.getWrapperField(this, google_protobuf_duration_pb.Duration, 2));
};


/**
 * @param {?proto.google.protobuf.Duration|undefined} value
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent} returns this
*/
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.setTtl = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent} returns this
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.clearTtl = function() {
  return this.setTtl(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.hasTtl = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool is_bot = 3;
 * @return {boolean}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.getIsBot = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent} returns this
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.setIsBot = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional bool usage_database = 4;
 * @return {boolean}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.getUsageDatabase = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent} returns this
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.setUsageDatabase = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional bool usage_app = 5;
 * @return {boolean}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.getUsageApp = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent} returns this
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.setUsageApp = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional bool usage_kubernetes = 6;
 * @return {boolean}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.getUsageKubernetes = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent} returns this
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.setUsageKubernetes = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional bool usage_desktop = 7;
 * @return {boolean}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.getUsageDesktop = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 7, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent} returns this
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.setUsageDesktop = function(value) {
  return jspb.Message.setProto3BooleanField(this, 7, value);
};


/**
 * optional string private_key_policy = 8;
 * @return {string}
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.getPrivateKeyPolicy = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 8, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UserCertificateIssuedEvent} returns this
 */
proto.prehog.v1alpha.UserCertificateIssuedEvent.prototype.setPrivateKeyPolicy = function(value) {
  return jspb.Message.setProto3StringField(this, 8, value);
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
proto.prehog.v1alpha.UIBannerClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIBannerClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIBannerClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIBannerClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    alert: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.prehog.v1alpha.UIBannerClickEvent}
 */
proto.prehog.v1alpha.UIBannerClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIBannerClickEvent;
  return proto.prehog.v1alpha.UIBannerClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIBannerClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIBannerClickEvent}
 */
proto.prehog.v1alpha.UIBannerClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAlert(value);
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
proto.prehog.v1alpha.UIBannerClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIBannerClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIBannerClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIBannerClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAlert();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIBannerClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIBannerClickEvent} returns this
 */
proto.prehog.v1alpha.UIBannerClickEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string alert = 2;
 * @return {string}
 */
proto.prehog.v1alpha.UIBannerClickEvent.prototype.getAlert = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIBannerClickEvent} returns this
 */
proto.prehog.v1alpha.UIBannerClickEvent.prototype.setAlert = function(value) {
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
proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent}
 */
proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent;
  return proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent}
 */
proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent} returns this
 */
proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent}
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent;
  return proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent}
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent} returns this
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent}
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent;
  return proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent}
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent} returns this
 */
proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent}
 */
proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent;
  return proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent}
 */
proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent} returns this
 */
proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    mfaType: jspb.Message.getFieldWithDefault(msg, 2, ""),
    loginFlow: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent}
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent;
  return proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent}
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setMfaType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setLoginFlow(value);
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
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMfaType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getLoginFlow();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent} returns this
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string mfa_type = 2;
 * @return {string}
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.prototype.getMfaType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent} returns this
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.prototype.setMfaType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string login_flow = 3;
 * @return {string}
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.prototype.getLoginFlow = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent} returns this
 */
proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.prototype.setLoginFlow = function(value) {
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
proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent}
 */
proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent;
  return proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent}
 */
proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent} returns this
 */
proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent}
 */
proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent;
  return proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent}
 */
proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent} returns this
 */
proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent}
 */
proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent;
  return proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent}
 */
proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent} returns this
 */
proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent}
 */
proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent;
  return proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent}
 */
proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent} returns this
 */
proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.DiscoverMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.DiscoverMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.DiscoverMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoverMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    userName: jspb.Message.getFieldWithDefault(msg, 2, ""),
    sso: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
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
 * @return {!proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.DiscoverMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.DiscoverMetadata;
  return proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.DiscoverMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
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
proto.prehog.v1alpha.DiscoverMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.DiscoverMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getSso();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.prehog.v1alpha.DiscoverMetadata.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DiscoverMetadata} returns this
 */
proto.prehog.v1alpha.DiscoverMetadata.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string user_name = 2;
 * @return {string}
 */
proto.prehog.v1alpha.DiscoverMetadata.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DiscoverMetadata} returns this
 */
proto.prehog.v1alpha.DiscoverMetadata.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional bool sso = 3;
 * @return {boolean}
 */
proto.prehog.v1alpha.DiscoverMetadata.prototype.getSso = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.DiscoverMetadata} returns this
 */
proto.prehog.v1alpha.DiscoverMetadata.prototype.setSso = function(value) {
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
proto.prehog.v1alpha.DiscoverResourceMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.DiscoverResourceMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoverResourceMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    resource: jspb.Message.getFieldWithDefault(msg, 1, 0)
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
 * @return {!proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.DiscoverResourceMetadata;
  return proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.DiscoverResourceMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.prehog.v1alpha.DiscoverResource} */ (reader.readEnum());
      msg.setResource(value);
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
proto.prehog.v1alpha.DiscoverResourceMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.DiscoverResourceMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResource();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
};


/**
 * optional DiscoverResource resource = 1;
 * @return {!proto.prehog.v1alpha.DiscoverResource}
 */
proto.prehog.v1alpha.DiscoverResourceMetadata.prototype.getResource = function() {
  return /** @type {!proto.prehog.v1alpha.DiscoverResource} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.prehog.v1alpha.DiscoverResource} value
 * @return {!proto.prehog.v1alpha.DiscoverResourceMetadata} returns this
 */
proto.prehog.v1alpha.DiscoverResourceMetadata.prototype.setResource = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
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
proto.prehog.v1alpha.DiscoverStepStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.DiscoverStepStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.DiscoverStepStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoverStepStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
    status: jspb.Message.getFieldWithDefault(msg, 1, 0),
    error: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.DiscoverStepStatus;
  return proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.DiscoverStepStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.prehog.v1alpha.DiscoverStatus} */ (reader.readEnum());
      msg.setStatus(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setError(value);
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
proto.prehog.v1alpha.DiscoverStepStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.DiscoverStepStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
  f = message.getError();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional DiscoverStatus status = 1;
 * @return {!proto.prehog.v1alpha.DiscoverStatus}
 */
proto.prehog.v1alpha.DiscoverStepStatus.prototype.getStatus = function() {
  return /** @type {!proto.prehog.v1alpha.DiscoverStatus} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.prehog.v1alpha.DiscoverStatus} value
 * @return {!proto.prehog.v1alpha.DiscoverStepStatus} returns this
 */
proto.prehog.v1alpha.DiscoverStepStatus.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};


/**
 * optional string error = 2;
 * @return {string}
 */
proto.prehog.v1alpha.DiscoverStepStatus.prototype.getError = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DiscoverStepStatus} returns this
 */
proto.prehog.v1alpha.DiscoverStepStatus.prototype.setError = function(value) {
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
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverStartedEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverStartedEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverStartedEvent}
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverStartedEvent;
  return proto.prehog.v1alpha.UIDiscoverStartedEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverStartedEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverStartedEvent}
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverStartedEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverStartedEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverStartedEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverStartedEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverStepStatus status = 2;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverStartedEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverStartedEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverStartedEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent}
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent;
  return proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent}
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent}
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent;
  return proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent}
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f),
    selectedResourcesCount: jspb.Message.getFieldWithDefault(msg, 4, 0)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent;
  return proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setSelectedResourcesCount(value);
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
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
  f = message.getSelectedResourcesCount();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.hasStatus = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional int64 selected_resources_count = 4;
 * @return {number}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.getSelectedResourcesCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.prototype.setSelectedResourcesCount = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
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
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f),
    deployMethod: jspb.Message.getFieldWithDefault(msg, 4, 0),
    deployType: jspb.Message.getFieldWithDefault(msg, 5, 0)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverDeployServiceEvent;
  return proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
      break;
    case 4:
      var value = /** @type {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployMethod} */ (reader.readEnum());
      msg.setDeployMethod(value);
      break;
    case 5:
      var value = /** @type {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployType} */ (reader.readEnum());
      msg.setDeployType(value);
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
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
  f = message.getDeployMethod();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
  f = message.getDeployType();
  if (f !== 0.0) {
    writer.writeEnum(
      5,
      f
    );
  }
};


/**
 * @enum {number}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployMethod = {
  DEPLOY_METHOD_UNSPECIFIED: 0,
  DEPLOY_METHOD_AUTO: 1,
  DEPLOY_METHOD_MANUAL: 2
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployType = {
  DEPLOY_TYPE_UNSPECIFIED: 0,
  DEPLOY_TYPE_INSTALL_SCRIPT: 1,
  DEPLOY_TYPE_AMAZON_ECS: 2
};

/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.hasStatus = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional DeployMethod deploy_method = 4;
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployMethod}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.getDeployMethod = function() {
  return /** @type {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployMethod} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployMethod} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.setDeployMethod = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};


/**
 * optional DeployType deploy_type = 5;
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployType}
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.getDeployType = function() {
  return /** @type {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployType} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.DeployType} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.prototype.setDeployType = function(value) {
  return jspb.Message.setProto3EnumField(this, 5, value);
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
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent;
  return proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent;
  return proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent;
  return proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent;
  return proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f),
    resourcesCount: jspb.Message.getFieldWithDefault(msg, 4, 0)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent}
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent;
  return proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent}
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setResourcesCount(value);
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
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
  f = message.getResourcesCount();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.hasStatus = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional int64 resources_count = 4;
 * @return {number}
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.getResourcesCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.prototype.setResourcesCount = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
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
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent}
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent;
  return proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent}
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent}
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverDeployEICEEvent;
  return proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent}
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent}
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverCreateNodeEvent;
  return proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent}
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent;
  return proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent}
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent;
  return proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent}
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent}
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverTestConnectionEvent;
  return proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent}
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIDiscoverCompletedEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIDiscoverCompletedEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.DiscoverMetadata.toObject(includeInstance, f),
    resource: (f = msg.getResource()) && proto.prehog.v1alpha.DiscoverResourceMetadata.toObject(includeInstance, f),
    status: (f = msg.getStatus()) && proto.prehog.v1alpha.DiscoverStepStatus.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIDiscoverCompletedEvent}
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIDiscoverCompletedEvent;
  return proto.prehog.v1alpha.UIDiscoverCompletedEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIDiscoverCompletedEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIDiscoverCompletedEvent}
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.DiscoverMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.DiscoverResourceMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverResourceMetadata.deserializeBinaryFromReader);
      msg.setResource(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.DiscoverStepStatus;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoverStepStatus.deserializeBinaryFromReader);
      msg.setStatus(value);
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
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIDiscoverCompletedEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIDiscoverCompletedEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.DiscoverMetadata.serializeBinaryToWriter
    );
  }
  f = message.getResource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.DiscoverResourceMetadata.serializeBinaryToWriter
    );
  }
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.DiscoverStepStatus.serializeBinaryToWriter
    );
  }
};


/**
 * optional DiscoverMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.DiscoverMetadata}
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverCompletedEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverCompletedEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional DiscoverResourceMetadata resource = 2;
 * @return {?proto.prehog.v1alpha.DiscoverResourceMetadata}
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.getResource = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverResourceMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverResourceMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverResourceMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverCompletedEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.setResource = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverCompletedEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.clearResource = function() {
  return this.setResource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.hasResource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional DiscoverStepStatus status = 3;
 * @return {?proto.prehog.v1alpha.DiscoverStepStatus}
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.getStatus = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoverStepStatus} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoverStepStatus, 3));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoverStepStatus|undefined} value
 * @return {!proto.prehog.v1alpha.UIDiscoverCompletedEvent} returns this
*/
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIDiscoverCompletedEvent} returns this
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIDiscoverCompletedEvent.prototype.hasStatus = function() {
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
proto.prehog.v1alpha.RoleCreateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.RoleCreateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.RoleCreateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.RoleCreateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    roleName: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.prehog.v1alpha.RoleCreateEvent}
 */
proto.prehog.v1alpha.RoleCreateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.RoleCreateEvent;
  return proto.prehog.v1alpha.RoleCreateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.RoleCreateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.RoleCreateEvent}
 */
proto.prehog.v1alpha.RoleCreateEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoleName(value);
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
proto.prehog.v1alpha.RoleCreateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.RoleCreateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.RoleCreateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.RoleCreateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getRoleName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.RoleCreateEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.RoleCreateEvent} returns this
 */
proto.prehog.v1alpha.RoleCreateEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string role_name = 2;
 * @return {string}
 */
proto.prehog.v1alpha.RoleCreateEvent.prototype.getRoleName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.RoleCreateEvent} returns this
 */
proto.prehog.v1alpha.RoleCreateEvent.prototype.setRoleName = function(value) {
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
proto.prehog.v1alpha.BotCreateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.BotCreateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.BotCreateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.BotCreateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    botUserName: jspb.Message.getFieldWithDefault(msg, 2, ""),
    roleName: jspb.Message.getFieldWithDefault(msg, 3, ""),
    roleCount: jspb.Message.getFieldWithDefault(msg, 4, 0),
    joinMethod: jspb.Message.getFieldWithDefault(msg, 5, ""),
    botName: jspb.Message.getFieldWithDefault(msg, 6, "")
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
 * @return {!proto.prehog.v1alpha.BotCreateEvent}
 */
proto.prehog.v1alpha.BotCreateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.BotCreateEvent;
  return proto.prehog.v1alpha.BotCreateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.BotCreateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.BotCreateEvent}
 */
proto.prehog.v1alpha.BotCreateEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setBotUserName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setRoleName(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setRoleCount(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setJoinMethod(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setBotName(value);
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
proto.prehog.v1alpha.BotCreateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.BotCreateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.BotCreateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.BotCreateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getBotUserName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getRoleName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getRoleCount();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getJoinMethod();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getBotName();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.BotCreateEvent} returns this
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string bot_user_name = 2;
 * @return {string}
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.getBotUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.BotCreateEvent} returns this
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.setBotUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string role_name = 3;
 * @return {string}
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.getRoleName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.BotCreateEvent} returns this
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.setRoleName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional int64 role_count = 4;
 * @return {number}
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.getRoleCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.BotCreateEvent} returns this
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.setRoleCount = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional string join_method = 5;
 * @return {string}
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.getJoinMethod = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.BotCreateEvent} returns this
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.setJoinMethod = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string bot_name = 6;
 * @return {string}
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.getBotName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.BotCreateEvent} returns this
 */
proto.prehog.v1alpha.BotCreateEvent.prototype.setBotName = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
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
proto.prehog.v1alpha.BotJoinEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.BotJoinEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.BotJoinEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.BotJoinEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    botName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    joinMethod: jspb.Message.getFieldWithDefault(msg, 2, ""),
    joinTokenName: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.prehog.v1alpha.BotJoinEvent}
 */
proto.prehog.v1alpha.BotJoinEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.BotJoinEvent;
  return proto.prehog.v1alpha.BotJoinEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.BotJoinEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.BotJoinEvent}
 */
proto.prehog.v1alpha.BotJoinEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setBotName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setJoinMethod(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setJoinTokenName(value);
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
proto.prehog.v1alpha.BotJoinEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.BotJoinEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.BotJoinEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.BotJoinEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getBotName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getJoinMethod();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getJoinTokenName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string bot_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.BotJoinEvent.prototype.getBotName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.BotJoinEvent} returns this
 */
proto.prehog.v1alpha.BotJoinEvent.prototype.setBotName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string join_method = 2;
 * @return {string}
 */
proto.prehog.v1alpha.BotJoinEvent.prototype.getJoinMethod = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.BotJoinEvent} returns this
 */
proto.prehog.v1alpha.BotJoinEvent.prototype.setJoinMethod = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string join_token_name = 3;
 * @return {string}
 */
proto.prehog.v1alpha.BotJoinEvent.prototype.getJoinTokenName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.BotJoinEvent} returns this
 */
proto.prehog.v1alpha.BotJoinEvent.prototype.setJoinTokenName = function(value) {
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
proto.prehog.v1alpha.UICreateNewRoleClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UICreateNewRoleClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UICreateNewRoleClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICreateNewRoleClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UICreateNewRoleClickEvent}
 */
proto.prehog.v1alpha.UICreateNewRoleClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UICreateNewRoleClickEvent;
  return proto.prehog.v1alpha.UICreateNewRoleClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UICreateNewRoleClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UICreateNewRoleClickEvent}
 */
proto.prehog.v1alpha.UICreateNewRoleClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UICreateNewRoleClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UICreateNewRoleClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UICreateNewRoleClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICreateNewRoleClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UICreateNewRoleClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UICreateNewRoleClickEvent} returns this
 */
proto.prehog.v1alpha.UICreateNewRoleClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent}
 */
proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent;
  return proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent}
 */
proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent} returns this
 */
proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent}
 */
proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent;
  return proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent}
 */
proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent} returns this
 */
proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent}
 */
proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent;
  return proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent}
 */
proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent} returns this
 */
proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UICallToActionClickEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UICallToActionClickEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UICallToActionClickEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICallToActionClickEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    cta: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.prehog.v1alpha.UICallToActionClickEvent}
 */
proto.prehog.v1alpha.UICallToActionClickEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UICallToActionClickEvent;
  return proto.prehog.v1alpha.UICallToActionClickEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UICallToActionClickEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UICallToActionClickEvent}
 */
proto.prehog.v1alpha.UICallToActionClickEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {!proto.prehog.v1alpha.CTA} */ (reader.readEnum());
      msg.setCta(value);
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
proto.prehog.v1alpha.UICallToActionClickEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UICallToActionClickEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UICallToActionClickEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UICallToActionClickEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCta();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.UICallToActionClickEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.UICallToActionClickEvent} returns this
 */
proto.prehog.v1alpha.UICallToActionClickEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional CTA cta = 2;
 * @return {!proto.prehog.v1alpha.CTA}
 */
proto.prehog.v1alpha.UICallToActionClickEvent.prototype.getCta = function() {
  return /** @type {!proto.prehog.v1alpha.CTA} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.prehog.v1alpha.CTA} value
 * @return {!proto.prehog.v1alpha.UICallToActionClickEvent} returns this
 */
proto.prehog.v1alpha.UICallToActionClickEvent.prototype.setCta = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
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
proto.prehog.v1alpha.KubeRequestEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.KubeRequestEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.KubeRequestEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.KubeRequestEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    userKind: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.prehog.v1alpha.KubeRequestEvent}
 */
proto.prehog.v1alpha.KubeRequestEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.KubeRequestEvent;
  return proto.prehog.v1alpha.KubeRequestEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.KubeRequestEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.KubeRequestEvent}
 */
proto.prehog.v1alpha.KubeRequestEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {!proto.prehog.v1alpha.UserKind} */ (reader.readEnum());
      msg.setUserKind(value);
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
proto.prehog.v1alpha.KubeRequestEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.KubeRequestEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.KubeRequestEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.KubeRequestEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUserKind();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.KubeRequestEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.KubeRequestEvent} returns this
 */
proto.prehog.v1alpha.KubeRequestEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional UserKind user_kind = 2;
 * @return {!proto.prehog.v1alpha.UserKind}
 */
proto.prehog.v1alpha.KubeRequestEvent.prototype.getUserKind = function() {
  return /** @type {!proto.prehog.v1alpha.UserKind} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.prehog.v1alpha.UserKind} value
 * @return {!proto.prehog.v1alpha.KubeRequestEvent} returns this
 */
proto.prehog.v1alpha.KubeRequestEvent.prototype.setUserKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
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
proto.prehog.v1alpha.SFTPEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SFTPEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SFTPEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SFTPEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    action: jspb.Message.getFieldWithDefault(msg, 2, 0),
    userKind: jspb.Message.getFieldWithDefault(msg, 3, 0)
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
 * @return {!proto.prehog.v1alpha.SFTPEvent}
 */
proto.prehog.v1alpha.SFTPEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SFTPEvent;
  return proto.prehog.v1alpha.SFTPEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SFTPEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SFTPEvent}
 */
proto.prehog.v1alpha.SFTPEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setAction(value);
      break;
    case 3:
      var value = /** @type {!proto.prehog.v1alpha.UserKind} */ (reader.readEnum());
      msg.setUserKind(value);
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
proto.prehog.v1alpha.SFTPEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SFTPEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SFTPEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SFTPEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAction();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getUserKind();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.SFTPEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SFTPEvent} returns this
 */
proto.prehog.v1alpha.SFTPEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 action = 2;
 * @return {number}
 */
proto.prehog.v1alpha.SFTPEvent.prototype.getAction = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.SFTPEvent} returns this
 */
proto.prehog.v1alpha.SFTPEvent.prototype.setAction = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional UserKind user_kind = 3;
 * @return {!proto.prehog.v1alpha.UserKind}
 */
proto.prehog.v1alpha.SFTPEvent.prototype.getUserKind = function() {
  return /** @type {!proto.prehog.v1alpha.UserKind} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.prehog.v1alpha.UserKind} value
 * @return {!proto.prehog.v1alpha.SFTPEvent} returns this
 */
proto.prehog.v1alpha.SFTPEvent.prototype.setUserKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.prehog.v1alpha.AgentMetadataEvent.repeatedFields_ = [3,8];



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
proto.prehog.v1alpha.AgentMetadataEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AgentMetadataEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AgentMetadataEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AgentMetadataEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    version: jspb.Message.getFieldWithDefault(msg, 1, ""),
    hostId: jspb.Message.getFieldWithDefault(msg, 2, ""),
    servicesList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f,
    os: jspb.Message.getFieldWithDefault(msg, 4, ""),
    osVersion: jspb.Message.getFieldWithDefault(msg, 5, ""),
    hostArchitecture: jspb.Message.getFieldWithDefault(msg, 6, ""),
    glibcVersion: jspb.Message.getFieldWithDefault(msg, 7, ""),
    installMethodsList: (f = jspb.Message.getRepeatedField(msg, 8)) == null ? undefined : f,
    containerRuntime: jspb.Message.getFieldWithDefault(msg, 9, ""),
    containerOrchestrator: jspb.Message.getFieldWithDefault(msg, 10, ""),
    cloudEnvironment: jspb.Message.getFieldWithDefault(msg, 11, ""),
    externalUpgrader: jspb.Message.getFieldWithDefault(msg, 12, "")
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
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent}
 */
proto.prehog.v1alpha.AgentMetadataEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AgentMetadataEvent;
  return proto.prehog.v1alpha.AgentMetadataEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AgentMetadataEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent}
 */
proto.prehog.v1alpha.AgentMetadataEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setVersion(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setHostId(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.addServices(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setOs(value);
      break;
    case 5:
      var value = /** @type {string} */ (reader.readString());
      msg.setOsVersion(value);
      break;
    case 6:
      var value = /** @type {string} */ (reader.readString());
      msg.setHostArchitecture(value);
      break;
    case 7:
      var value = /** @type {string} */ (reader.readString());
      msg.setGlibcVersion(value);
      break;
    case 8:
      var value = /** @type {string} */ (reader.readString());
      msg.addInstallMethods(value);
      break;
    case 9:
      var value = /** @type {string} */ (reader.readString());
      msg.setContainerRuntime(value);
      break;
    case 10:
      var value = /** @type {string} */ (reader.readString());
      msg.setContainerOrchestrator(value);
      break;
    case 11:
      var value = /** @type {string} */ (reader.readString());
      msg.setCloudEnvironment(value);
      break;
    case 12:
      var value = /** @type {string} */ (reader.readString());
      msg.setExternalUpgrader(value);
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
proto.prehog.v1alpha.AgentMetadataEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AgentMetadataEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AgentMetadataEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AgentMetadataEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getVersion();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getHostId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getServicesList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      3,
      f
    );
  }
  f = message.getOs();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
  f = message.getOsVersion();
  if (f.length > 0) {
    writer.writeString(
      5,
      f
    );
  }
  f = message.getHostArchitecture();
  if (f.length > 0) {
    writer.writeString(
      6,
      f
    );
  }
  f = message.getGlibcVersion();
  if (f.length > 0) {
    writer.writeString(
      7,
      f
    );
  }
  f = message.getInstallMethodsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      8,
      f
    );
  }
  f = message.getContainerRuntime();
  if (f.length > 0) {
    writer.writeString(
      9,
      f
    );
  }
  f = message.getContainerOrchestrator();
  if (f.length > 0) {
    writer.writeString(
      10,
      f
    );
  }
  f = message.getCloudEnvironment();
  if (f.length > 0) {
    writer.writeString(
      11,
      f
    );
  }
  f = message.getExternalUpgrader();
  if (f.length > 0) {
    writer.writeString(
      12,
      f
    );
  }
};


/**
 * optional string version = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string host_id = 2;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getHostId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setHostId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated string services = 3;
 * @return {!Array<string>}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getServicesList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setServicesList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.addServices = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.clearServicesList = function() {
  return this.setServicesList([]);
};


/**
 * optional string os = 4;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getOs = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setOs = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};


/**
 * optional string os_version = 5;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getOsVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 5, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setOsVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 5, value);
};


/**
 * optional string host_architecture = 6;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getHostArchitecture = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 6, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setHostArchitecture = function(value) {
  return jspb.Message.setProto3StringField(this, 6, value);
};


/**
 * optional string glibc_version = 7;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getGlibcVersion = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 7, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setGlibcVersion = function(value) {
  return jspb.Message.setProto3StringField(this, 7, value);
};


/**
 * repeated string install_methods = 8;
 * @return {!Array<string>}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getInstallMethodsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 8));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setInstallMethodsList = function(value) {
  return jspb.Message.setField(this, 8, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.addInstallMethods = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 8, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.clearInstallMethodsList = function() {
  return this.setInstallMethodsList([]);
};


/**
 * optional string container_runtime = 9;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getContainerRuntime = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 9, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setContainerRuntime = function(value) {
  return jspb.Message.setProto3StringField(this, 9, value);
};


/**
 * optional string container_orchestrator = 10;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getContainerOrchestrator = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 10, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setContainerOrchestrator = function(value) {
  return jspb.Message.setProto3StringField(this, 10, value);
};


/**
 * optional string cloud_environment = 11;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getCloudEnvironment = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 11, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setCloudEnvironment = function(value) {
  return jspb.Message.setProto3StringField(this, 11, value);
};


/**
 * optional string external_upgrader = 12;
 * @return {string}
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.getExternalUpgrader = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 12, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AgentMetadataEvent} returns this
 */
proto.prehog.v1alpha.AgentMetadataEvent.prototype.setExternalUpgrader = function(value) {
  return jspb.Message.setProto3StringField(this, 12, value);
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
proto.prehog.v1alpha.AssistCompletionEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AssistCompletionEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AssistCompletionEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistCompletionEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    conversationId: jspb.Message.getFieldWithDefault(msg, 2, ""),
    totalTokens: jspb.Message.getFieldWithDefault(msg, 3, 0),
    promptTokens: jspb.Message.getFieldWithDefault(msg, 4, 0),
    completionTokens: jspb.Message.getFieldWithDefault(msg, 5, 0)
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
 * @return {!proto.prehog.v1alpha.AssistCompletionEvent}
 */
proto.prehog.v1alpha.AssistCompletionEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AssistCompletionEvent;
  return proto.prehog.v1alpha.AssistCompletionEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AssistCompletionEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AssistCompletionEvent}
 */
proto.prehog.v1alpha.AssistCompletionEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setConversationId(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTotalTokens(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setPromptTokens(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setCompletionTokens(value);
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
proto.prehog.v1alpha.AssistCompletionEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AssistCompletionEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AssistCompletionEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistCompletionEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getConversationId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getTotalTokens();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
  f = message.getPromptTokens();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getCompletionTokens();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistCompletionEvent} returns this
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string conversation_id = 2;
 * @return {string}
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.getConversationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistCompletionEvent} returns this
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.setConversationId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int64 total_tokens = 3;
 * @return {number}
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.getTotalTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistCompletionEvent} returns this
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.setTotalTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int64 prompt_tokens = 4;
 * @return {number}
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.getPromptTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistCompletionEvent} returns this
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.setPromptTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 completion_tokens = 5;
 * @return {number}
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.getCompletionTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistCompletionEvent} returns this
 */
proto.prehog.v1alpha.AssistCompletionEvent.prototype.setCompletionTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
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
proto.prehog.v1alpha.AssistExecutionEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AssistExecutionEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AssistExecutionEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistExecutionEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    conversationId: jspb.Message.getFieldWithDefault(msg, 2, ""),
    nodeCount: jspb.Message.getFieldWithDefault(msg, 3, 0),
    totalTokens: jspb.Message.getFieldWithDefault(msg, 4, 0),
    promptTokens: jspb.Message.getFieldWithDefault(msg, 5, 0),
    completionTokens: jspb.Message.getFieldWithDefault(msg, 6, 0)
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
 * @return {!proto.prehog.v1alpha.AssistExecutionEvent}
 */
proto.prehog.v1alpha.AssistExecutionEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AssistExecutionEvent;
  return proto.prehog.v1alpha.AssistExecutionEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AssistExecutionEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AssistExecutionEvent}
 */
proto.prehog.v1alpha.AssistExecutionEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setConversationId(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setNodeCount(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTotalTokens(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setPromptTokens(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setCompletionTokens(value);
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
proto.prehog.v1alpha.AssistExecutionEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AssistExecutionEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AssistExecutionEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistExecutionEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getConversationId();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getNodeCount();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
  f = message.getTotalTokens();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getPromptTokens();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getCompletionTokens();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistExecutionEvent} returns this
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string conversation_id = 2;
 * @return {string}
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.getConversationId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistExecutionEvent} returns this
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.setConversationId = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int64 node_count = 3;
 * @return {number}
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.getNodeCount = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistExecutionEvent} returns this
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.setNodeCount = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional int64 total_tokens = 4;
 * @return {number}
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.getTotalTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistExecutionEvent} returns this
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.setTotalTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 prompt_tokens = 5;
 * @return {number}
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.getPromptTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistExecutionEvent} returns this
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.setPromptTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional int64 completion_tokens = 6;
 * @return {number}
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.getCompletionTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistExecutionEvent} returns this
 */
proto.prehog.v1alpha.AssistExecutionEvent.prototype.setCompletionTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
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
proto.prehog.v1alpha.AssistNewConversationEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AssistNewConversationEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AssistNewConversationEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistNewConversationEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    category: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.prehog.v1alpha.AssistNewConversationEvent}
 */
proto.prehog.v1alpha.AssistNewConversationEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AssistNewConversationEvent;
  return proto.prehog.v1alpha.AssistNewConversationEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AssistNewConversationEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AssistNewConversationEvent}
 */
proto.prehog.v1alpha.AssistNewConversationEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setCategory(value);
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
proto.prehog.v1alpha.AssistNewConversationEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AssistNewConversationEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AssistNewConversationEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistNewConversationEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCategory();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AssistNewConversationEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistNewConversationEvent} returns this
 */
proto.prehog.v1alpha.AssistNewConversationEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string category = 2;
 * @return {string}
 */
proto.prehog.v1alpha.AssistNewConversationEvent.prototype.getCategory = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistNewConversationEvent} returns this
 */
proto.prehog.v1alpha.AssistNewConversationEvent.prototype.setCategory = function(value) {
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
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AssistAccessRequestEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AssistAccessRequestEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    resourceType: jspb.Message.getFieldWithDefault(msg, 2, ""),
    totalTokens: jspb.Message.getFieldWithDefault(msg, 4, 0),
    promptTokens: jspb.Message.getFieldWithDefault(msg, 5, 0),
    completionTokens: jspb.Message.getFieldWithDefault(msg, 6, 0)
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
 * @return {!proto.prehog.v1alpha.AssistAccessRequestEvent}
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AssistAccessRequestEvent;
  return proto.prehog.v1alpha.AssistAccessRequestEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AssistAccessRequestEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AssistAccessRequestEvent}
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceType(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTotalTokens(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setPromptTokens(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setCompletionTokens(value);
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
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AssistAccessRequestEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AssistAccessRequestEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getResourceType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getTotalTokens();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getPromptTokens();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getCompletionTokens();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistAccessRequestEvent} returns this
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string resource_type = 2;
 * @return {string}
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.getResourceType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistAccessRequestEvent} returns this
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.setResourceType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int64 total_tokens = 4;
 * @return {number}
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.getTotalTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistAccessRequestEvent} returns this
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.setTotalTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 prompt_tokens = 5;
 * @return {number}
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.getPromptTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistAccessRequestEvent} returns this
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.setPromptTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional int64 completion_tokens = 6;
 * @return {number}
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.getCompletionTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistAccessRequestEvent} returns this
 */
proto.prehog.v1alpha.AssistAccessRequestEvent.prototype.setCompletionTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
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
proto.prehog.v1alpha.AssistActionEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AssistActionEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AssistActionEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistActionEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    action: jspb.Message.getFieldWithDefault(msg, 2, ""),
    totalTokens: jspb.Message.getFieldWithDefault(msg, 4, 0),
    promptTokens: jspb.Message.getFieldWithDefault(msg, 5, 0),
    completionTokens: jspb.Message.getFieldWithDefault(msg, 6, 0)
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
 * @return {!proto.prehog.v1alpha.AssistActionEvent}
 */
proto.prehog.v1alpha.AssistActionEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AssistActionEvent;
  return proto.prehog.v1alpha.AssistActionEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AssistActionEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AssistActionEvent}
 */
proto.prehog.v1alpha.AssistActionEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setAction(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTotalTokens(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setPromptTokens(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setCompletionTokens(value);
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
proto.prehog.v1alpha.AssistActionEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AssistActionEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AssistActionEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AssistActionEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getAction();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getTotalTokens();
  if (f !== 0) {
    writer.writeInt64(
      4,
      f
    );
  }
  f = message.getPromptTokens();
  if (f !== 0) {
    writer.writeInt64(
      5,
      f
    );
  }
  f = message.getCompletionTokens();
  if (f !== 0) {
    writer.writeInt64(
      6,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistActionEvent} returns this
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string action = 2;
 * @return {string}
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.getAction = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AssistActionEvent} returns this
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.setAction = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int64 total_tokens = 4;
 * @return {number}
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.getTotalTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistActionEvent} returns this
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.setTotalTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 4, value);
};


/**
 * optional int64 prompt_tokens = 5;
 * @return {number}
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.getPromptTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 5, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistActionEvent} returns this
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.setPromptTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 5, value);
};


/**
 * optional int64 completion_tokens = 6;
 * @return {number}
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.getCompletionTokens = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 6, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AssistActionEvent} returns this
 */
proto.prehog.v1alpha.AssistActionEvent.prototype.setCompletionTokens = function(value) {
  return jspb.Message.setProto3IntField(this, 6, value);
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
proto.prehog.v1alpha.AccessListMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListMetadata;
  return proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
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
proto.prehog.v1alpha.AccessListMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListMetadata.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListMetadata} returns this
 */
proto.prehog.v1alpha.AccessListMetadata.prototype.setId = function(value) {
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
proto.prehog.v1alpha.AccessListCreateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListCreateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListCreateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListCreateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.AccessListMetadata.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.AccessListCreateEvent}
 */
proto.prehog.v1alpha.AccessListCreateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListCreateEvent;
  return proto.prehog.v1alpha.AccessListCreateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListCreateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListCreateEvent}
 */
proto.prehog.v1alpha.AccessListCreateEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.AccessListMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
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
proto.prehog.v1alpha.AccessListCreateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListCreateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListCreateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListCreateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListCreateEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListCreateEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional AccessListMetadata metadata = 2;
 * @return {?proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListCreateEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.AccessListCreateEvent} returns this
*/
proto.prehog.v1alpha.AccessListCreateEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.AccessListCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListCreateEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListCreateEvent.prototype.hasMetadata = function() {
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
proto.prehog.v1alpha.AccessListUpdateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListUpdateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListUpdateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListUpdateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.AccessListMetadata.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.AccessListUpdateEvent}
 */
proto.prehog.v1alpha.AccessListUpdateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListUpdateEvent;
  return proto.prehog.v1alpha.AccessListUpdateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListUpdateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListUpdateEvent}
 */
proto.prehog.v1alpha.AccessListUpdateEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.AccessListMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
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
proto.prehog.v1alpha.AccessListUpdateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListUpdateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListUpdateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListUpdateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListUpdateEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListUpdateEvent} returns this
 */
proto.prehog.v1alpha.AccessListUpdateEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional AccessListMetadata metadata = 2;
 * @return {?proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListUpdateEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.AccessListUpdateEvent} returns this
*/
proto.prehog.v1alpha.AccessListUpdateEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.AccessListUpdateEvent} returns this
 */
proto.prehog.v1alpha.AccessListUpdateEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListUpdateEvent.prototype.hasMetadata = function() {
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
proto.prehog.v1alpha.AccessListDeleteEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListDeleteEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListDeleteEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListDeleteEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.AccessListMetadata.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.AccessListDeleteEvent}
 */
proto.prehog.v1alpha.AccessListDeleteEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListDeleteEvent;
  return proto.prehog.v1alpha.AccessListDeleteEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListDeleteEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListDeleteEvent}
 */
proto.prehog.v1alpha.AccessListDeleteEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.AccessListMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
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
proto.prehog.v1alpha.AccessListDeleteEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListDeleteEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListDeleteEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListDeleteEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListDeleteEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListDeleteEvent} returns this
 */
proto.prehog.v1alpha.AccessListDeleteEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional AccessListMetadata metadata = 2;
 * @return {?proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListDeleteEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.AccessListDeleteEvent} returns this
*/
proto.prehog.v1alpha.AccessListDeleteEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.AccessListDeleteEvent} returns this
 */
proto.prehog.v1alpha.AccessListDeleteEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListDeleteEvent.prototype.hasMetadata = function() {
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
proto.prehog.v1alpha.AccessListMemberCreateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListMemberCreateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListMemberCreateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListMemberCreateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.AccessListMetadata.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.AccessListMemberCreateEvent}
 */
proto.prehog.v1alpha.AccessListMemberCreateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListMemberCreateEvent;
  return proto.prehog.v1alpha.AccessListMemberCreateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListMemberCreateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListMemberCreateEvent}
 */
proto.prehog.v1alpha.AccessListMemberCreateEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.AccessListMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
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
proto.prehog.v1alpha.AccessListMemberCreateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListMemberCreateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListMemberCreateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListMemberCreateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListMemberCreateEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListMemberCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListMemberCreateEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional AccessListMetadata metadata = 2;
 * @return {?proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListMemberCreateEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.AccessListMemberCreateEvent} returns this
*/
proto.prehog.v1alpha.AccessListMemberCreateEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.AccessListMemberCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListMemberCreateEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListMemberCreateEvent.prototype.hasMetadata = function() {
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
proto.prehog.v1alpha.AccessListMemberUpdateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListMemberUpdateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListMemberUpdateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListMemberUpdateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.AccessListMetadata.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.AccessListMemberUpdateEvent}
 */
proto.prehog.v1alpha.AccessListMemberUpdateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListMemberUpdateEvent;
  return proto.prehog.v1alpha.AccessListMemberUpdateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListMemberUpdateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListMemberUpdateEvent}
 */
proto.prehog.v1alpha.AccessListMemberUpdateEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.AccessListMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
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
proto.prehog.v1alpha.AccessListMemberUpdateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListMemberUpdateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListMemberUpdateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListMemberUpdateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListMemberUpdateEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListMemberUpdateEvent} returns this
 */
proto.prehog.v1alpha.AccessListMemberUpdateEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional AccessListMetadata metadata = 2;
 * @return {?proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListMemberUpdateEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.AccessListMemberUpdateEvent} returns this
*/
proto.prehog.v1alpha.AccessListMemberUpdateEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.AccessListMemberUpdateEvent} returns this
 */
proto.prehog.v1alpha.AccessListMemberUpdateEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListMemberUpdateEvent.prototype.hasMetadata = function() {
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
proto.prehog.v1alpha.AccessListMemberDeleteEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListMemberDeleteEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListMemberDeleteEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListMemberDeleteEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.AccessListMetadata.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.AccessListMemberDeleteEvent}
 */
proto.prehog.v1alpha.AccessListMemberDeleteEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListMemberDeleteEvent;
  return proto.prehog.v1alpha.AccessListMemberDeleteEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListMemberDeleteEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListMemberDeleteEvent}
 */
proto.prehog.v1alpha.AccessListMemberDeleteEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.AccessListMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
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
proto.prehog.v1alpha.AccessListMemberDeleteEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListMemberDeleteEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListMemberDeleteEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListMemberDeleteEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListMemberDeleteEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListMemberDeleteEvent} returns this
 */
proto.prehog.v1alpha.AccessListMemberDeleteEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional AccessListMetadata metadata = 2;
 * @return {?proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListMemberDeleteEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.AccessListMemberDeleteEvent} returns this
*/
proto.prehog.v1alpha.AccessListMemberDeleteEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.AccessListMemberDeleteEvent} returns this
 */
proto.prehog.v1alpha.AccessListMemberDeleteEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListMemberDeleteEvent.prototype.hasMetadata = function() {
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
proto.prehog.v1alpha.AccessListGrantsToUserEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListGrantsToUserEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListGrantsToUserEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    countRolesGranted: jspb.Message.getFieldWithDefault(msg, 2, 0),
    countTraitsGranted: jspb.Message.getFieldWithDefault(msg, 3, 0)
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
 * @return {!proto.prehog.v1alpha.AccessListGrantsToUserEvent}
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListGrantsToUserEvent;
  return proto.prehog.v1alpha.AccessListGrantsToUserEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListGrantsToUserEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListGrantsToUserEvent}
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setCountRolesGranted(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setCountTraitsGranted(value);
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
proto.prehog.v1alpha.AccessListGrantsToUserEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListGrantsToUserEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListGrantsToUserEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCountRolesGranted();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getCountTraitsGranted();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListGrantsToUserEvent} returns this
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 count_roles_granted = 2;
 * @return {number}
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.prototype.getCountRolesGranted = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AccessListGrantsToUserEvent} returns this
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.prototype.setCountRolesGranted = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional int32 count_traits_granted = 3;
 * @return {number}
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.prototype.getCountTraitsGranted = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AccessListGrantsToUserEvent} returns this
 */
proto.prehog.v1alpha.AccessListGrantsToUserEvent.prototype.setCountTraitsGranted = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
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
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListReviewCreateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListReviewCreateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.AccessListMetadata.toObject(includeInstance, f),
    daysPastNextAuditDate: jspb.Message.getFieldWithDefault(msg, 3, 0),
    membershipRequirementsChanged: jspb.Message.getBooleanFieldWithDefault(msg, 4, false),
    reviewFrequencyChanged: jspb.Message.getBooleanFieldWithDefault(msg, 5, false),
    reviewDayOfMonthChanged: jspb.Message.getBooleanFieldWithDefault(msg, 6, false),
    numberOfRemovedMembers: jspb.Message.getFieldWithDefault(msg, 7, 0)
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
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListReviewCreateEvent;
  return proto.prehog.v1alpha.AccessListReviewCreateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListReviewCreateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.AccessListMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setDaysPastNextAuditDate(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setMembershipRequirementsChanged(value);
      break;
    case 5:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setReviewFrequencyChanged(value);
      break;
    case 6:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setReviewDayOfMonthChanged(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setNumberOfRemovedMembers(value);
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
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListReviewCreateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListReviewCreateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter
    );
  }
  f = message.getDaysPastNextAuditDate();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
  f = message.getMembershipRequirementsChanged();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
  f = message.getReviewFrequencyChanged();
  if (f) {
    writer.writeBool(
      5,
      f
    );
  }
  f = message.getReviewDayOfMonthChanged();
  if (f) {
    writer.writeBool(
      6,
      f
    );
  }
  f = message.getNumberOfRemovedMembers();
  if (f !== 0) {
    writer.writeInt32(
      7,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional AccessListMetadata metadata = 2;
 * @return {?proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent} returns this
*/
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional int32 days_past_next_audit_date = 3;
 * @return {number}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.getDaysPastNextAuditDate = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.setDaysPastNextAuditDate = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional bool membership_requirements_changed = 4;
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.getMembershipRequirementsChanged = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.setMembershipRequirementsChanged = function(value) {
  return jspb.Message.setProto3BooleanField(this, 4, value);
};


/**
 * optional bool review_frequency_changed = 5;
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.getReviewFrequencyChanged = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 5, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.setReviewFrequencyChanged = function(value) {
  return jspb.Message.setProto3BooleanField(this, 5, value);
};


/**
 * optional bool review_day_of_month_changed = 6;
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.getReviewDayOfMonthChanged = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 6, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.setReviewDayOfMonthChanged = function(value) {
  return jspb.Message.setProto3BooleanField(this, 6, value);
};


/**
 * optional int32 number_of_removed_members = 7;
 * @return {number}
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.getNumberOfRemovedMembers = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 7, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AccessListReviewCreateEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewCreateEvent.prototype.setNumberOfRemovedMembers = function(value) {
  return jspb.Message.setProto3IntField(this, 7, value);
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
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListReviewDeleteEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListReviewDeleteEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.AccessListMetadata.toObject(includeInstance, f),
    accessListReviewId: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.prehog.v1alpha.AccessListReviewDeleteEvent}
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListReviewDeleteEvent;
  return proto.prehog.v1alpha.AccessListReviewDeleteEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListReviewDeleteEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListReviewDeleteEvent}
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = new proto.prehog.v1alpha.AccessListMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setAccessListReviewId(value);
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
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListReviewDeleteEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListReviewDeleteEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.prehog.v1alpha.AccessListMetadata.serializeBinaryToWriter
    );
  }
  f = message.getAccessListReviewId();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListReviewDeleteEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional AccessListMetadata metadata = 2;
 * @return {?proto.prehog.v1alpha.AccessListMetadata}
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMetadata, 2));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.AccessListReviewDeleteEvent} returns this
*/
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.AccessListReviewDeleteEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.hasMetadata = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional string access_list_review_id = 3;
 * @return {string}
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.getAccessListReviewId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AccessListReviewDeleteEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewDeleteEvent.prototype.setAccessListReviewId = function(value) {
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
proto.prehog.v1alpha.AccessListReviewComplianceEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AccessListReviewComplianceEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AccessListReviewComplianceEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListReviewComplianceEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    totalAccessLists: jspb.Message.getFieldWithDefault(msg, 1, 0),
    accessListsNeedReview: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.prehog.v1alpha.AccessListReviewComplianceEvent}
 */
proto.prehog.v1alpha.AccessListReviewComplianceEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AccessListReviewComplianceEvent;
  return proto.prehog.v1alpha.AccessListReviewComplianceEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AccessListReviewComplianceEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AccessListReviewComplianceEvent}
 */
proto.prehog.v1alpha.AccessListReviewComplianceEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setTotalAccessLists(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setAccessListsNeedReview(value);
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
proto.prehog.v1alpha.AccessListReviewComplianceEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AccessListReviewComplianceEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AccessListReviewComplianceEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AccessListReviewComplianceEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTotalAccessLists();
  if (f !== 0) {
    writer.writeInt32(
      1,
      f
    );
  }
  f = message.getAccessListsNeedReview();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
};


/**
 * optional int32 total_access_lists = 1;
 * @return {number}
 */
proto.prehog.v1alpha.AccessListReviewComplianceEvent.prototype.getTotalAccessLists = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AccessListReviewComplianceEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewComplianceEvent.prototype.setTotalAccessLists = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional int32 access_lists_need_review = 2;
 * @return {number}
 */
proto.prehog.v1alpha.AccessListReviewComplianceEvent.prototype.getAccessListsNeedReview = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AccessListReviewComplianceEvent} returns this
 */
proto.prehog.v1alpha.AccessListReviewComplianceEvent.prototype.setAccessListsNeedReview = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
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
proto.prehog.v1alpha.IntegrationEnrollMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.IntegrationEnrollMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.IntegrationEnrollMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    kind: jspb.Message.getFieldWithDefault(msg, 2, 0),
    userName: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.prehog.v1alpha.IntegrationEnrollMetadata}
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.IntegrationEnrollMetadata;
  return proto.prehog.v1alpha.IntegrationEnrollMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.IntegrationEnrollMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.IntegrationEnrollMetadata}
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {!proto.prehog.v1alpha.IntegrationEnrollKind} */ (reader.readEnum());
      msg.setKind(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.IntegrationEnrollMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.IntegrationEnrollMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.IntegrationEnrollMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getKind();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.IntegrationEnrollMetadata} returns this
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional IntegrationEnrollKind kind = 2;
 * @return {!proto.prehog.v1alpha.IntegrationEnrollKind}
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.prototype.getKind = function() {
  return /** @type {!proto.prehog.v1alpha.IntegrationEnrollKind} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.prehog.v1alpha.IntegrationEnrollKind} value
 * @return {!proto.prehog.v1alpha.IntegrationEnrollMetadata} returns this
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.prototype.setKind = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional string user_name = 3;
 * @return {string}
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.IntegrationEnrollMetadata} returns this
 */
proto.prehog.v1alpha.IntegrationEnrollMetadata.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIIntegrationEnrollStartEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.IntegrationEnrollMetadata.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIIntegrationEnrollStartEvent}
 */
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIIntegrationEnrollStartEvent;
  return proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIIntegrationEnrollStartEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIIntegrationEnrollStartEvent}
 */
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.IntegrationEnrollMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.IntegrationEnrollMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
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
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIIntegrationEnrollStartEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.IntegrationEnrollMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional IntegrationEnrollMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.IntegrationEnrollMetadata}
 */
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.IntegrationEnrollMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.IntegrationEnrollMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.IntegrationEnrollMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIIntegrationEnrollStartEvent} returns this
*/
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIIntegrationEnrollStartEvent} returns this
 */
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.prototype.hasMetadata = function() {
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
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    metadata: (f = msg.getMetadata()) && proto.prehog.v1alpha.IntegrationEnrollMetadata.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent}
 */
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent;
  return proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent}
 */
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.IntegrationEnrollMetadata;
      reader.readMessage(value,proto.prehog.v1alpha.IntegrationEnrollMetadata.deserializeBinaryFromReader);
      msg.setMetadata(value);
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
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetadata();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.prehog.v1alpha.IntegrationEnrollMetadata.serializeBinaryToWriter
    );
  }
};


/**
 * optional IntegrationEnrollMetadata metadata = 1;
 * @return {?proto.prehog.v1alpha.IntegrationEnrollMetadata}
 */
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.prototype.getMetadata = function() {
  return /** @type{?proto.prehog.v1alpha.IntegrationEnrollMetadata} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.IntegrationEnrollMetadata, 1));
};


/**
 * @param {?proto.prehog.v1alpha.IntegrationEnrollMetadata|undefined} value
 * @return {!proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent} returns this
*/
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.prototype.setMetadata = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent} returns this
 */
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.prototype.clearMetadata = function() {
  return this.setMetadata(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.prototype.hasMetadata = function() {
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
proto.prehog.v1alpha.EditorChangeEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.EditorChangeEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.EditorChangeEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.EditorChangeEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    status: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.prehog.v1alpha.EditorChangeEvent}
 */
proto.prehog.v1alpha.EditorChangeEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.EditorChangeEvent;
  return proto.prehog.v1alpha.EditorChangeEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.EditorChangeEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.EditorChangeEvent}
 */
proto.prehog.v1alpha.EditorChangeEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {!proto.prehog.v1alpha.EditorChangeStatus} */ (reader.readEnum());
      msg.setStatus(value);
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
proto.prehog.v1alpha.EditorChangeEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.EditorChangeEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.EditorChangeEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.EditorChangeEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.EditorChangeEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.EditorChangeEvent} returns this
 */
proto.prehog.v1alpha.EditorChangeEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional EditorChangeStatus status = 2;
 * @return {!proto.prehog.v1alpha.EditorChangeStatus}
 */
proto.prehog.v1alpha.EditorChangeEvent.prototype.getStatus = function() {
  return /** @type {!proto.prehog.v1alpha.EditorChangeStatus} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.prehog.v1alpha.EditorChangeStatus} value
 * @return {!proto.prehog.v1alpha.EditorChangeEvent} returns this
 */
proto.prehog.v1alpha.EditorChangeEvent.prototype.setStatus = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
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
proto.prehog.v1alpha.DeviceAuthenticateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.DeviceAuthenticateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.DeviceAuthenticateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    deviceId: jspb.Message.getFieldWithDefault(msg, 1, ""),
    userName: jspb.Message.getFieldWithDefault(msg, 2, ""),
    deviceOsType: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.prehog.v1alpha.DeviceAuthenticateEvent}
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.DeviceAuthenticateEvent;
  return proto.prehog.v1alpha.DeviceAuthenticateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.DeviceAuthenticateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.DeviceAuthenticateEvent}
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDeviceOsType(value);
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
proto.prehog.v1alpha.DeviceAuthenticateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.DeviceAuthenticateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.DeviceAuthenticateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDeviceId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDeviceOsType();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string device_id = 1;
 * @return {string}
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.prototype.getDeviceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DeviceAuthenticateEvent} returns this
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.prototype.setDeviceId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string user_name = 2;
 * @return {string}
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DeviceAuthenticateEvent} returns this
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string device_os_type = 3;
 * @return {string}
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.prototype.getDeviceOsType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DeviceAuthenticateEvent} returns this
 */
proto.prehog.v1alpha.DeviceAuthenticateEvent.prototype.setDeviceOsType = function(value) {
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
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.DeviceEnrollEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.DeviceEnrollEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DeviceEnrollEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    deviceId: jspb.Message.getFieldWithDefault(msg, 1, ""),
    userName: jspb.Message.getFieldWithDefault(msg, 2, ""),
    deviceOsType: jspb.Message.getFieldWithDefault(msg, 3, ""),
    deviceOrigin: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.prehog.v1alpha.DeviceEnrollEvent}
 */
proto.prehog.v1alpha.DeviceEnrollEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.DeviceEnrollEvent;
  return proto.prehog.v1alpha.DeviceEnrollEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.DeviceEnrollEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.DeviceEnrollEvent}
 */
proto.prehog.v1alpha.DeviceEnrollEvent.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDeviceOsType(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setDeviceOrigin(value);
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
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.DeviceEnrollEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.DeviceEnrollEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DeviceEnrollEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDeviceId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDeviceOsType();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getDeviceOrigin();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string device_id = 1;
 * @return {string}
 */
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.getDeviceId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DeviceEnrollEvent} returns this
 */
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.setDeviceId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string user_name = 2;
 * @return {string}
 */
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DeviceEnrollEvent} returns this
 */
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string device_os_type = 3;
 * @return {string}
 */
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.getDeviceOsType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DeviceEnrollEvent} returns this
 */
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.setDeviceOsType = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string device_origin = 4;
 * @return {string}
 */
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.getDeviceOrigin = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DeviceEnrollEvent} returns this
 */
proto.prehog.v1alpha.DeviceEnrollEvent.prototype.setDeviceOrigin = function(value) {
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
proto.prehog.v1alpha.FeatureRecommendationEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.FeatureRecommendationEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.FeatureRecommendationEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    feature: jspb.Message.getFieldWithDefault(msg, 2, 0),
    featureRecommendationStatus: jspb.Message.getFieldWithDefault(msg, 3, 0)
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
 * @return {!proto.prehog.v1alpha.FeatureRecommendationEvent}
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.FeatureRecommendationEvent;
  return proto.prehog.v1alpha.FeatureRecommendationEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.FeatureRecommendationEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.FeatureRecommendationEvent}
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {!proto.prehog.v1alpha.Feature} */ (reader.readEnum());
      msg.setFeature(value);
      break;
    case 3:
      var value = /** @type {!proto.prehog.v1alpha.FeatureRecommendationStatus} */ (reader.readEnum());
      msg.setFeatureRecommendationStatus(value);
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
proto.prehog.v1alpha.FeatureRecommendationEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.FeatureRecommendationEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.FeatureRecommendationEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getFeature();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
  f = message.getFeatureRecommendationStatus();
  if (f !== 0.0) {
    writer.writeEnum(
      3,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.FeatureRecommendationEvent} returns this
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional Feature feature = 2;
 * @return {!proto.prehog.v1alpha.Feature}
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.prototype.getFeature = function() {
  return /** @type {!proto.prehog.v1alpha.Feature} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.prehog.v1alpha.Feature} value
 * @return {!proto.prehog.v1alpha.FeatureRecommendationEvent} returns this
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.prototype.setFeature = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};


/**
 * optional FeatureRecommendationStatus feature_recommendation_status = 3;
 * @return {!proto.prehog.v1alpha.FeatureRecommendationStatus}
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.prototype.getFeatureRecommendationStatus = function() {
  return /** @type {!proto.prehog.v1alpha.FeatureRecommendationStatus} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {!proto.prehog.v1alpha.FeatureRecommendationStatus} value
 * @return {!proto.prehog.v1alpha.FeatureRecommendationEvent} returns this
 */
proto.prehog.v1alpha.FeatureRecommendationEvent.prototype.setFeatureRecommendationStatus = function(value) {
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
proto.prehog.v1alpha.LicenseLimitEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.LicenseLimitEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.LicenseLimitEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.LicenseLimitEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    licenseLimit: jspb.Message.getFieldWithDefault(msg, 1, 0)
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
 * @return {!proto.prehog.v1alpha.LicenseLimitEvent}
 */
proto.prehog.v1alpha.LicenseLimitEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.LicenseLimitEvent;
  return proto.prehog.v1alpha.LicenseLimitEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.LicenseLimitEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.LicenseLimitEvent}
 */
proto.prehog.v1alpha.LicenseLimitEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.prehog.v1alpha.LicenseLimit} */ (reader.readEnum());
      msg.setLicenseLimit(value);
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
proto.prehog.v1alpha.LicenseLimitEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.LicenseLimitEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.LicenseLimitEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.LicenseLimitEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getLicenseLimit();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
};


/**
 * optional LicenseLimit license_limit = 1;
 * @return {!proto.prehog.v1alpha.LicenseLimit}
 */
proto.prehog.v1alpha.LicenseLimitEvent.prototype.getLicenseLimit = function() {
  return /** @type {!proto.prehog.v1alpha.LicenseLimit} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.prehog.v1alpha.LicenseLimit} value
 * @return {!proto.prehog.v1alpha.LicenseLimitEvent} returns this
 */
proto.prehog.v1alpha.LicenseLimitEvent.prototype.setLicenseLimit = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
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
proto.prehog.v1alpha.DesktopDirectoryShareEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.DesktopDirectoryShareEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.DesktopDirectoryShareEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    desktop: jspb.Message.getFieldWithDefault(msg, 1, ""),
    userName: jspb.Message.getFieldWithDefault(msg, 2, ""),
    directoryName: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.prehog.v1alpha.DesktopDirectoryShareEvent}
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.DesktopDirectoryShareEvent;
  return proto.prehog.v1alpha.DesktopDirectoryShareEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.DesktopDirectoryShareEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.DesktopDirectoryShareEvent}
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDesktop(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDirectoryName(value);
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
proto.prehog.v1alpha.DesktopDirectoryShareEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.DesktopDirectoryShareEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.DesktopDirectoryShareEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDesktop();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDirectoryName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string desktop = 1;
 * @return {string}
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.prototype.getDesktop = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DesktopDirectoryShareEvent} returns this
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.prototype.setDesktop = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string user_name = 2;
 * @return {string}
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DesktopDirectoryShareEvent} returns this
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string directory_name = 3;
 * @return {string}
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.prototype.getDirectoryName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DesktopDirectoryShareEvent} returns this
 */
proto.prehog.v1alpha.DesktopDirectoryShareEvent.prototype.setDirectoryName = function(value) {
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
proto.prehog.v1alpha.DesktopClipboardEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.DesktopClipboardEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.DesktopClipboardEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DesktopClipboardEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    desktop: jspb.Message.getFieldWithDefault(msg, 1, ""),
    userName: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.prehog.v1alpha.DesktopClipboardEvent}
 */
proto.prehog.v1alpha.DesktopClipboardEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.DesktopClipboardEvent;
  return proto.prehog.v1alpha.DesktopClipboardEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.DesktopClipboardEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.DesktopClipboardEvent}
 */
proto.prehog.v1alpha.DesktopClipboardEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setDesktop(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
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
proto.prehog.v1alpha.DesktopClipboardEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.DesktopClipboardEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.DesktopClipboardEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DesktopClipboardEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDesktop();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string desktop = 1;
 * @return {string}
 */
proto.prehog.v1alpha.DesktopClipboardEvent.prototype.getDesktop = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DesktopClipboardEvent} returns this
 */
proto.prehog.v1alpha.DesktopClipboardEvent.prototype.setDesktop = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string user_name = 2;
 * @return {string}
 */
proto.prehog.v1alpha.DesktopClipboardEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DesktopClipboardEvent} returns this
 */
proto.prehog.v1alpha.DesktopClipboardEvent.prototype.setUserName = function(value) {
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
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.TAGExecuteQueryEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.TAGExecuteQueryEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    totalNodes: jspb.Message.getFieldWithDefault(msg, 2, 0),
    totalEdges: jspb.Message.getFieldWithDefault(msg, 3, 0),
    isSuccess: jspb.Message.getBooleanFieldWithDefault(msg, 4, false)
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
 * @return {!proto.prehog.v1alpha.TAGExecuteQueryEvent}
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.TAGExecuteQueryEvent;
  return proto.prehog.v1alpha.TAGExecuteQueryEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.TAGExecuteQueryEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.TAGExecuteQueryEvent}
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTotalNodes(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt64());
      msg.setTotalEdges(value);
      break;
    case 4:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIsSuccess(value);
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
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.TAGExecuteQueryEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.TAGExecuteQueryEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTotalNodes();
  if (f !== 0) {
    writer.writeInt64(
      2,
      f
    );
  }
  f = message.getTotalEdges();
  if (f !== 0) {
    writer.writeInt64(
      3,
      f
    );
  }
  f = message.getIsSuccess();
  if (f) {
    writer.writeBool(
      4,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.TAGExecuteQueryEvent} returns this
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int64 total_nodes = 2;
 * @return {number}
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.getTotalNodes = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.TAGExecuteQueryEvent} returns this
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.setTotalNodes = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional int64 total_edges = 3;
 * @return {number}
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.getTotalEdges = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.TAGExecuteQueryEvent} returns this
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.setTotalEdges = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
};


/**
 * optional bool is_success = 4;
 * @return {boolean}
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.getIsSuccess = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 4, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.TAGExecuteQueryEvent} returns this
 */
proto.prehog.v1alpha.TAGExecuteQueryEvent.prototype.setIsSuccess = function(value) {
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
proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.toObject = function(includeInstance, msg) {
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
 * @return {!proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent}
 */
proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent;
  return proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent}
 */
proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.deserializeBinaryFromReader = function(msg, reader) {
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
proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.serializeBinaryToWriter = function(message, writer) {
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
proto.prehog.v1alpha.SecurityReportGetResultEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SecurityReportGetResultEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SecurityReportGetResultEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    name: jspb.Message.getFieldWithDefault(msg, 2, ""),
    days: jspb.Message.getFieldWithDefault(msg, 3, 0)
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
 * @return {!proto.prehog.v1alpha.SecurityReportGetResultEvent}
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SecurityReportGetResultEvent;
  return proto.prehog.v1alpha.SecurityReportGetResultEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SecurityReportGetResultEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SecurityReportGetResultEvent}
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setDays(value);
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
proto.prehog.v1alpha.SecurityReportGetResultEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SecurityReportGetResultEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SecurityReportGetResultEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDays();
  if (f !== 0) {
    writer.writeInt32(
      3,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SecurityReportGetResultEvent} returns this
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SecurityReportGetResultEvent} returns this
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional int32 days = 3;
 * @return {number}
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.prototype.getDays = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 3, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.SecurityReportGetResultEvent} returns this
 */
proto.prehog.v1alpha.SecurityReportGetResultEvent.prototype.setDays = function(value) {
  return jspb.Message.setProto3IntField(this, 3, value);
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
proto.prehog.v1alpha.AuditQueryRunEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.AuditQueryRunEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.AuditQueryRunEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AuditQueryRunEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    userName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    days: jspb.Message.getFieldWithDefault(msg, 2, 0),
    isSuccess: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
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
 * @return {!proto.prehog.v1alpha.AuditQueryRunEvent}
 */
proto.prehog.v1alpha.AuditQueryRunEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.AuditQueryRunEvent;
  return proto.prehog.v1alpha.AuditQueryRunEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.AuditQueryRunEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.AuditQueryRunEvent}
 */
proto.prehog.v1alpha.AuditQueryRunEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setUserName(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setDays(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setIsSuccess(value);
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
proto.prehog.v1alpha.AuditQueryRunEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.AuditQueryRunEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.AuditQueryRunEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.AuditQueryRunEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getUserName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getDays();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getIsSuccess();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * optional string user_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.AuditQueryRunEvent.prototype.getUserName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.AuditQueryRunEvent} returns this
 */
proto.prehog.v1alpha.AuditQueryRunEvent.prototype.setUserName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 days = 2;
 * @return {number}
 */
proto.prehog.v1alpha.AuditQueryRunEvent.prototype.getDays = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.prehog.v1alpha.AuditQueryRunEvent} returns this
 */
proto.prehog.v1alpha.AuditQueryRunEvent.prototype.setDays = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional bool is_success = 3;
 * @return {boolean}
 */
proto.prehog.v1alpha.AuditQueryRunEvent.prototype.getIsSuccess = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.prehog.v1alpha.AuditQueryRunEvent} returns this
 */
proto.prehog.v1alpha.AuditQueryRunEvent.prototype.setIsSuccess = function(value) {
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
proto.prehog.v1alpha.DiscoveryFetchEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.DiscoveryFetchEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.DiscoveryFetchEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoveryFetchEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    cloudProvider: jspb.Message.getFieldWithDefault(msg, 1, ""),
    resourceType: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.prehog.v1alpha.DiscoveryFetchEvent}
 */
proto.prehog.v1alpha.DiscoveryFetchEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.DiscoveryFetchEvent;
  return proto.prehog.v1alpha.DiscoveryFetchEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.DiscoveryFetchEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.DiscoveryFetchEvent}
 */
proto.prehog.v1alpha.DiscoveryFetchEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setCloudProvider(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceType(value);
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
proto.prehog.v1alpha.DiscoveryFetchEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.DiscoveryFetchEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.DiscoveryFetchEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.DiscoveryFetchEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCloudProvider();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getResourceType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string cloud_provider = 1;
 * @return {string}
 */
proto.prehog.v1alpha.DiscoveryFetchEvent.prototype.getCloudProvider = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DiscoveryFetchEvent} returns this
 */
proto.prehog.v1alpha.DiscoveryFetchEvent.prototype.setCloudProvider = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string resource_type = 2;
 * @return {string}
 */
proto.prehog.v1alpha.DiscoveryFetchEvent.prototype.getResourceType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.DiscoveryFetchEvent} returns this
 */
proto.prehog.v1alpha.DiscoveryFetchEvent.prototype.setResourceType = function(value) {
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
proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_ = [[3,4,5,6,7,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50,51,52,53,54,55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75,76,77]];

/**
 * @enum {number}
 */
proto.prehog.v1alpha.SubmitEventRequest.EventCase = {
  EVENT_NOT_SET: 0,
  USER_LOGIN: 3,
  SSO_CREATE: 4,
  RESOURCE_CREATE: 5,
  SESSION_START: 6,
  UI_BANNER_CLICK: 7,
  UI_ONBOARD_COMPLETE_GO_TO_DASHBOARD_CLICK: 9,
  UI_ONBOARD_ADD_FIRST_RESOURCE_CLICK: 10,
  UI_ONBOARD_ADD_FIRST_RESOURCE_LATER_CLICK: 11,
  UI_ONBOARD_SET_CREDENTIAL_SUBMIT: 12,
  UI_ONBOARD_REGISTER_CHALLENGE_SUBMIT: 13,
  UI_RECOVERY_CODES_CONTINUE_CLICK: 14,
  UI_RECOVERY_CODES_COPY_CLICK: 15,
  UI_RECOVERY_CODES_PRINT_CLICK: 16,
  UI_DISCOVER_STARTED_EVENT: 17,
  UI_DISCOVER_RESOURCE_SELECTION_EVENT: 18,
  USER_CERTIFICATE_ISSUED_EVENT: 19,
  SESSION_START_V2: 20,
  UI_DISCOVER_DEPLOY_SERVICE_EVENT: 21,
  UI_DISCOVER_DATABASE_REGISTER_EVENT: 22,
  UI_DISCOVER_DATABASE_CONFIGURE_MTLS_EVENT: 23,
  UI_DISCOVER_DESKTOP_ACTIVE_DIRECTORY_TOOLS_INSTALL_EVENT: 24,
  UI_DISCOVER_DESKTOP_ACTIVE_DIRECTORY_CONFIGURE_EVENT: 25,
  UI_DISCOVER_AUTO_DISCOVERED_RESOURCES_EVENT: 26,
  UI_DISCOVER_DATABASE_CONFIGURE_IAM_POLICY_EVENT: 27,
  UI_DISCOVER_PRINCIPALS_CONFIGURE_EVENT: 28,
  UI_DISCOVER_TEST_CONNECTION_EVENT: 29,
  UI_DISCOVER_COMPLETED_EVENT: 30,
  ROLE_CREATE: 31,
  UI_CREATE_NEW_ROLE_CLICK: 32,
  UI_CREATE_NEW_ROLE_SAVE_CLICK: 33,
  UI_CREATE_NEW_ROLE_CANCEL_CLICK: 34,
  UI_CREATE_NEW_ROLE_VIEW_DOCUMENTATION_CLICK: 35,
  KUBE_REQUEST: 36,
  SFTP: 37,
  AGENT_METADATA_EVENT: 38,
  RESOURCE_HEARTBEAT: 39,
  UI_DISCOVER_INTEGRATION_AWS_OIDC_CONNECT_EVENT: 40,
  UI_DISCOVER_DATABASE_RDS_ENROLL_EVENT: 41,
  UI_CALL_TO_ACTION_CLICK_EVENT: 42,
  ASSIST_COMPLETION: 43,
  UI_INTEGRATION_ENROLL_START_EVENT: 44,
  UI_INTEGRATION_ENROLL_COMPLETE_EVENT: 45,
  EDITOR_CHANGE_EVENT: 46,
  BOT_CREATE: 47,
  UI_ONBOARD_QUESTIONNAIRE_SUBMIT: 48,
  BOT_JOIN: 49,
  ASSIST_EXECUTION: 50,
  ASSIST_NEW_CONVERSATION: 51,
  DEVICE_AUTHENTICATE_EVENT: 52,
  FEATURE_RECOMMENDATION_EVENT: 53,
  ASSIST_ACCESS_REQUEST: 54,
  ASSIST_ACTION: 55,
  DEVICE_ENROLL_EVENT: 56,
  LICENSE_LIMIT_EVENT: 57,
  ACCESS_LIST_CREATE: 58,
  ACCESS_LIST_UPDATE: 59,
  ACCESS_LIST_DELETE: 60,
  ACCESS_LIST_MEMBER_CREATE: 61,
  ACCESS_LIST_MEMBER_UPDATE: 62,
  ACCESS_LIST_MEMBER_DELETE: 63,
  ACCESS_LIST_GRANTS_TO_USER: 64,
  UI_DISCOVER_EC2_INSTANCE_SELECTION: 65,
  UI_DISCOVER_DEPLOY_EICE: 66,
  UI_DISCOVER_CREATE_NODE: 67,
  DESKTOP_DIRECTORY_SHARE: 68,
  DESKTOP_CLIPBOARD_TRANSFER: 69,
  TAG_EXECUTE_QUERY: 70,
  EXTERNAL_AUDIT_STORAGE_AUTHENTICATE: 71,
  SECURITY_REPORT_GET_RESULT: 72,
  AUDIT_QUERY_RUN: 73,
  DISCOVERY_FETCH_EVENT: 74,
  ACCESS_LIST_REVIEW_CREATE: 75,
  ACCESS_LIST_REVIEW_DELETE: 76,
  ACCESS_LIST_REVIEW_COMPLIANCE: 77
};

/**
 * @return {proto.prehog.v1alpha.SubmitEventRequest.EventCase}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getEventCase = function() {
  return /** @type {proto.prehog.v1alpha.SubmitEventRequest.EventCase} */(jspb.Message.computeOneofCase(this, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0]));
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
proto.prehog.v1alpha.SubmitEventRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SubmitEventRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SubmitEventRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SubmitEventRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    clusterName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    timestamp: (f = msg.getTimestamp()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
    userLogin: (f = msg.getUserLogin()) && proto.prehog.v1alpha.UserLoginEvent.toObject(includeInstance, f),
    ssoCreate: (f = msg.getSsoCreate()) && proto.prehog.v1alpha.SSOCreateEvent.toObject(includeInstance, f),
    resourceCreate: (f = msg.getResourceCreate()) && proto.prehog.v1alpha.ResourceCreateEvent.toObject(includeInstance, f),
    sessionStart: (f = msg.getSessionStart()) && proto.prehog.v1alpha.SessionStartEvent.toObject(includeInstance, f),
    uiBannerClick: (f = msg.getUiBannerClick()) && proto.prehog.v1alpha.UIBannerClickEvent.toObject(includeInstance, f),
    uiOnboardCompleteGoToDashboardClick: (f = msg.getUiOnboardCompleteGoToDashboardClick()) && proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.toObject(includeInstance, f),
    uiOnboardAddFirstResourceClick: (f = msg.getUiOnboardAddFirstResourceClick()) && proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.toObject(includeInstance, f),
    uiOnboardAddFirstResourceLaterClick: (f = msg.getUiOnboardAddFirstResourceLaterClick()) && proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.toObject(includeInstance, f),
    uiOnboardSetCredentialSubmit: (f = msg.getUiOnboardSetCredentialSubmit()) && proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.toObject(includeInstance, f),
    uiOnboardRegisterChallengeSubmit: (f = msg.getUiOnboardRegisterChallengeSubmit()) && proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.toObject(includeInstance, f),
    uiRecoveryCodesContinueClick: (f = msg.getUiRecoveryCodesContinueClick()) && proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.toObject(includeInstance, f),
    uiRecoveryCodesCopyClick: (f = msg.getUiRecoveryCodesCopyClick()) && proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.toObject(includeInstance, f),
    uiRecoveryCodesPrintClick: (f = msg.getUiRecoveryCodesPrintClick()) && proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.toObject(includeInstance, f),
    uiDiscoverStartedEvent: (f = msg.getUiDiscoverStartedEvent()) && proto.prehog.v1alpha.UIDiscoverStartedEvent.toObject(includeInstance, f),
    uiDiscoverResourceSelectionEvent: (f = msg.getUiDiscoverResourceSelectionEvent()) && proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.toObject(includeInstance, f),
    userCertificateIssuedEvent: (f = msg.getUserCertificateIssuedEvent()) && proto.prehog.v1alpha.UserCertificateIssuedEvent.toObject(includeInstance, f),
    sessionStartV2: (f = msg.getSessionStartV2()) && proto.prehog.v1alpha.SessionStartEvent.toObject(includeInstance, f),
    uiDiscoverDeployServiceEvent: (f = msg.getUiDiscoverDeployServiceEvent()) && proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.toObject(includeInstance, f),
    uiDiscoverDatabaseRegisterEvent: (f = msg.getUiDiscoverDatabaseRegisterEvent()) && proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.toObject(includeInstance, f),
    uiDiscoverDatabaseConfigureMtlsEvent: (f = msg.getUiDiscoverDatabaseConfigureMtlsEvent()) && proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.toObject(includeInstance, f),
    uiDiscoverDesktopActiveDirectoryToolsInstallEvent: (f = msg.getUiDiscoverDesktopActiveDirectoryToolsInstallEvent()) && proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.toObject(includeInstance, f),
    uiDiscoverDesktopActiveDirectoryConfigureEvent: (f = msg.getUiDiscoverDesktopActiveDirectoryConfigureEvent()) && proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.toObject(includeInstance, f),
    uiDiscoverAutoDiscoveredResourcesEvent: (f = msg.getUiDiscoverAutoDiscoveredResourcesEvent()) && proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.toObject(includeInstance, f),
    uiDiscoverDatabaseConfigureIamPolicyEvent: (f = msg.getUiDiscoverDatabaseConfigureIamPolicyEvent()) && proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.toObject(includeInstance, f),
    uiDiscoverPrincipalsConfigureEvent: (f = msg.getUiDiscoverPrincipalsConfigureEvent()) && proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.toObject(includeInstance, f),
    uiDiscoverTestConnectionEvent: (f = msg.getUiDiscoverTestConnectionEvent()) && proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.toObject(includeInstance, f),
    uiDiscoverCompletedEvent: (f = msg.getUiDiscoverCompletedEvent()) && proto.prehog.v1alpha.UIDiscoverCompletedEvent.toObject(includeInstance, f),
    roleCreate: (f = msg.getRoleCreate()) && proto.prehog.v1alpha.RoleCreateEvent.toObject(includeInstance, f),
    uiCreateNewRoleClick: (f = msg.getUiCreateNewRoleClick()) && proto.prehog.v1alpha.UICreateNewRoleClickEvent.toObject(includeInstance, f),
    uiCreateNewRoleSaveClick: (f = msg.getUiCreateNewRoleSaveClick()) && proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.toObject(includeInstance, f),
    uiCreateNewRoleCancelClick: (f = msg.getUiCreateNewRoleCancelClick()) && proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.toObject(includeInstance, f),
    uiCreateNewRoleViewDocumentationClick: (f = msg.getUiCreateNewRoleViewDocumentationClick()) && proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.toObject(includeInstance, f),
    kubeRequest: (f = msg.getKubeRequest()) && proto.prehog.v1alpha.KubeRequestEvent.toObject(includeInstance, f),
    sftp: (f = msg.getSftp()) && proto.prehog.v1alpha.SFTPEvent.toObject(includeInstance, f),
    agentMetadataEvent: (f = msg.getAgentMetadataEvent()) && proto.prehog.v1alpha.AgentMetadataEvent.toObject(includeInstance, f),
    resourceHeartbeat: (f = msg.getResourceHeartbeat()) && proto.prehog.v1alpha.ResourceHeartbeatEvent.toObject(includeInstance, f),
    uiDiscoverIntegrationAwsOidcConnectEvent: (f = msg.getUiDiscoverIntegrationAwsOidcConnectEvent()) && proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.toObject(includeInstance, f),
    uiDiscoverDatabaseRdsEnrollEvent: (f = msg.getUiDiscoverDatabaseRdsEnrollEvent()) && proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.toObject(includeInstance, f),
    uiCallToActionClickEvent: (f = msg.getUiCallToActionClickEvent()) && proto.prehog.v1alpha.UICallToActionClickEvent.toObject(includeInstance, f),
    assistCompletion: (f = msg.getAssistCompletion()) && proto.prehog.v1alpha.AssistCompletionEvent.toObject(includeInstance, f),
    uiIntegrationEnrollStartEvent: (f = msg.getUiIntegrationEnrollStartEvent()) && proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.toObject(includeInstance, f),
    uiIntegrationEnrollCompleteEvent: (f = msg.getUiIntegrationEnrollCompleteEvent()) && proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.toObject(includeInstance, f),
    editorChangeEvent: (f = msg.getEditorChangeEvent()) && proto.prehog.v1alpha.EditorChangeEvent.toObject(includeInstance, f),
    botCreate: (f = msg.getBotCreate()) && proto.prehog.v1alpha.BotCreateEvent.toObject(includeInstance, f),
    uiOnboardQuestionnaireSubmit: (f = msg.getUiOnboardQuestionnaireSubmit()) && proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.toObject(includeInstance, f),
    botJoin: (f = msg.getBotJoin()) && proto.prehog.v1alpha.BotJoinEvent.toObject(includeInstance, f),
    assistExecution: (f = msg.getAssistExecution()) && proto.prehog.v1alpha.AssistExecutionEvent.toObject(includeInstance, f),
    assistNewConversation: (f = msg.getAssistNewConversation()) && proto.prehog.v1alpha.AssistNewConversationEvent.toObject(includeInstance, f),
    deviceAuthenticateEvent: (f = msg.getDeviceAuthenticateEvent()) && proto.prehog.v1alpha.DeviceAuthenticateEvent.toObject(includeInstance, f),
    featureRecommendationEvent: (f = msg.getFeatureRecommendationEvent()) && proto.prehog.v1alpha.FeatureRecommendationEvent.toObject(includeInstance, f),
    assistAccessRequest: (f = msg.getAssistAccessRequest()) && proto.prehog.v1alpha.AssistAccessRequestEvent.toObject(includeInstance, f),
    assistAction: (f = msg.getAssistAction()) && proto.prehog.v1alpha.AssistActionEvent.toObject(includeInstance, f),
    deviceEnrollEvent: (f = msg.getDeviceEnrollEvent()) && proto.prehog.v1alpha.DeviceEnrollEvent.toObject(includeInstance, f),
    licenseLimitEvent: (f = msg.getLicenseLimitEvent()) && proto.prehog.v1alpha.LicenseLimitEvent.toObject(includeInstance, f),
    accessListCreate: (f = msg.getAccessListCreate()) && proto.prehog.v1alpha.AccessListCreateEvent.toObject(includeInstance, f),
    accessListUpdate: (f = msg.getAccessListUpdate()) && proto.prehog.v1alpha.AccessListUpdateEvent.toObject(includeInstance, f),
    accessListDelete: (f = msg.getAccessListDelete()) && proto.prehog.v1alpha.AccessListDeleteEvent.toObject(includeInstance, f),
    accessListMemberCreate: (f = msg.getAccessListMemberCreate()) && proto.prehog.v1alpha.AccessListMemberCreateEvent.toObject(includeInstance, f),
    accessListMemberUpdate: (f = msg.getAccessListMemberUpdate()) && proto.prehog.v1alpha.AccessListMemberUpdateEvent.toObject(includeInstance, f),
    accessListMemberDelete: (f = msg.getAccessListMemberDelete()) && proto.prehog.v1alpha.AccessListMemberDeleteEvent.toObject(includeInstance, f),
    accessListGrantsToUser: (f = msg.getAccessListGrantsToUser()) && proto.prehog.v1alpha.AccessListGrantsToUserEvent.toObject(includeInstance, f),
    uiDiscoverEc2InstanceSelection: (f = msg.getUiDiscoverEc2InstanceSelection()) && proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.toObject(includeInstance, f),
    uiDiscoverDeployEice: (f = msg.getUiDiscoverDeployEice()) && proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.toObject(includeInstance, f),
    uiDiscoverCreateNode: (f = msg.getUiDiscoverCreateNode()) && proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.toObject(includeInstance, f),
    desktopDirectoryShare: (f = msg.getDesktopDirectoryShare()) && proto.prehog.v1alpha.DesktopDirectoryShareEvent.toObject(includeInstance, f),
    desktopClipboardTransfer: (f = msg.getDesktopClipboardTransfer()) && proto.prehog.v1alpha.DesktopClipboardEvent.toObject(includeInstance, f),
    tagExecuteQuery: (f = msg.getTagExecuteQuery()) && proto.prehog.v1alpha.TAGExecuteQueryEvent.toObject(includeInstance, f),
    externalAuditStorageAuthenticate: (f = msg.getExternalAuditStorageAuthenticate()) && proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.toObject(includeInstance, f),
    securityReportGetResult: (f = msg.getSecurityReportGetResult()) && proto.prehog.v1alpha.SecurityReportGetResultEvent.toObject(includeInstance, f),
    auditQueryRun: (f = msg.getAuditQueryRun()) && proto.prehog.v1alpha.AuditQueryRunEvent.toObject(includeInstance, f),
    discoveryFetchEvent: (f = msg.getDiscoveryFetchEvent()) && proto.prehog.v1alpha.DiscoveryFetchEvent.toObject(includeInstance, f),
    accessListReviewCreate: (f = msg.getAccessListReviewCreate()) && proto.prehog.v1alpha.AccessListReviewCreateEvent.toObject(includeInstance, f),
    accessListReviewDelete: (f = msg.getAccessListReviewDelete()) && proto.prehog.v1alpha.AccessListReviewDeleteEvent.toObject(includeInstance, f),
    accessListReviewCompliance: (f = msg.getAccessListReviewCompliance()) && proto.prehog.v1alpha.AccessListReviewComplianceEvent.toObject(includeInstance, f)
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
 * @return {!proto.prehog.v1alpha.SubmitEventRequest}
 */
proto.prehog.v1alpha.SubmitEventRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SubmitEventRequest;
  return proto.prehog.v1alpha.SubmitEventRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SubmitEventRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest}
 */
proto.prehog.v1alpha.SubmitEventRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setClusterName(value);
      break;
    case 2:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setTimestamp(value);
      break;
    case 3:
      var value = new proto.prehog.v1alpha.UserLoginEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UserLoginEvent.deserializeBinaryFromReader);
      msg.setUserLogin(value);
      break;
    case 4:
      var value = new proto.prehog.v1alpha.SSOCreateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.SSOCreateEvent.deserializeBinaryFromReader);
      msg.setSsoCreate(value);
      break;
    case 5:
      var value = new proto.prehog.v1alpha.ResourceCreateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.ResourceCreateEvent.deserializeBinaryFromReader);
      msg.setResourceCreate(value);
      break;
    case 6:
      var value = new proto.prehog.v1alpha.SessionStartEvent;
      reader.readMessage(value,proto.prehog.v1alpha.SessionStartEvent.deserializeBinaryFromReader);
      msg.setSessionStart(value);
      break;
    case 7:
      var value = new proto.prehog.v1alpha.UIBannerClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIBannerClickEvent.deserializeBinaryFromReader);
      msg.setUiBannerClick(value);
      break;
    case 9:
      var value = new proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.deserializeBinaryFromReader);
      msg.setUiOnboardCompleteGoToDashboardClick(value);
      break;
    case 10:
      var value = new proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.deserializeBinaryFromReader);
      msg.setUiOnboardAddFirstResourceClick(value);
      break;
    case 11:
      var value = new proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.deserializeBinaryFromReader);
      msg.setUiOnboardAddFirstResourceLaterClick(value);
      break;
    case 12:
      var value = new proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.deserializeBinaryFromReader);
      msg.setUiOnboardSetCredentialSubmit(value);
      break;
    case 13:
      var value = new proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.deserializeBinaryFromReader);
      msg.setUiOnboardRegisterChallengeSubmit(value);
      break;
    case 14:
      var value = new proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.deserializeBinaryFromReader);
      msg.setUiRecoveryCodesContinueClick(value);
      break;
    case 15:
      var value = new proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.deserializeBinaryFromReader);
      msg.setUiRecoveryCodesCopyClick(value);
      break;
    case 16:
      var value = new proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.deserializeBinaryFromReader);
      msg.setUiRecoveryCodesPrintClick(value);
      break;
    case 17:
      var value = new proto.prehog.v1alpha.UIDiscoverStartedEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverStartedEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverStartedEvent(value);
      break;
    case 18:
      var value = new proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverResourceSelectionEvent(value);
      break;
    case 19:
      var value = new proto.prehog.v1alpha.UserCertificateIssuedEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UserCertificateIssuedEvent.deserializeBinaryFromReader);
      msg.setUserCertificateIssuedEvent(value);
      break;
    case 20:
      var value = new proto.prehog.v1alpha.SessionStartEvent;
      reader.readMessage(value,proto.prehog.v1alpha.SessionStartEvent.deserializeBinaryFromReader);
      msg.setSessionStartV2(value);
      break;
    case 21:
      var value = new proto.prehog.v1alpha.UIDiscoverDeployServiceEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverDeployServiceEvent(value);
      break;
    case 22:
      var value = new proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverDatabaseRegisterEvent(value);
      break;
    case 23:
      var value = new proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverDatabaseConfigureMtlsEvent(value);
      break;
    case 24:
      var value = new proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverDesktopActiveDirectoryToolsInstallEvent(value);
      break;
    case 25:
      var value = new proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverDesktopActiveDirectoryConfigureEvent(value);
      break;
    case 26:
      var value = new proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverAutoDiscoveredResourcesEvent(value);
      break;
    case 27:
      var value = new proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverDatabaseConfigureIamPolicyEvent(value);
      break;
    case 28:
      var value = new proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverPrincipalsConfigureEvent(value);
      break;
    case 29:
      var value = new proto.prehog.v1alpha.UIDiscoverTestConnectionEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverTestConnectionEvent(value);
      break;
    case 30:
      var value = new proto.prehog.v1alpha.UIDiscoverCompletedEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverCompletedEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverCompletedEvent(value);
      break;
    case 31:
      var value = new proto.prehog.v1alpha.RoleCreateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.RoleCreateEvent.deserializeBinaryFromReader);
      msg.setRoleCreate(value);
      break;
    case 32:
      var value = new proto.prehog.v1alpha.UICreateNewRoleClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UICreateNewRoleClickEvent.deserializeBinaryFromReader);
      msg.setUiCreateNewRoleClick(value);
      break;
    case 33:
      var value = new proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.deserializeBinaryFromReader);
      msg.setUiCreateNewRoleSaveClick(value);
      break;
    case 34:
      var value = new proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.deserializeBinaryFromReader);
      msg.setUiCreateNewRoleCancelClick(value);
      break;
    case 35:
      var value = new proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.deserializeBinaryFromReader);
      msg.setUiCreateNewRoleViewDocumentationClick(value);
      break;
    case 36:
      var value = new proto.prehog.v1alpha.KubeRequestEvent;
      reader.readMessage(value,proto.prehog.v1alpha.KubeRequestEvent.deserializeBinaryFromReader);
      msg.setKubeRequest(value);
      break;
    case 37:
      var value = new proto.prehog.v1alpha.SFTPEvent;
      reader.readMessage(value,proto.prehog.v1alpha.SFTPEvent.deserializeBinaryFromReader);
      msg.setSftp(value);
      break;
    case 38:
      var value = new proto.prehog.v1alpha.AgentMetadataEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AgentMetadataEvent.deserializeBinaryFromReader);
      msg.setAgentMetadataEvent(value);
      break;
    case 39:
      var value = new proto.prehog.v1alpha.ResourceHeartbeatEvent;
      reader.readMessage(value,proto.prehog.v1alpha.ResourceHeartbeatEvent.deserializeBinaryFromReader);
      msg.setResourceHeartbeat(value);
      break;
    case 40:
      var value = new proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverIntegrationAwsOidcConnectEvent(value);
      break;
    case 41:
      var value = new proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverDatabaseRdsEnrollEvent(value);
      break;
    case 42:
      var value = new proto.prehog.v1alpha.UICallToActionClickEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UICallToActionClickEvent.deserializeBinaryFromReader);
      msg.setUiCallToActionClickEvent(value);
      break;
    case 43:
      var value = new proto.prehog.v1alpha.AssistCompletionEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AssistCompletionEvent.deserializeBinaryFromReader);
      msg.setAssistCompletion(value);
      break;
    case 44:
      var value = new proto.prehog.v1alpha.UIIntegrationEnrollStartEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.deserializeBinaryFromReader);
      msg.setUiIntegrationEnrollStartEvent(value);
      break;
    case 45:
      var value = new proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.deserializeBinaryFromReader);
      msg.setUiIntegrationEnrollCompleteEvent(value);
      break;
    case 46:
      var value = new proto.prehog.v1alpha.EditorChangeEvent;
      reader.readMessage(value,proto.prehog.v1alpha.EditorChangeEvent.deserializeBinaryFromReader);
      msg.setEditorChangeEvent(value);
      break;
    case 47:
      var value = new proto.prehog.v1alpha.BotCreateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.BotCreateEvent.deserializeBinaryFromReader);
      msg.setBotCreate(value);
      break;
    case 48:
      var value = new proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.deserializeBinaryFromReader);
      msg.setUiOnboardQuestionnaireSubmit(value);
      break;
    case 49:
      var value = new proto.prehog.v1alpha.BotJoinEvent;
      reader.readMessage(value,proto.prehog.v1alpha.BotJoinEvent.deserializeBinaryFromReader);
      msg.setBotJoin(value);
      break;
    case 50:
      var value = new proto.prehog.v1alpha.AssistExecutionEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AssistExecutionEvent.deserializeBinaryFromReader);
      msg.setAssistExecution(value);
      break;
    case 51:
      var value = new proto.prehog.v1alpha.AssistNewConversationEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AssistNewConversationEvent.deserializeBinaryFromReader);
      msg.setAssistNewConversation(value);
      break;
    case 52:
      var value = new proto.prehog.v1alpha.DeviceAuthenticateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.DeviceAuthenticateEvent.deserializeBinaryFromReader);
      msg.setDeviceAuthenticateEvent(value);
      break;
    case 53:
      var value = new proto.prehog.v1alpha.FeatureRecommendationEvent;
      reader.readMessage(value,proto.prehog.v1alpha.FeatureRecommendationEvent.deserializeBinaryFromReader);
      msg.setFeatureRecommendationEvent(value);
      break;
    case 54:
      var value = new proto.prehog.v1alpha.AssistAccessRequestEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AssistAccessRequestEvent.deserializeBinaryFromReader);
      msg.setAssistAccessRequest(value);
      break;
    case 55:
      var value = new proto.prehog.v1alpha.AssistActionEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AssistActionEvent.deserializeBinaryFromReader);
      msg.setAssistAction(value);
      break;
    case 56:
      var value = new proto.prehog.v1alpha.DeviceEnrollEvent;
      reader.readMessage(value,proto.prehog.v1alpha.DeviceEnrollEvent.deserializeBinaryFromReader);
      msg.setDeviceEnrollEvent(value);
      break;
    case 57:
      var value = new proto.prehog.v1alpha.LicenseLimitEvent;
      reader.readMessage(value,proto.prehog.v1alpha.LicenseLimitEvent.deserializeBinaryFromReader);
      msg.setLicenseLimitEvent(value);
      break;
    case 58:
      var value = new proto.prehog.v1alpha.AccessListCreateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListCreateEvent.deserializeBinaryFromReader);
      msg.setAccessListCreate(value);
      break;
    case 59:
      var value = new proto.prehog.v1alpha.AccessListUpdateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListUpdateEvent.deserializeBinaryFromReader);
      msg.setAccessListUpdate(value);
      break;
    case 60:
      var value = new proto.prehog.v1alpha.AccessListDeleteEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListDeleteEvent.deserializeBinaryFromReader);
      msg.setAccessListDelete(value);
      break;
    case 61:
      var value = new proto.prehog.v1alpha.AccessListMemberCreateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMemberCreateEvent.deserializeBinaryFromReader);
      msg.setAccessListMemberCreate(value);
      break;
    case 62:
      var value = new proto.prehog.v1alpha.AccessListMemberUpdateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMemberUpdateEvent.deserializeBinaryFromReader);
      msg.setAccessListMemberUpdate(value);
      break;
    case 63:
      var value = new proto.prehog.v1alpha.AccessListMemberDeleteEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListMemberDeleteEvent.deserializeBinaryFromReader);
      msg.setAccessListMemberDelete(value);
      break;
    case 64:
      var value = new proto.prehog.v1alpha.AccessListGrantsToUserEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListGrantsToUserEvent.deserializeBinaryFromReader);
      msg.setAccessListGrantsToUser(value);
      break;
    case 65:
      var value = new proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverEc2InstanceSelection(value);
      break;
    case 66:
      var value = new proto.prehog.v1alpha.UIDiscoverDeployEICEEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverDeployEice(value);
      break;
    case 67:
      var value = new proto.prehog.v1alpha.UIDiscoverCreateNodeEvent;
      reader.readMessage(value,proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.deserializeBinaryFromReader);
      msg.setUiDiscoverCreateNode(value);
      break;
    case 68:
      var value = new proto.prehog.v1alpha.DesktopDirectoryShareEvent;
      reader.readMessage(value,proto.prehog.v1alpha.DesktopDirectoryShareEvent.deserializeBinaryFromReader);
      msg.setDesktopDirectoryShare(value);
      break;
    case 69:
      var value = new proto.prehog.v1alpha.DesktopClipboardEvent;
      reader.readMessage(value,proto.prehog.v1alpha.DesktopClipboardEvent.deserializeBinaryFromReader);
      msg.setDesktopClipboardTransfer(value);
      break;
    case 70:
      var value = new proto.prehog.v1alpha.TAGExecuteQueryEvent;
      reader.readMessage(value,proto.prehog.v1alpha.TAGExecuteQueryEvent.deserializeBinaryFromReader);
      msg.setTagExecuteQuery(value);
      break;
    case 71:
      var value = new proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.deserializeBinaryFromReader);
      msg.setExternalAuditStorageAuthenticate(value);
      break;
    case 72:
      var value = new proto.prehog.v1alpha.SecurityReportGetResultEvent;
      reader.readMessage(value,proto.prehog.v1alpha.SecurityReportGetResultEvent.deserializeBinaryFromReader);
      msg.setSecurityReportGetResult(value);
      break;
    case 73:
      var value = new proto.prehog.v1alpha.AuditQueryRunEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AuditQueryRunEvent.deserializeBinaryFromReader);
      msg.setAuditQueryRun(value);
      break;
    case 74:
      var value = new proto.prehog.v1alpha.DiscoveryFetchEvent;
      reader.readMessage(value,proto.prehog.v1alpha.DiscoveryFetchEvent.deserializeBinaryFromReader);
      msg.setDiscoveryFetchEvent(value);
      break;
    case 75:
      var value = new proto.prehog.v1alpha.AccessListReviewCreateEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListReviewCreateEvent.deserializeBinaryFromReader);
      msg.setAccessListReviewCreate(value);
      break;
    case 76:
      var value = new proto.prehog.v1alpha.AccessListReviewDeleteEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListReviewDeleteEvent.deserializeBinaryFromReader);
      msg.setAccessListReviewDelete(value);
      break;
    case 77:
      var value = new proto.prehog.v1alpha.AccessListReviewComplianceEvent;
      reader.readMessage(value,proto.prehog.v1alpha.AccessListReviewComplianceEvent.deserializeBinaryFromReader);
      msg.setAccessListReviewCompliance(value);
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
proto.prehog.v1alpha.SubmitEventRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SubmitEventRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SubmitEventRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SubmitEventRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getClusterName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getTimestamp();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getUserLogin();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.prehog.v1alpha.UserLoginEvent.serializeBinaryToWriter
    );
  }
  f = message.getSsoCreate();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.prehog.v1alpha.SSOCreateEvent.serializeBinaryToWriter
    );
  }
  f = message.getResourceCreate();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      proto.prehog.v1alpha.ResourceCreateEvent.serializeBinaryToWriter
    );
  }
  f = message.getSessionStart();
  if (f != null) {
    writer.writeMessage(
      6,
      f,
      proto.prehog.v1alpha.SessionStartEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiBannerClick();
  if (f != null) {
    writer.writeMessage(
      7,
      f,
      proto.prehog.v1alpha.UIBannerClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiOnboardCompleteGoToDashboardClick();
  if (f != null) {
    writer.writeMessage(
      9,
      f,
      proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiOnboardAddFirstResourceClick();
  if (f != null) {
    writer.writeMessage(
      10,
      f,
      proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiOnboardAddFirstResourceLaterClick();
  if (f != null) {
    writer.writeMessage(
      11,
      f,
      proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiOnboardSetCredentialSubmit();
  if (f != null) {
    writer.writeMessage(
      12,
      f,
      proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiOnboardRegisterChallengeSubmit();
  if (f != null) {
    writer.writeMessage(
      13,
      f,
      proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiRecoveryCodesContinueClick();
  if (f != null) {
    writer.writeMessage(
      14,
      f,
      proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiRecoveryCodesCopyClick();
  if (f != null) {
    writer.writeMessage(
      15,
      f,
      proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiRecoveryCodesPrintClick();
  if (f != null) {
    writer.writeMessage(
      16,
      f,
      proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverStartedEvent();
  if (f != null) {
    writer.writeMessage(
      17,
      f,
      proto.prehog.v1alpha.UIDiscoverStartedEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverResourceSelectionEvent();
  if (f != null) {
    writer.writeMessage(
      18,
      f,
      proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent.serializeBinaryToWriter
    );
  }
  f = message.getUserCertificateIssuedEvent();
  if (f != null) {
    writer.writeMessage(
      19,
      f,
      proto.prehog.v1alpha.UserCertificateIssuedEvent.serializeBinaryToWriter
    );
  }
  f = message.getSessionStartV2();
  if (f != null) {
    writer.writeMessage(
      20,
      f,
      proto.prehog.v1alpha.SessionStartEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverDeployServiceEvent();
  if (f != null) {
    writer.writeMessage(
      21,
      f,
      proto.prehog.v1alpha.UIDiscoverDeployServiceEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverDatabaseRegisterEvent();
  if (f != null) {
    writer.writeMessage(
      22,
      f,
      proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverDatabaseConfigureMtlsEvent();
  if (f != null) {
    writer.writeMessage(
      23,
      f,
      proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverDesktopActiveDirectoryToolsInstallEvent();
  if (f != null) {
    writer.writeMessage(
      24,
      f,
      proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverDesktopActiveDirectoryConfigureEvent();
  if (f != null) {
    writer.writeMessage(
      25,
      f,
      proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverAutoDiscoveredResourcesEvent();
  if (f != null) {
    writer.writeMessage(
      26,
      f,
      proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverDatabaseConfigureIamPolicyEvent();
  if (f != null) {
    writer.writeMessage(
      27,
      f,
      proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverPrincipalsConfigureEvent();
  if (f != null) {
    writer.writeMessage(
      28,
      f,
      proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverTestConnectionEvent();
  if (f != null) {
    writer.writeMessage(
      29,
      f,
      proto.prehog.v1alpha.UIDiscoverTestConnectionEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverCompletedEvent();
  if (f != null) {
    writer.writeMessage(
      30,
      f,
      proto.prehog.v1alpha.UIDiscoverCompletedEvent.serializeBinaryToWriter
    );
  }
  f = message.getRoleCreate();
  if (f != null) {
    writer.writeMessage(
      31,
      f,
      proto.prehog.v1alpha.RoleCreateEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiCreateNewRoleClick();
  if (f != null) {
    writer.writeMessage(
      32,
      f,
      proto.prehog.v1alpha.UICreateNewRoleClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiCreateNewRoleSaveClick();
  if (f != null) {
    writer.writeMessage(
      33,
      f,
      proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiCreateNewRoleCancelClick();
  if (f != null) {
    writer.writeMessage(
      34,
      f,
      proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiCreateNewRoleViewDocumentationClick();
  if (f != null) {
    writer.writeMessage(
      35,
      f,
      proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getKubeRequest();
  if (f != null) {
    writer.writeMessage(
      36,
      f,
      proto.prehog.v1alpha.KubeRequestEvent.serializeBinaryToWriter
    );
  }
  f = message.getSftp();
  if (f != null) {
    writer.writeMessage(
      37,
      f,
      proto.prehog.v1alpha.SFTPEvent.serializeBinaryToWriter
    );
  }
  f = message.getAgentMetadataEvent();
  if (f != null) {
    writer.writeMessage(
      38,
      f,
      proto.prehog.v1alpha.AgentMetadataEvent.serializeBinaryToWriter
    );
  }
  f = message.getResourceHeartbeat();
  if (f != null) {
    writer.writeMessage(
      39,
      f,
      proto.prehog.v1alpha.ResourceHeartbeatEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverIntegrationAwsOidcConnectEvent();
  if (f != null) {
    writer.writeMessage(
      40,
      f,
      proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverDatabaseRdsEnrollEvent();
  if (f != null) {
    writer.writeMessage(
      41,
      f,
      proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiCallToActionClickEvent();
  if (f != null) {
    writer.writeMessage(
      42,
      f,
      proto.prehog.v1alpha.UICallToActionClickEvent.serializeBinaryToWriter
    );
  }
  f = message.getAssistCompletion();
  if (f != null) {
    writer.writeMessage(
      43,
      f,
      proto.prehog.v1alpha.AssistCompletionEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiIntegrationEnrollStartEvent();
  if (f != null) {
    writer.writeMessage(
      44,
      f,
      proto.prehog.v1alpha.UIIntegrationEnrollStartEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiIntegrationEnrollCompleteEvent();
  if (f != null) {
    writer.writeMessage(
      45,
      f,
      proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent.serializeBinaryToWriter
    );
  }
  f = message.getEditorChangeEvent();
  if (f != null) {
    writer.writeMessage(
      46,
      f,
      proto.prehog.v1alpha.EditorChangeEvent.serializeBinaryToWriter
    );
  }
  f = message.getBotCreate();
  if (f != null) {
    writer.writeMessage(
      47,
      f,
      proto.prehog.v1alpha.BotCreateEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiOnboardQuestionnaireSubmit();
  if (f != null) {
    writer.writeMessage(
      48,
      f,
      proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent.serializeBinaryToWriter
    );
  }
  f = message.getBotJoin();
  if (f != null) {
    writer.writeMessage(
      49,
      f,
      proto.prehog.v1alpha.BotJoinEvent.serializeBinaryToWriter
    );
  }
  f = message.getAssistExecution();
  if (f != null) {
    writer.writeMessage(
      50,
      f,
      proto.prehog.v1alpha.AssistExecutionEvent.serializeBinaryToWriter
    );
  }
  f = message.getAssistNewConversation();
  if (f != null) {
    writer.writeMessage(
      51,
      f,
      proto.prehog.v1alpha.AssistNewConversationEvent.serializeBinaryToWriter
    );
  }
  f = message.getDeviceAuthenticateEvent();
  if (f != null) {
    writer.writeMessage(
      52,
      f,
      proto.prehog.v1alpha.DeviceAuthenticateEvent.serializeBinaryToWriter
    );
  }
  f = message.getFeatureRecommendationEvent();
  if (f != null) {
    writer.writeMessage(
      53,
      f,
      proto.prehog.v1alpha.FeatureRecommendationEvent.serializeBinaryToWriter
    );
  }
  f = message.getAssistAccessRequest();
  if (f != null) {
    writer.writeMessage(
      54,
      f,
      proto.prehog.v1alpha.AssistAccessRequestEvent.serializeBinaryToWriter
    );
  }
  f = message.getAssistAction();
  if (f != null) {
    writer.writeMessage(
      55,
      f,
      proto.prehog.v1alpha.AssistActionEvent.serializeBinaryToWriter
    );
  }
  f = message.getDeviceEnrollEvent();
  if (f != null) {
    writer.writeMessage(
      56,
      f,
      proto.prehog.v1alpha.DeviceEnrollEvent.serializeBinaryToWriter
    );
  }
  f = message.getLicenseLimitEvent();
  if (f != null) {
    writer.writeMessage(
      57,
      f,
      proto.prehog.v1alpha.LicenseLimitEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListCreate();
  if (f != null) {
    writer.writeMessage(
      58,
      f,
      proto.prehog.v1alpha.AccessListCreateEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListUpdate();
  if (f != null) {
    writer.writeMessage(
      59,
      f,
      proto.prehog.v1alpha.AccessListUpdateEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListDelete();
  if (f != null) {
    writer.writeMessage(
      60,
      f,
      proto.prehog.v1alpha.AccessListDeleteEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListMemberCreate();
  if (f != null) {
    writer.writeMessage(
      61,
      f,
      proto.prehog.v1alpha.AccessListMemberCreateEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListMemberUpdate();
  if (f != null) {
    writer.writeMessage(
      62,
      f,
      proto.prehog.v1alpha.AccessListMemberUpdateEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListMemberDelete();
  if (f != null) {
    writer.writeMessage(
      63,
      f,
      proto.prehog.v1alpha.AccessListMemberDeleteEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListGrantsToUser();
  if (f != null) {
    writer.writeMessage(
      64,
      f,
      proto.prehog.v1alpha.AccessListGrantsToUserEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverEc2InstanceSelection();
  if (f != null) {
    writer.writeMessage(
      65,
      f,
      proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverDeployEice();
  if (f != null) {
    writer.writeMessage(
      66,
      f,
      proto.prehog.v1alpha.UIDiscoverDeployEICEEvent.serializeBinaryToWriter
    );
  }
  f = message.getUiDiscoverCreateNode();
  if (f != null) {
    writer.writeMessage(
      67,
      f,
      proto.prehog.v1alpha.UIDiscoverCreateNodeEvent.serializeBinaryToWriter
    );
  }
  f = message.getDesktopDirectoryShare();
  if (f != null) {
    writer.writeMessage(
      68,
      f,
      proto.prehog.v1alpha.DesktopDirectoryShareEvent.serializeBinaryToWriter
    );
  }
  f = message.getDesktopClipboardTransfer();
  if (f != null) {
    writer.writeMessage(
      69,
      f,
      proto.prehog.v1alpha.DesktopClipboardEvent.serializeBinaryToWriter
    );
  }
  f = message.getTagExecuteQuery();
  if (f != null) {
    writer.writeMessage(
      70,
      f,
      proto.prehog.v1alpha.TAGExecuteQueryEvent.serializeBinaryToWriter
    );
  }
  f = message.getExternalAuditStorageAuthenticate();
  if (f != null) {
    writer.writeMessage(
      71,
      f,
      proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent.serializeBinaryToWriter
    );
  }
  f = message.getSecurityReportGetResult();
  if (f != null) {
    writer.writeMessage(
      72,
      f,
      proto.prehog.v1alpha.SecurityReportGetResultEvent.serializeBinaryToWriter
    );
  }
  f = message.getAuditQueryRun();
  if (f != null) {
    writer.writeMessage(
      73,
      f,
      proto.prehog.v1alpha.AuditQueryRunEvent.serializeBinaryToWriter
    );
  }
  f = message.getDiscoveryFetchEvent();
  if (f != null) {
    writer.writeMessage(
      74,
      f,
      proto.prehog.v1alpha.DiscoveryFetchEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListReviewCreate();
  if (f != null) {
    writer.writeMessage(
      75,
      f,
      proto.prehog.v1alpha.AccessListReviewCreateEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListReviewDelete();
  if (f != null) {
    writer.writeMessage(
      76,
      f,
      proto.prehog.v1alpha.AccessListReviewDeleteEvent.serializeBinaryToWriter
    );
  }
  f = message.getAccessListReviewCompliance();
  if (f != null) {
    writer.writeMessage(
      77,
      f,
      proto.prehog.v1alpha.AccessListReviewComplianceEvent.serializeBinaryToWriter
    );
  }
};


/**
 * optional string cluster_name = 1;
 * @return {string}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getClusterName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.setClusterName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional google.protobuf.Timestamp timestamp = 2;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getTimestamp = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 2));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setTimestamp = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearTimestamp = function() {
  return this.setTimestamp(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasTimestamp = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional UserLoginEvent user_login = 3;
 * @return {?proto.prehog.v1alpha.UserLoginEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUserLogin = function() {
  return /** @type{?proto.prehog.v1alpha.UserLoginEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UserLoginEvent, 3));
};


/**
 * @param {?proto.prehog.v1alpha.UserLoginEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUserLogin = function(value) {
  return jspb.Message.setOneofWrapperField(this, 3, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUserLogin = function() {
  return this.setUserLogin(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUserLogin = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional SSOCreateEvent sso_create = 4;
 * @return {?proto.prehog.v1alpha.SSOCreateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getSsoCreate = function() {
  return /** @type{?proto.prehog.v1alpha.SSOCreateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.SSOCreateEvent, 4));
};


/**
 * @param {?proto.prehog.v1alpha.SSOCreateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setSsoCreate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 4, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearSsoCreate = function() {
  return this.setSsoCreate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasSsoCreate = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional ResourceCreateEvent resource_create = 5;
 * @return {?proto.prehog.v1alpha.ResourceCreateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getResourceCreate = function() {
  return /** @type{?proto.prehog.v1alpha.ResourceCreateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.ResourceCreateEvent, 5));
};


/**
 * @param {?proto.prehog.v1alpha.ResourceCreateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setResourceCreate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 5, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearResourceCreate = function() {
  return this.setResourceCreate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasResourceCreate = function() {
  return jspb.Message.getField(this, 5) != null;
};


/**
 * optional SessionStartEvent session_start = 6;
 * @return {?proto.prehog.v1alpha.SessionStartEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getSessionStart = function() {
  return /** @type{?proto.prehog.v1alpha.SessionStartEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.SessionStartEvent, 6));
};


/**
 * @param {?proto.prehog.v1alpha.SessionStartEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setSessionStart = function(value) {
  return jspb.Message.setOneofWrapperField(this, 6, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearSessionStart = function() {
  return this.setSessionStart(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasSessionStart = function() {
  return jspb.Message.getField(this, 6) != null;
};


/**
 * optional UIBannerClickEvent ui_banner_click = 7;
 * @return {?proto.prehog.v1alpha.UIBannerClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiBannerClick = function() {
  return /** @type{?proto.prehog.v1alpha.UIBannerClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIBannerClickEvent, 7));
};


/**
 * @param {?proto.prehog.v1alpha.UIBannerClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiBannerClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 7, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiBannerClick = function() {
  return this.setUiBannerClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiBannerClick = function() {
  return jspb.Message.getField(this, 7) != null;
};


/**
 * optional UIOnboardCompleteGoToDashboardClickEvent ui_onboard_complete_go_to_dashboard_click = 9;
 * @return {?proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiOnboardCompleteGoToDashboardClick = function() {
  return /** @type{?proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent, 9));
};


/**
 * @param {?proto.prehog.v1alpha.UIOnboardCompleteGoToDashboardClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiOnboardCompleteGoToDashboardClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 9, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiOnboardCompleteGoToDashboardClick = function() {
  return this.setUiOnboardCompleteGoToDashboardClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiOnboardCompleteGoToDashboardClick = function() {
  return jspb.Message.getField(this, 9) != null;
};


/**
 * optional UIOnboardAddFirstResourceClickEvent ui_onboard_add_first_resource_click = 10;
 * @return {?proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiOnboardAddFirstResourceClick = function() {
  return /** @type{?proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent, 10));
};


/**
 * @param {?proto.prehog.v1alpha.UIOnboardAddFirstResourceClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiOnboardAddFirstResourceClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 10, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiOnboardAddFirstResourceClick = function() {
  return this.setUiOnboardAddFirstResourceClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiOnboardAddFirstResourceClick = function() {
  return jspb.Message.getField(this, 10) != null;
};


/**
 * optional UIOnboardAddFirstResourceLaterClickEvent ui_onboard_add_first_resource_later_click = 11;
 * @return {?proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiOnboardAddFirstResourceLaterClick = function() {
  return /** @type{?proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent, 11));
};


/**
 * @param {?proto.prehog.v1alpha.UIOnboardAddFirstResourceLaterClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiOnboardAddFirstResourceLaterClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 11, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiOnboardAddFirstResourceLaterClick = function() {
  return this.setUiOnboardAddFirstResourceLaterClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiOnboardAddFirstResourceLaterClick = function() {
  return jspb.Message.getField(this, 11) != null;
};


/**
 * optional UIOnboardSetCredentialSubmitEvent ui_onboard_set_credential_submit = 12;
 * @return {?proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiOnboardSetCredentialSubmit = function() {
  return /** @type{?proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent, 12));
};


/**
 * @param {?proto.prehog.v1alpha.UIOnboardSetCredentialSubmitEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiOnboardSetCredentialSubmit = function(value) {
  return jspb.Message.setOneofWrapperField(this, 12, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiOnboardSetCredentialSubmit = function() {
  return this.setUiOnboardSetCredentialSubmit(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiOnboardSetCredentialSubmit = function() {
  return jspb.Message.getField(this, 12) != null;
};


/**
 * optional UIOnboardRegisterChallengeSubmitEvent ui_onboard_register_challenge_submit = 13;
 * @return {?proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiOnboardRegisterChallengeSubmit = function() {
  return /** @type{?proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent, 13));
};


/**
 * @param {?proto.prehog.v1alpha.UIOnboardRegisterChallengeSubmitEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiOnboardRegisterChallengeSubmit = function(value) {
  return jspb.Message.setOneofWrapperField(this, 13, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiOnboardRegisterChallengeSubmit = function() {
  return this.setUiOnboardRegisterChallengeSubmit(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiOnboardRegisterChallengeSubmit = function() {
  return jspb.Message.getField(this, 13) != null;
};


/**
 * optional UIRecoveryCodesContinueClickEvent ui_recovery_codes_continue_click = 14;
 * @return {?proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiRecoveryCodesContinueClick = function() {
  return /** @type{?proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent, 14));
};


/**
 * @param {?proto.prehog.v1alpha.UIRecoveryCodesContinueClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiRecoveryCodesContinueClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 14, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiRecoveryCodesContinueClick = function() {
  return this.setUiRecoveryCodesContinueClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiRecoveryCodesContinueClick = function() {
  return jspb.Message.getField(this, 14) != null;
};


/**
 * optional UIRecoveryCodesCopyClickEvent ui_recovery_codes_copy_click = 15;
 * @return {?proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiRecoveryCodesCopyClick = function() {
  return /** @type{?proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent, 15));
};


/**
 * @param {?proto.prehog.v1alpha.UIRecoveryCodesCopyClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiRecoveryCodesCopyClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 15, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiRecoveryCodesCopyClick = function() {
  return this.setUiRecoveryCodesCopyClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiRecoveryCodesCopyClick = function() {
  return jspb.Message.getField(this, 15) != null;
};


/**
 * optional UIRecoveryCodesPrintClickEvent ui_recovery_codes_print_click = 16;
 * @return {?proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiRecoveryCodesPrintClick = function() {
  return /** @type{?proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent, 16));
};


/**
 * @param {?proto.prehog.v1alpha.UIRecoveryCodesPrintClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiRecoveryCodesPrintClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 16, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiRecoveryCodesPrintClick = function() {
  return this.setUiRecoveryCodesPrintClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiRecoveryCodesPrintClick = function() {
  return jspb.Message.getField(this, 16) != null;
};


/**
 * optional UIDiscoverStartedEvent ui_discover_started_event = 17;
 * @return {?proto.prehog.v1alpha.UIDiscoverStartedEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverStartedEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverStartedEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverStartedEvent, 17));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverStartedEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverStartedEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 17, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverStartedEvent = function() {
  return this.setUiDiscoverStartedEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverStartedEvent = function() {
  return jspb.Message.getField(this, 17) != null;
};


/**
 * optional UIDiscoverResourceSelectionEvent ui_discover_resource_selection_event = 18;
 * @return {?proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverResourceSelectionEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent, 18));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverResourceSelectionEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverResourceSelectionEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 18, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverResourceSelectionEvent = function() {
  return this.setUiDiscoverResourceSelectionEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverResourceSelectionEvent = function() {
  return jspb.Message.getField(this, 18) != null;
};


/**
 * optional UserCertificateIssuedEvent user_certificate_issued_event = 19;
 * @return {?proto.prehog.v1alpha.UserCertificateIssuedEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUserCertificateIssuedEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UserCertificateIssuedEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UserCertificateIssuedEvent, 19));
};


/**
 * @param {?proto.prehog.v1alpha.UserCertificateIssuedEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUserCertificateIssuedEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 19, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUserCertificateIssuedEvent = function() {
  return this.setUserCertificateIssuedEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUserCertificateIssuedEvent = function() {
  return jspb.Message.getField(this, 19) != null;
};


/**
 * optional SessionStartEvent session_start_v2 = 20;
 * @return {?proto.prehog.v1alpha.SessionStartEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getSessionStartV2 = function() {
  return /** @type{?proto.prehog.v1alpha.SessionStartEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.SessionStartEvent, 20));
};


/**
 * @param {?proto.prehog.v1alpha.SessionStartEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setSessionStartV2 = function(value) {
  return jspb.Message.setOneofWrapperField(this, 20, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearSessionStartV2 = function() {
  return this.setSessionStartV2(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasSessionStartV2 = function() {
  return jspb.Message.getField(this, 20) != null;
};


/**
 * optional UIDiscoverDeployServiceEvent ui_discover_deploy_service_event = 21;
 * @return {?proto.prehog.v1alpha.UIDiscoverDeployServiceEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverDeployServiceEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverDeployServiceEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverDeployServiceEvent, 21));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverDeployServiceEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverDeployServiceEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 21, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverDeployServiceEvent = function() {
  return this.setUiDiscoverDeployServiceEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverDeployServiceEvent = function() {
  return jspb.Message.getField(this, 21) != null;
};


/**
 * optional UIDiscoverDatabaseRegisterEvent ui_discover_database_register_event = 22;
 * @return {?proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverDatabaseRegisterEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent, 22));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverDatabaseRegisterEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverDatabaseRegisterEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 22, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverDatabaseRegisterEvent = function() {
  return this.setUiDiscoverDatabaseRegisterEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverDatabaseRegisterEvent = function() {
  return jspb.Message.getField(this, 22) != null;
};


/**
 * optional UIDiscoverDatabaseConfigureMTLSEvent ui_discover_database_configure_mtls_event = 23;
 * @return {?proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverDatabaseConfigureMtlsEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent, 23));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverDatabaseConfigureMTLSEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverDatabaseConfigureMtlsEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 23, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverDatabaseConfigureMtlsEvent = function() {
  return this.setUiDiscoverDatabaseConfigureMtlsEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverDatabaseConfigureMtlsEvent = function() {
  return jspb.Message.getField(this, 23) != null;
};


/**
 * optional UIDiscoverDesktopActiveDirectoryToolsInstallEvent ui_discover_desktop_active_directory_tools_install_event = 24;
 * @return {?proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverDesktopActiveDirectoryToolsInstallEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent, 24));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryToolsInstallEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverDesktopActiveDirectoryToolsInstallEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 24, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverDesktopActiveDirectoryToolsInstallEvent = function() {
  return this.setUiDiscoverDesktopActiveDirectoryToolsInstallEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverDesktopActiveDirectoryToolsInstallEvent = function() {
  return jspb.Message.getField(this, 24) != null;
};


/**
 * optional UIDiscoverDesktopActiveDirectoryConfigureEvent ui_discover_desktop_active_directory_configure_event = 25;
 * @return {?proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverDesktopActiveDirectoryConfigureEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent, 25));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverDesktopActiveDirectoryConfigureEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverDesktopActiveDirectoryConfigureEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 25, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverDesktopActiveDirectoryConfigureEvent = function() {
  return this.setUiDiscoverDesktopActiveDirectoryConfigureEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverDesktopActiveDirectoryConfigureEvent = function() {
  return jspb.Message.getField(this, 25) != null;
};


/**
 * optional UIDiscoverAutoDiscoveredResourcesEvent ui_discover_auto_discovered_resources_event = 26;
 * @return {?proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverAutoDiscoveredResourcesEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent, 26));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverAutoDiscoveredResourcesEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverAutoDiscoveredResourcesEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 26, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverAutoDiscoveredResourcesEvent = function() {
  return this.setUiDiscoverAutoDiscoveredResourcesEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverAutoDiscoveredResourcesEvent = function() {
  return jspb.Message.getField(this, 26) != null;
};


/**
 * optional UIDiscoverDatabaseConfigureIAMPolicyEvent ui_discover_database_configure_iam_policy_event = 27;
 * @return {?proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverDatabaseConfigureIamPolicyEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent, 27));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverDatabaseConfigureIAMPolicyEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverDatabaseConfigureIamPolicyEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 27, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverDatabaseConfigureIamPolicyEvent = function() {
  return this.setUiDiscoverDatabaseConfigureIamPolicyEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverDatabaseConfigureIamPolicyEvent = function() {
  return jspb.Message.getField(this, 27) != null;
};


/**
 * optional UIDiscoverPrincipalsConfigureEvent ui_discover_principals_configure_event = 28;
 * @return {?proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverPrincipalsConfigureEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent, 28));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverPrincipalsConfigureEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverPrincipalsConfigureEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 28, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverPrincipalsConfigureEvent = function() {
  return this.setUiDiscoverPrincipalsConfigureEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverPrincipalsConfigureEvent = function() {
  return jspb.Message.getField(this, 28) != null;
};


/**
 * optional UIDiscoverTestConnectionEvent ui_discover_test_connection_event = 29;
 * @return {?proto.prehog.v1alpha.UIDiscoverTestConnectionEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverTestConnectionEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverTestConnectionEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverTestConnectionEvent, 29));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverTestConnectionEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverTestConnectionEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 29, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverTestConnectionEvent = function() {
  return this.setUiDiscoverTestConnectionEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverTestConnectionEvent = function() {
  return jspb.Message.getField(this, 29) != null;
};


/**
 * optional UIDiscoverCompletedEvent ui_discover_completed_event = 30;
 * @return {?proto.prehog.v1alpha.UIDiscoverCompletedEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverCompletedEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverCompletedEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverCompletedEvent, 30));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverCompletedEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverCompletedEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 30, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverCompletedEvent = function() {
  return this.setUiDiscoverCompletedEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverCompletedEvent = function() {
  return jspb.Message.getField(this, 30) != null;
};


/**
 * optional RoleCreateEvent role_create = 31;
 * @return {?proto.prehog.v1alpha.RoleCreateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getRoleCreate = function() {
  return /** @type{?proto.prehog.v1alpha.RoleCreateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.RoleCreateEvent, 31));
};


/**
 * @param {?proto.prehog.v1alpha.RoleCreateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setRoleCreate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 31, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearRoleCreate = function() {
  return this.setRoleCreate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasRoleCreate = function() {
  return jspb.Message.getField(this, 31) != null;
};


/**
 * optional UICreateNewRoleClickEvent ui_create_new_role_click = 32;
 * @return {?proto.prehog.v1alpha.UICreateNewRoleClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiCreateNewRoleClick = function() {
  return /** @type{?proto.prehog.v1alpha.UICreateNewRoleClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UICreateNewRoleClickEvent, 32));
};


/**
 * @param {?proto.prehog.v1alpha.UICreateNewRoleClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiCreateNewRoleClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 32, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiCreateNewRoleClick = function() {
  return this.setUiCreateNewRoleClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiCreateNewRoleClick = function() {
  return jspb.Message.getField(this, 32) != null;
};


/**
 * optional UICreateNewRoleSaveClickEvent ui_create_new_role_save_click = 33;
 * @return {?proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiCreateNewRoleSaveClick = function() {
  return /** @type{?proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent, 33));
};


/**
 * @param {?proto.prehog.v1alpha.UICreateNewRoleSaveClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiCreateNewRoleSaveClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 33, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiCreateNewRoleSaveClick = function() {
  return this.setUiCreateNewRoleSaveClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiCreateNewRoleSaveClick = function() {
  return jspb.Message.getField(this, 33) != null;
};


/**
 * optional UICreateNewRoleCancelClickEvent ui_create_new_role_cancel_click = 34;
 * @return {?proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiCreateNewRoleCancelClick = function() {
  return /** @type{?proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent, 34));
};


/**
 * @param {?proto.prehog.v1alpha.UICreateNewRoleCancelClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiCreateNewRoleCancelClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 34, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiCreateNewRoleCancelClick = function() {
  return this.setUiCreateNewRoleCancelClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiCreateNewRoleCancelClick = function() {
  return jspb.Message.getField(this, 34) != null;
};


/**
 * optional UICreateNewRoleViewDocumentationClickEvent ui_create_new_role_view_documentation_click = 35;
 * @return {?proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiCreateNewRoleViewDocumentationClick = function() {
  return /** @type{?proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent, 35));
};


/**
 * @param {?proto.prehog.v1alpha.UICreateNewRoleViewDocumentationClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiCreateNewRoleViewDocumentationClick = function(value) {
  return jspb.Message.setOneofWrapperField(this, 35, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiCreateNewRoleViewDocumentationClick = function() {
  return this.setUiCreateNewRoleViewDocumentationClick(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiCreateNewRoleViewDocumentationClick = function() {
  return jspb.Message.getField(this, 35) != null;
};


/**
 * optional KubeRequestEvent kube_request = 36;
 * @return {?proto.prehog.v1alpha.KubeRequestEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getKubeRequest = function() {
  return /** @type{?proto.prehog.v1alpha.KubeRequestEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.KubeRequestEvent, 36));
};


/**
 * @param {?proto.prehog.v1alpha.KubeRequestEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setKubeRequest = function(value) {
  return jspb.Message.setOneofWrapperField(this, 36, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearKubeRequest = function() {
  return this.setKubeRequest(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasKubeRequest = function() {
  return jspb.Message.getField(this, 36) != null;
};


/**
 * optional SFTPEvent sftp = 37;
 * @return {?proto.prehog.v1alpha.SFTPEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getSftp = function() {
  return /** @type{?proto.prehog.v1alpha.SFTPEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.SFTPEvent, 37));
};


/**
 * @param {?proto.prehog.v1alpha.SFTPEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setSftp = function(value) {
  return jspb.Message.setOneofWrapperField(this, 37, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearSftp = function() {
  return this.setSftp(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasSftp = function() {
  return jspb.Message.getField(this, 37) != null;
};


/**
 * optional AgentMetadataEvent agent_metadata_event = 38;
 * @return {?proto.prehog.v1alpha.AgentMetadataEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAgentMetadataEvent = function() {
  return /** @type{?proto.prehog.v1alpha.AgentMetadataEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AgentMetadataEvent, 38));
};


/**
 * @param {?proto.prehog.v1alpha.AgentMetadataEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAgentMetadataEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 38, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAgentMetadataEvent = function() {
  return this.setAgentMetadataEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAgentMetadataEvent = function() {
  return jspb.Message.getField(this, 38) != null;
};


/**
 * optional ResourceHeartbeatEvent resource_heartbeat = 39;
 * @return {?proto.prehog.v1alpha.ResourceHeartbeatEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getResourceHeartbeat = function() {
  return /** @type{?proto.prehog.v1alpha.ResourceHeartbeatEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.ResourceHeartbeatEvent, 39));
};


/**
 * @param {?proto.prehog.v1alpha.ResourceHeartbeatEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setResourceHeartbeat = function(value) {
  return jspb.Message.setOneofWrapperField(this, 39, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearResourceHeartbeat = function() {
  return this.setResourceHeartbeat(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasResourceHeartbeat = function() {
  return jspb.Message.getField(this, 39) != null;
};


/**
 * optional UIDiscoverIntegrationAWSOIDCConnectEvent ui_discover_integration_aws_oidc_connect_event = 40;
 * @return {?proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverIntegrationAwsOidcConnectEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent, 40));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverIntegrationAWSOIDCConnectEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverIntegrationAwsOidcConnectEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 40, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverIntegrationAwsOidcConnectEvent = function() {
  return this.setUiDiscoverIntegrationAwsOidcConnectEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverIntegrationAwsOidcConnectEvent = function() {
  return jspb.Message.getField(this, 40) != null;
};


/**
 * optional UIDiscoverDatabaseRDSEnrollEvent ui_discover_database_rds_enroll_event = 41;
 * @return {?proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverDatabaseRdsEnrollEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent, 41));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverDatabaseRDSEnrollEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverDatabaseRdsEnrollEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 41, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverDatabaseRdsEnrollEvent = function() {
  return this.setUiDiscoverDatabaseRdsEnrollEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverDatabaseRdsEnrollEvent = function() {
  return jspb.Message.getField(this, 41) != null;
};


/**
 * optional UICallToActionClickEvent ui_call_to_action_click_event = 42;
 * @return {?proto.prehog.v1alpha.UICallToActionClickEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiCallToActionClickEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UICallToActionClickEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UICallToActionClickEvent, 42));
};


/**
 * @param {?proto.prehog.v1alpha.UICallToActionClickEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiCallToActionClickEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 42, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiCallToActionClickEvent = function() {
  return this.setUiCallToActionClickEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiCallToActionClickEvent = function() {
  return jspb.Message.getField(this, 42) != null;
};


/**
 * optional AssistCompletionEvent assist_completion = 43;
 * @return {?proto.prehog.v1alpha.AssistCompletionEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAssistCompletion = function() {
  return /** @type{?proto.prehog.v1alpha.AssistCompletionEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AssistCompletionEvent, 43));
};


/**
 * @param {?proto.prehog.v1alpha.AssistCompletionEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAssistCompletion = function(value) {
  return jspb.Message.setOneofWrapperField(this, 43, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAssistCompletion = function() {
  return this.setAssistCompletion(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAssistCompletion = function() {
  return jspb.Message.getField(this, 43) != null;
};


/**
 * optional UIIntegrationEnrollStartEvent ui_integration_enroll_start_event = 44;
 * @return {?proto.prehog.v1alpha.UIIntegrationEnrollStartEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiIntegrationEnrollStartEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIIntegrationEnrollStartEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIIntegrationEnrollStartEvent, 44));
};


/**
 * @param {?proto.prehog.v1alpha.UIIntegrationEnrollStartEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiIntegrationEnrollStartEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 44, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiIntegrationEnrollStartEvent = function() {
  return this.setUiIntegrationEnrollStartEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiIntegrationEnrollStartEvent = function() {
  return jspb.Message.getField(this, 44) != null;
};


/**
 * optional UIIntegrationEnrollCompleteEvent ui_integration_enroll_complete_event = 45;
 * @return {?proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiIntegrationEnrollCompleteEvent = function() {
  return /** @type{?proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent, 45));
};


/**
 * @param {?proto.prehog.v1alpha.UIIntegrationEnrollCompleteEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiIntegrationEnrollCompleteEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 45, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiIntegrationEnrollCompleteEvent = function() {
  return this.setUiIntegrationEnrollCompleteEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiIntegrationEnrollCompleteEvent = function() {
  return jspb.Message.getField(this, 45) != null;
};


/**
 * optional EditorChangeEvent editor_change_event = 46;
 * @return {?proto.prehog.v1alpha.EditorChangeEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getEditorChangeEvent = function() {
  return /** @type{?proto.prehog.v1alpha.EditorChangeEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.EditorChangeEvent, 46));
};


/**
 * @param {?proto.prehog.v1alpha.EditorChangeEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setEditorChangeEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 46, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearEditorChangeEvent = function() {
  return this.setEditorChangeEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasEditorChangeEvent = function() {
  return jspb.Message.getField(this, 46) != null;
};


/**
 * optional BotCreateEvent bot_create = 47;
 * @return {?proto.prehog.v1alpha.BotCreateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getBotCreate = function() {
  return /** @type{?proto.prehog.v1alpha.BotCreateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.BotCreateEvent, 47));
};


/**
 * @param {?proto.prehog.v1alpha.BotCreateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setBotCreate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 47, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearBotCreate = function() {
  return this.setBotCreate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasBotCreate = function() {
  return jspb.Message.getField(this, 47) != null;
};


/**
 * optional UIOnboardQuestionnaireSubmitEvent ui_onboard_questionnaire_submit = 48;
 * @return {?proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiOnboardQuestionnaireSubmit = function() {
  return /** @type{?proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent, 48));
};


/**
 * @param {?proto.prehog.v1alpha.UIOnboardQuestionnaireSubmitEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiOnboardQuestionnaireSubmit = function(value) {
  return jspb.Message.setOneofWrapperField(this, 48, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiOnboardQuestionnaireSubmit = function() {
  return this.setUiOnboardQuestionnaireSubmit(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiOnboardQuestionnaireSubmit = function() {
  return jspb.Message.getField(this, 48) != null;
};


/**
 * optional BotJoinEvent bot_join = 49;
 * @return {?proto.prehog.v1alpha.BotJoinEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getBotJoin = function() {
  return /** @type{?proto.prehog.v1alpha.BotJoinEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.BotJoinEvent, 49));
};


/**
 * @param {?proto.prehog.v1alpha.BotJoinEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setBotJoin = function(value) {
  return jspb.Message.setOneofWrapperField(this, 49, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearBotJoin = function() {
  return this.setBotJoin(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasBotJoin = function() {
  return jspb.Message.getField(this, 49) != null;
};


/**
 * optional AssistExecutionEvent assist_execution = 50;
 * @return {?proto.prehog.v1alpha.AssistExecutionEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAssistExecution = function() {
  return /** @type{?proto.prehog.v1alpha.AssistExecutionEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AssistExecutionEvent, 50));
};


/**
 * @param {?proto.prehog.v1alpha.AssistExecutionEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAssistExecution = function(value) {
  return jspb.Message.setOneofWrapperField(this, 50, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAssistExecution = function() {
  return this.setAssistExecution(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAssistExecution = function() {
  return jspb.Message.getField(this, 50) != null;
};


/**
 * optional AssistNewConversationEvent assist_new_conversation = 51;
 * @return {?proto.prehog.v1alpha.AssistNewConversationEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAssistNewConversation = function() {
  return /** @type{?proto.prehog.v1alpha.AssistNewConversationEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AssistNewConversationEvent, 51));
};


/**
 * @param {?proto.prehog.v1alpha.AssistNewConversationEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAssistNewConversation = function(value) {
  return jspb.Message.setOneofWrapperField(this, 51, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAssistNewConversation = function() {
  return this.setAssistNewConversation(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAssistNewConversation = function() {
  return jspb.Message.getField(this, 51) != null;
};


/**
 * optional DeviceAuthenticateEvent device_authenticate_event = 52;
 * @return {?proto.prehog.v1alpha.DeviceAuthenticateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getDeviceAuthenticateEvent = function() {
  return /** @type{?proto.prehog.v1alpha.DeviceAuthenticateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DeviceAuthenticateEvent, 52));
};


/**
 * @param {?proto.prehog.v1alpha.DeviceAuthenticateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setDeviceAuthenticateEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 52, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearDeviceAuthenticateEvent = function() {
  return this.setDeviceAuthenticateEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasDeviceAuthenticateEvent = function() {
  return jspb.Message.getField(this, 52) != null;
};


/**
 * optional FeatureRecommendationEvent feature_recommendation_event = 53;
 * @return {?proto.prehog.v1alpha.FeatureRecommendationEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getFeatureRecommendationEvent = function() {
  return /** @type{?proto.prehog.v1alpha.FeatureRecommendationEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.FeatureRecommendationEvent, 53));
};


/**
 * @param {?proto.prehog.v1alpha.FeatureRecommendationEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setFeatureRecommendationEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 53, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearFeatureRecommendationEvent = function() {
  return this.setFeatureRecommendationEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasFeatureRecommendationEvent = function() {
  return jspb.Message.getField(this, 53) != null;
};


/**
 * optional AssistAccessRequestEvent assist_access_request = 54;
 * @return {?proto.prehog.v1alpha.AssistAccessRequestEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAssistAccessRequest = function() {
  return /** @type{?proto.prehog.v1alpha.AssistAccessRequestEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AssistAccessRequestEvent, 54));
};


/**
 * @param {?proto.prehog.v1alpha.AssistAccessRequestEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAssistAccessRequest = function(value) {
  return jspb.Message.setOneofWrapperField(this, 54, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAssistAccessRequest = function() {
  return this.setAssistAccessRequest(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAssistAccessRequest = function() {
  return jspb.Message.getField(this, 54) != null;
};


/**
 * optional AssistActionEvent assist_action = 55;
 * @return {?proto.prehog.v1alpha.AssistActionEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAssistAction = function() {
  return /** @type{?proto.prehog.v1alpha.AssistActionEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AssistActionEvent, 55));
};


/**
 * @param {?proto.prehog.v1alpha.AssistActionEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAssistAction = function(value) {
  return jspb.Message.setOneofWrapperField(this, 55, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAssistAction = function() {
  return this.setAssistAction(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAssistAction = function() {
  return jspb.Message.getField(this, 55) != null;
};


/**
 * optional DeviceEnrollEvent device_enroll_event = 56;
 * @return {?proto.prehog.v1alpha.DeviceEnrollEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getDeviceEnrollEvent = function() {
  return /** @type{?proto.prehog.v1alpha.DeviceEnrollEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DeviceEnrollEvent, 56));
};


/**
 * @param {?proto.prehog.v1alpha.DeviceEnrollEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setDeviceEnrollEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 56, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearDeviceEnrollEvent = function() {
  return this.setDeviceEnrollEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasDeviceEnrollEvent = function() {
  return jspb.Message.getField(this, 56) != null;
};


/**
 * optional LicenseLimitEvent license_limit_event = 57;
 * @return {?proto.prehog.v1alpha.LicenseLimitEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getLicenseLimitEvent = function() {
  return /** @type{?proto.prehog.v1alpha.LicenseLimitEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.LicenseLimitEvent, 57));
};


/**
 * @param {?proto.prehog.v1alpha.LicenseLimitEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setLicenseLimitEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 57, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearLicenseLimitEvent = function() {
  return this.setLicenseLimitEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasLicenseLimitEvent = function() {
  return jspb.Message.getField(this, 57) != null;
};


/**
 * optional AccessListCreateEvent access_list_create = 58;
 * @return {?proto.prehog.v1alpha.AccessListCreateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListCreate = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListCreateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListCreateEvent, 58));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListCreateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListCreate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 58, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListCreate = function() {
  return this.setAccessListCreate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListCreate = function() {
  return jspb.Message.getField(this, 58) != null;
};


/**
 * optional AccessListUpdateEvent access_list_update = 59;
 * @return {?proto.prehog.v1alpha.AccessListUpdateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListUpdate = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListUpdateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListUpdateEvent, 59));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListUpdateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListUpdate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 59, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListUpdate = function() {
  return this.setAccessListUpdate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListUpdate = function() {
  return jspb.Message.getField(this, 59) != null;
};


/**
 * optional AccessListDeleteEvent access_list_delete = 60;
 * @return {?proto.prehog.v1alpha.AccessListDeleteEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListDelete = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListDeleteEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListDeleteEvent, 60));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListDeleteEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListDelete = function(value) {
  return jspb.Message.setOneofWrapperField(this, 60, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListDelete = function() {
  return this.setAccessListDelete(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListDelete = function() {
  return jspb.Message.getField(this, 60) != null;
};


/**
 * optional AccessListMemberCreateEvent access_list_member_create = 61;
 * @return {?proto.prehog.v1alpha.AccessListMemberCreateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListMemberCreate = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMemberCreateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMemberCreateEvent, 61));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMemberCreateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListMemberCreate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 61, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListMemberCreate = function() {
  return this.setAccessListMemberCreate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListMemberCreate = function() {
  return jspb.Message.getField(this, 61) != null;
};


/**
 * optional AccessListMemberUpdateEvent access_list_member_update = 62;
 * @return {?proto.prehog.v1alpha.AccessListMemberUpdateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListMemberUpdate = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMemberUpdateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMemberUpdateEvent, 62));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMemberUpdateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListMemberUpdate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 62, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListMemberUpdate = function() {
  return this.setAccessListMemberUpdate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListMemberUpdate = function() {
  return jspb.Message.getField(this, 62) != null;
};


/**
 * optional AccessListMemberDeleteEvent access_list_member_delete = 63;
 * @return {?proto.prehog.v1alpha.AccessListMemberDeleteEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListMemberDelete = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListMemberDeleteEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListMemberDeleteEvent, 63));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListMemberDeleteEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListMemberDelete = function(value) {
  return jspb.Message.setOneofWrapperField(this, 63, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListMemberDelete = function() {
  return this.setAccessListMemberDelete(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListMemberDelete = function() {
  return jspb.Message.getField(this, 63) != null;
};


/**
 * optional AccessListGrantsToUserEvent access_list_grants_to_user = 64;
 * @return {?proto.prehog.v1alpha.AccessListGrantsToUserEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListGrantsToUser = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListGrantsToUserEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListGrantsToUserEvent, 64));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListGrantsToUserEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListGrantsToUser = function(value) {
  return jspb.Message.setOneofWrapperField(this, 64, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListGrantsToUser = function() {
  return this.setAccessListGrantsToUser(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListGrantsToUser = function() {
  return jspb.Message.getField(this, 64) != null;
};


/**
 * optional UIDiscoverEC2InstanceSelectionEvent ui_discover_ec2_instance_selection = 65;
 * @return {?proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverEc2InstanceSelection = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent, 65));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverEC2InstanceSelectionEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverEc2InstanceSelection = function(value) {
  return jspb.Message.setOneofWrapperField(this, 65, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverEc2InstanceSelection = function() {
  return this.setUiDiscoverEc2InstanceSelection(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverEc2InstanceSelection = function() {
  return jspb.Message.getField(this, 65) != null;
};


/**
 * optional UIDiscoverDeployEICEEvent ui_discover_deploy_eice = 66;
 * @return {?proto.prehog.v1alpha.UIDiscoverDeployEICEEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverDeployEice = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverDeployEICEEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverDeployEICEEvent, 66));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverDeployEICEEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverDeployEice = function(value) {
  return jspb.Message.setOneofWrapperField(this, 66, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverDeployEice = function() {
  return this.setUiDiscoverDeployEice(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverDeployEice = function() {
  return jspb.Message.getField(this, 66) != null;
};


/**
 * optional UIDiscoverCreateNodeEvent ui_discover_create_node = 67;
 * @return {?proto.prehog.v1alpha.UIDiscoverCreateNodeEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getUiDiscoverCreateNode = function() {
  return /** @type{?proto.prehog.v1alpha.UIDiscoverCreateNodeEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.UIDiscoverCreateNodeEvent, 67));
};


/**
 * @param {?proto.prehog.v1alpha.UIDiscoverCreateNodeEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setUiDiscoverCreateNode = function(value) {
  return jspb.Message.setOneofWrapperField(this, 67, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearUiDiscoverCreateNode = function() {
  return this.setUiDiscoverCreateNode(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasUiDiscoverCreateNode = function() {
  return jspb.Message.getField(this, 67) != null;
};


/**
 * optional DesktopDirectoryShareEvent desktop_directory_share = 68;
 * @return {?proto.prehog.v1alpha.DesktopDirectoryShareEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getDesktopDirectoryShare = function() {
  return /** @type{?proto.prehog.v1alpha.DesktopDirectoryShareEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DesktopDirectoryShareEvent, 68));
};


/**
 * @param {?proto.prehog.v1alpha.DesktopDirectoryShareEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setDesktopDirectoryShare = function(value) {
  return jspb.Message.setOneofWrapperField(this, 68, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearDesktopDirectoryShare = function() {
  return this.setDesktopDirectoryShare(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasDesktopDirectoryShare = function() {
  return jspb.Message.getField(this, 68) != null;
};


/**
 * optional DesktopClipboardEvent desktop_clipboard_transfer = 69;
 * @return {?proto.prehog.v1alpha.DesktopClipboardEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getDesktopClipboardTransfer = function() {
  return /** @type{?proto.prehog.v1alpha.DesktopClipboardEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DesktopClipboardEvent, 69));
};


/**
 * @param {?proto.prehog.v1alpha.DesktopClipboardEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setDesktopClipboardTransfer = function(value) {
  return jspb.Message.setOneofWrapperField(this, 69, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearDesktopClipboardTransfer = function() {
  return this.setDesktopClipboardTransfer(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasDesktopClipboardTransfer = function() {
  return jspb.Message.getField(this, 69) != null;
};


/**
 * optional TAGExecuteQueryEvent tag_execute_query = 70;
 * @return {?proto.prehog.v1alpha.TAGExecuteQueryEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getTagExecuteQuery = function() {
  return /** @type{?proto.prehog.v1alpha.TAGExecuteQueryEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.TAGExecuteQueryEvent, 70));
};


/**
 * @param {?proto.prehog.v1alpha.TAGExecuteQueryEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setTagExecuteQuery = function(value) {
  return jspb.Message.setOneofWrapperField(this, 70, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearTagExecuteQuery = function() {
  return this.setTagExecuteQuery(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasTagExecuteQuery = function() {
  return jspb.Message.getField(this, 70) != null;
};


/**
 * optional ExternalAuditStorageAuthenticateEvent external_audit_storage_authenticate = 71;
 * @return {?proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getExternalAuditStorageAuthenticate = function() {
  return /** @type{?proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent, 71));
};


/**
 * @param {?proto.prehog.v1alpha.ExternalAuditStorageAuthenticateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setExternalAuditStorageAuthenticate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 71, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearExternalAuditStorageAuthenticate = function() {
  return this.setExternalAuditStorageAuthenticate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasExternalAuditStorageAuthenticate = function() {
  return jspb.Message.getField(this, 71) != null;
};


/**
 * optional SecurityReportGetResultEvent security_report_get_result = 72;
 * @return {?proto.prehog.v1alpha.SecurityReportGetResultEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getSecurityReportGetResult = function() {
  return /** @type{?proto.prehog.v1alpha.SecurityReportGetResultEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.SecurityReportGetResultEvent, 72));
};


/**
 * @param {?proto.prehog.v1alpha.SecurityReportGetResultEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setSecurityReportGetResult = function(value) {
  return jspb.Message.setOneofWrapperField(this, 72, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearSecurityReportGetResult = function() {
  return this.setSecurityReportGetResult(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasSecurityReportGetResult = function() {
  return jspb.Message.getField(this, 72) != null;
};


/**
 * optional AuditQueryRunEvent audit_query_run = 73;
 * @return {?proto.prehog.v1alpha.AuditQueryRunEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAuditQueryRun = function() {
  return /** @type{?proto.prehog.v1alpha.AuditQueryRunEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AuditQueryRunEvent, 73));
};


/**
 * @param {?proto.prehog.v1alpha.AuditQueryRunEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAuditQueryRun = function(value) {
  return jspb.Message.setOneofWrapperField(this, 73, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAuditQueryRun = function() {
  return this.setAuditQueryRun(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAuditQueryRun = function() {
  return jspb.Message.getField(this, 73) != null;
};


/**
 * optional DiscoveryFetchEvent discovery_fetch_event = 74;
 * @return {?proto.prehog.v1alpha.DiscoveryFetchEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getDiscoveryFetchEvent = function() {
  return /** @type{?proto.prehog.v1alpha.DiscoveryFetchEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.DiscoveryFetchEvent, 74));
};


/**
 * @param {?proto.prehog.v1alpha.DiscoveryFetchEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setDiscoveryFetchEvent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 74, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearDiscoveryFetchEvent = function() {
  return this.setDiscoveryFetchEvent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasDiscoveryFetchEvent = function() {
  return jspb.Message.getField(this, 74) != null;
};


/**
 * optional AccessListReviewCreateEvent access_list_review_create = 75;
 * @return {?proto.prehog.v1alpha.AccessListReviewCreateEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListReviewCreate = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListReviewCreateEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListReviewCreateEvent, 75));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListReviewCreateEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListReviewCreate = function(value) {
  return jspb.Message.setOneofWrapperField(this, 75, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListReviewCreate = function() {
  return this.setAccessListReviewCreate(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListReviewCreate = function() {
  return jspb.Message.getField(this, 75) != null;
};


/**
 * optional AccessListReviewDeleteEvent access_list_review_delete = 76;
 * @return {?proto.prehog.v1alpha.AccessListReviewDeleteEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListReviewDelete = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListReviewDeleteEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListReviewDeleteEvent, 76));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListReviewDeleteEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListReviewDelete = function(value) {
  return jspb.Message.setOneofWrapperField(this, 76, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListReviewDelete = function() {
  return this.setAccessListReviewDelete(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListReviewDelete = function() {
  return jspb.Message.getField(this, 76) != null;
};


/**
 * optional AccessListReviewComplianceEvent access_list_review_compliance = 77;
 * @return {?proto.prehog.v1alpha.AccessListReviewComplianceEvent}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.getAccessListReviewCompliance = function() {
  return /** @type{?proto.prehog.v1alpha.AccessListReviewComplianceEvent} */ (
    jspb.Message.getWrapperField(this, proto.prehog.v1alpha.AccessListReviewComplianceEvent, 77));
};


/**
 * @param {?proto.prehog.v1alpha.AccessListReviewComplianceEvent|undefined} value
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventRequest.prototype.setAccessListReviewCompliance = function(value) {
  return jspb.Message.setOneofWrapperField(this, 77, proto.prehog.v1alpha.SubmitEventRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.prehog.v1alpha.SubmitEventRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.clearAccessListReviewCompliance = function() {
  return this.setAccessListReviewCompliance(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.prehog.v1alpha.SubmitEventRequest.prototype.hasAccessListReviewCompliance = function() {
  return jspb.Message.getField(this, 77) != null;
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
proto.prehog.v1alpha.SubmitEventResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SubmitEventResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SubmitEventResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SubmitEventResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.prehog.v1alpha.SubmitEventResponse}
 */
proto.prehog.v1alpha.SubmitEventResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SubmitEventResponse;
  return proto.prehog.v1alpha.SubmitEventResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SubmitEventResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SubmitEventResponse}
 */
proto.prehog.v1alpha.SubmitEventResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.prehog.v1alpha.SubmitEventResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SubmitEventResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SubmitEventResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SubmitEventResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.prehog.v1alpha.SubmitEventsRequest.repeatedFields_ = [1];



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
proto.prehog.v1alpha.SubmitEventsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SubmitEventsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SubmitEventsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SubmitEventsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    eventsList: jspb.Message.toObjectList(msg.getEventsList(),
    proto.prehog.v1alpha.SubmitEventRequest.toObject, includeInstance)
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
 * @return {!proto.prehog.v1alpha.SubmitEventsRequest}
 */
proto.prehog.v1alpha.SubmitEventsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SubmitEventsRequest;
  return proto.prehog.v1alpha.SubmitEventsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SubmitEventsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SubmitEventsRequest}
 */
proto.prehog.v1alpha.SubmitEventsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.prehog.v1alpha.SubmitEventRequest;
      reader.readMessage(value,proto.prehog.v1alpha.SubmitEventRequest.deserializeBinaryFromReader);
      msg.addEvents(value);
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
proto.prehog.v1alpha.SubmitEventsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SubmitEventsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SubmitEventsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SubmitEventsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getEventsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.prehog.v1alpha.SubmitEventRequest.serializeBinaryToWriter
    );
  }
};


/**
 * repeated SubmitEventRequest events = 1;
 * @return {!Array<!proto.prehog.v1alpha.SubmitEventRequest>}
 */
proto.prehog.v1alpha.SubmitEventsRequest.prototype.getEventsList = function() {
  return /** @type{!Array<!proto.prehog.v1alpha.SubmitEventRequest>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.prehog.v1alpha.SubmitEventRequest, 1));
};


/**
 * @param {!Array<!proto.prehog.v1alpha.SubmitEventRequest>} value
 * @return {!proto.prehog.v1alpha.SubmitEventsRequest} returns this
*/
proto.prehog.v1alpha.SubmitEventsRequest.prototype.setEventsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.prehog.v1alpha.SubmitEventRequest=} opt_value
 * @param {number=} opt_index
 * @return {!proto.prehog.v1alpha.SubmitEventRequest}
 */
proto.prehog.v1alpha.SubmitEventsRequest.prototype.addEvents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.prehog.v1alpha.SubmitEventRequest, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.prehog.v1alpha.SubmitEventsRequest} returns this
 */
proto.prehog.v1alpha.SubmitEventsRequest.prototype.clearEventsList = function() {
  return this.setEventsList([]);
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
proto.prehog.v1alpha.SubmitEventsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.SubmitEventsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.SubmitEventsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SubmitEventsResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.prehog.v1alpha.SubmitEventsResponse}
 */
proto.prehog.v1alpha.SubmitEventsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.SubmitEventsResponse;
  return proto.prehog.v1alpha.SubmitEventsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.SubmitEventsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.SubmitEventsResponse}
 */
proto.prehog.v1alpha.SubmitEventsResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.prehog.v1alpha.SubmitEventsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.SubmitEventsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.SubmitEventsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.SubmitEventsResponse.serializeBinaryToWriter = function(message, writer) {
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
proto.prehog.v1alpha.HelloTeleportRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.HelloTeleportRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.HelloTeleportRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.HelloTeleportRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.prehog.v1alpha.HelloTeleportRequest}
 */
proto.prehog.v1alpha.HelloTeleportRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.HelloTeleportRequest;
  return proto.prehog.v1alpha.HelloTeleportRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.HelloTeleportRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.HelloTeleportRequest}
 */
proto.prehog.v1alpha.HelloTeleportRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.prehog.v1alpha.HelloTeleportRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.HelloTeleportRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.HelloTeleportRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.HelloTeleportRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.prehog.v1alpha.HelloTeleportResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.prehog.v1alpha.HelloTeleportResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.prehog.v1alpha.HelloTeleportResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.HelloTeleportResponse.toObject = function(includeInstance, msg) {
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
 * @return {!proto.prehog.v1alpha.HelloTeleportResponse}
 */
proto.prehog.v1alpha.HelloTeleportResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.prehog.v1alpha.HelloTeleportResponse;
  return proto.prehog.v1alpha.HelloTeleportResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.prehog.v1alpha.HelloTeleportResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.prehog.v1alpha.HelloTeleportResponse}
 */
proto.prehog.v1alpha.HelloTeleportResponse.deserializeBinaryFromReader = function(msg, reader) {
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
proto.prehog.v1alpha.HelloTeleportResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.prehog.v1alpha.HelloTeleportResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.prehog.v1alpha.HelloTeleportResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.prehog.v1alpha.HelloTeleportResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};


/**
 * @enum {number}
 */
proto.prehog.v1alpha.ResourceKind = {
  RESOURCE_KIND_UNSPECIFIED: 0,
  RESOURCE_KIND_NODE: 1,
  RESOURCE_KIND_APP_SERVER: 2,
  RESOURCE_KIND_KUBE_SERVER: 3,
  RESOURCE_KIND_DB_SERVER: 4,
  RESOURCE_KIND_WINDOWS_DESKTOP: 5,
  RESOURCE_KIND_NODE_OPENSSH: 6,
  RESOURCE_KIND_NODE_OPENSSH_EICE: 7
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.UserKind = {
  USER_KIND_UNSPECIFIED: 0,
  USER_KIND_HUMAN: 1,
  USER_KIND_BOT: 2
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.DiscoverResource = {
  DISCOVER_RESOURCE_UNSPECIFIED: 0,
  DISCOVER_RESOURCE_SERVER: 1,
  DISCOVER_RESOURCE_KUBERNETES: 2,
  DISCOVER_RESOURCE_DATABASE_POSTGRES_SELF_HOSTED: 3,
  DISCOVER_RESOURCE_DATABASE_MYSQL_SELF_HOSTED: 4,
  DISCOVER_RESOURCE_DATABASE_MONGODB_SELF_HOSTED: 5,
  DISCOVER_RESOURCE_DATABASE_POSTGRES_RDS: 6,
  DISCOVER_RESOURCE_DATABASE_MYSQL_RDS: 7,
  DISCOVER_RESOURCE_APPLICATION_HTTP: 8,
  DISCOVER_RESOURCE_APPLICATION_TCP: 9,
  DISCOVER_RESOURCE_WINDOWS_DESKTOP: 10,
  DISCOVER_RESOURCE_DATABASE_SQLSERVER_RDS: 11,
  DISCOVER_RESOURCE_DATABASE_POSTGRES_REDSHIFT: 12,
  DISCOVER_RESOURCE_DATABASE_SQLSERVER_SELF_HOSTED: 13,
  DISCOVER_RESOURCE_DATABASE_REDIS_SELF_HOSTED: 14,
  DISCOVER_RESOURCE_DATABASE_POSTGRES_GCP: 15,
  DISCOVER_RESOURCE_DATABASE_MYSQL_GCP: 16,
  DISCOVER_RESOURCE_DATABASE_SQLSERVER_GCP: 17,
  DISCOVER_RESOURCE_DATABASE_POSTGRES_REDSHIFT_SERVERLESS: 18,
  DISCOVER_RESOURCE_DATABASE_POSTGRES_AZURE: 19,
  DISCOVER_RESOURCE_DATABASE_DYNAMODB: 20,
  DISCOVER_RESOURCE_DATABASE_CASSANDRA_KEYSPACES: 21,
  DISCOVER_RESOURCE_DATABASE_CASSANDRA_SELF_HOSTED: 22,
  DISCOVER_RESOURCE_DATABASE_ELASTICSEARCH_SELF_HOSTED: 23,
  DISCOVER_RESOURCE_DATABASE_REDIS_ELASTICACHE: 24,
  DISCOVER_RESOURCE_DATABASE_REDIS_MEMORYDB: 25,
  DISCOVER_RESOURCE_DATABASE_REDIS_AZURE_CACHE: 26,
  DISCOVER_RESOURCE_DATABASE_REDIS_CLUSTER_SELF_HOSTED: 27,
  DISCOVER_RESOURCE_DATABASE_MYSQL_AZURE: 28,
  DISCOVER_RESOURCE_DATABASE_SQLSERVER_AZURE: 29,
  DISCOVER_RESOURCE_DATABASE_SQLSERVER_MICROSOFT: 30,
  DISCOVER_RESOURCE_DATABASE_COCKROACHDB_SELF_HOSTED: 31,
  DISCOVER_RESOURCE_DATABASE_MONGODB_ATLAS: 32,
  DISCOVER_RESOURCE_DATABASE_SNOWFLAKE: 33,
  DISCOVER_RESOURCE_DOC_DATABASE_RDS_PROXY: 34,
  DISCOVER_RESOURCE_DOC_DATABASE_HIGH_AVAILABILITY: 35,
  DISCOVER_RESOURCE_DOC_DATABASE_DYNAMIC_REGISTRATION: 36,
  DISCOVER_RESOURCE_SAML_APPLICATION: 37,
  DISCOVER_RESOURCE_EC2_INSTANCE: 38,
  DISCOVER_RESOURCE_DOC_WINDOWS_DESKTOP_NON_AD: 39
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.DiscoverStatus = {
  DISCOVER_STATUS_UNSPECIFIED: 0,
  DISCOVER_STATUS_SUCCESS: 1,
  DISCOVER_STATUS_SKIPPED: 2,
  DISCOVER_STATUS_ERROR: 3,
  DISCOVER_STATUS_ABORTED: 4
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.CTA = {
  CTA_UNSPECIFIED: 0,
  CTA_AUTH_CONNECTOR: 1,
  CTA_ACTIVE_SESSIONS: 2,
  CTA_ACCESS_REQUESTS: 3,
  CTA_PREMIUM_SUPPORT: 4,
  CTA_TRUSTED_DEVICES: 5,
  CTA_UPGRADE_BANNER: 6,
  CTA_BILLING_SUMMARY: 7,
  CTA_ACCESS_LIST: 8,
  CTA_ACCESS_MONITORING: 9,
  CTA_EXTERNAL_AUDIT_STORAGE: 10,
  CTA_OKTA_USER_SYNC: 11
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.IntegrationEnrollKind = {
  INTEGRATION_ENROLL_KIND_UNSPECIFIED: 0,
  INTEGRATION_ENROLL_KIND_SLACK: 1,
  INTEGRATION_ENROLL_KIND_AWS_OIDC: 2,
  INTEGRATION_ENROLL_KIND_PAGERDUTY: 3,
  INTEGRATION_ENROLL_KIND_EMAIL: 4,
  INTEGRATION_ENROLL_KIND_JIRA: 5,
  INTEGRATION_ENROLL_KIND_DISCORD: 6,
  INTEGRATION_ENROLL_KIND_MATTERMOST: 7,
  INTEGRATION_ENROLL_KIND_MS_TEAMS: 8,
  INTEGRATION_ENROLL_KIND_OPSGENIE: 9,
  INTEGRATION_ENROLL_KIND_OKTA: 10,
  INTEGRATION_ENROLL_KIND_JAMF: 11,
  INTEGRATION_ENROLL_KIND_MACHINE_ID: 12,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS: 13,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_CIRCLECI: 14,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_GITLAB: 15,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_JENKINS: 16,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_ANSIBLE: 17,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_AWS: 18,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_GCP: 19,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_AZURE: 20,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_SPACELIFT: 21,
  INTEGRATION_ENROLL_KIND_MACHINE_ID_KUBERNETES: 22
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.EditorChangeStatus = {
  EDITOR_CHANGE_STATUS_UNSPECIFIED: 0,
  EDITOR_CHANGE_STATUS_ROLE_GRANTED: 1,
  EDITOR_CHANGE_STATUS_ROLE_REMOVED: 2
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.Feature = {
  FEATURE_UNSPECIFIED: 0,
  FEATURE_TRUSTED_DEVICES: 1
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.FeatureRecommendationStatus = {
  FEATURE_RECOMMENDATION_STATUS_UNSPECIFIED: 0,
  FEATURE_RECOMMENDATION_STATUS_NOTIFIED: 1,
  FEATURE_RECOMMENDATION_STATUS_DONE: 2
};

/**
 * @enum {number}
 */
proto.prehog.v1alpha.LicenseLimit = {
  LICENSE_LIMIT_UNSPECIFIED: 0,
  LICENSE_LIMIT_DEVICE_TRUST_TEAM_JAMF: 1,
  LICENSE_LIMIT_DEVICE_TRUST_TEAM_USAGE: 2
};

goog.object.extend(exports, proto.prehog.v1alpha);
