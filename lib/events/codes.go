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

package events

import apievents "github.com/gravitational/teleport/api/types/events"

// Event describes an audit log event.
type Event struct {
	// Name is the event name.
	Name string
	// Code is the unique event code.
	Code string
}

// There is no strict algorithm for picking an event code, however existing
// event codes are currently loosely categorized as follows:
//
//   - Teleport event codes start with "T" and belong in this const block.
//
//   - Related events are grouped starting with the same number.
//     eg: All user related events are grouped under 1xxx.
//
//   - Suffix code with one of these letters: I (info), W (warn), E (error).
//
// After defining an event code, make sure to keep
// `web/packages/teleport/src/services/audit/types.ts` in sync and add an
// entry in the `eventsMap` in `lib/events/events_test.go`.
const (
	// UserLocalLoginCode is the successful local user login event code.
	UserLocalLoginCode = "T1000I"
	// UserLocalLoginFailureCode is the unsuccessful local user login event code.
	UserLocalLoginFailureCode = "T1000W"
	// UserSSOLoginCode is the successful SSO user login event code.
	UserSSOLoginCode = "T1001I"
	// UserSSOLoginFailureCode is the unsuccessful SSO user login event code.
	UserSSOLoginFailureCode = "T1001W"
	// UserCreateCode is the user create event code.
	UserCreateCode = "T1002I"
	// UserUpdateCode is the user update event code.
	UserUpdateCode = "T1003I"
	// UserDeleteCode is the user delete event code.
	UserDeleteCode = "T1004I"
	// UserPasswordChangeCode is an event code for when user changes their own password.
	UserPasswordChangeCode = "T1005I"
	// MFADeviceAddEventCode is an event code for users adding MFA devices.
	MFADeviceAddEventCode = "T1006I"
	// MFADeviceDeleteEventCode is an event code for users deleting MFA devices.
	MFADeviceDeleteEventCode = "T1007I"
	// RecoveryCodesGenerateCode is an event code for generation of recovery codes.
	RecoveryCodesGenerateCode = "T1008I"
	// RecoveryCodeUseSuccessCode is an event code for when a
	// recovery code was used successfully.
	RecoveryCodeUseSuccessCode = "T1009I"
	// RecoveryCodeUseFailureCode is an event code for when a
	// recovery code was not used successfully.
	RecoveryCodeUseFailureCode = "T1009W"
	// UserSSOTestFlowLoginCode is the successful SSO test flow user login event code.
	UserSSOTestFlowLoginCode = "T1010I"
	// UserSSOTestFlowLoginFailureCode is the unsuccessful SSO test flow user login event code.
	UserSSOTestFlowLoginFailureCode = "T1011W"
	// UserHeadlessLoginRequestedCode is an event code for when headless login attempt was requested.
	UserHeadlessLoginRequestedCode = "T1012I"
	// UserHeadlessLoginApprovedCode is an event code for when headless login attempt was successfully approved.
	UserHeadlessLoginApprovedCode = "T1013I"
	// UserHeadlessLoginApprovedFailureCode is an event code for when headless login was approved with an error.
	UserHeadlessLoginApprovedFailureCode = "T1013W"
	// UserHeadlessLoginRejectedCode is an event code for when headless login attempt was rejected.
	UserHeadlessLoginRejectedCode = "T1014W"
	// CreateMFAAuthChallenge is an event code for when an MFA auth challenge is created.
	CreateMFAAuthChallengeCode = "T1015I"
	// ValidateMFAAuthResponseCode is an event code for when an MFA auth challenge
	// response is successfully validated.
	ValidateMFAAuthResponseCode = "T1016I"
	// VValidateMFAAuthResponseFailureCode is an event code for when an MFA auth challenge
	// response fails validation.
	ValidateMFAAuthResponseFailureCode = "T1016W"

	// BillingCardCreateCode is an event code for when a user creates a new credit card.
	BillingCardCreateCode = "TBL00I"
	// BillingCardDeleteCode is an event code for when a user deletes a credit card.
	BillingCardDeleteCode = "TBL01I"
	// BillingCardUpdateCode is an event code for when a user updates an existing credit card.
	BillingCardUpdateCode = "TBL02I"
	// BillingInformationUpdateCode is an event code for when a user updates their billing info.
	BillingInformationUpdateCode = "TBL03I"

	// SessionRejectedCode is an event code for when a user's attempt to create an
	// session/connection has been rejected.
	SessionRejectedCode = "T1006W"

	// SessionStartCode is the session start event code.
	SessionStartCode = "T2000I"
	// SessionJoinCode is the session join event code.
	SessionJoinCode = "T2001I"
	// TerminalResizeCode is the terminal resize event code.
	TerminalResizeCode = "T2002I"
	// SessionLeaveCode is the session leave event code.
	SessionLeaveCode = "T2003I"
	// SessionEndCode is the session end event code.
	SessionEndCode = "T2004I"
	// SessionUploadCode is the session upload event code.
	SessionUploadCode = "T2005I"
	// SessionDataCode is the session data event code.
	SessionDataCode = "T2006I"
	// AppSessionStartCode is the application session start code.
	AppSessionStartCode = "T2007I"
	// AppSessionChunkCode is the application session chunk create code.
	AppSessionChunkCode = "T2008I"
	// AppSessionRequestCode is the application request/response code.
	AppSessionRequestCode = "T2009I"
	// SessionConnectCode is the session connect event code.
	SessionConnectCode = "T2010I"
	// AppSessionEndCode is the application session end event code.
	AppSessionEndCode = "T2011I"
	// SessionRecordingAccessCode is the session recording view data event code.
	SessionRecordingAccessCode = "T2012I"
	// AppSessionDynamoDBRequestCode is the application request/response code.
	AppSessionDynamoDBRequestCode = "T2013I"

	// AppCreateCode is the app.create event code.
	AppCreateCode = "TAP03I"
	// AppUpdateCode is the app.update event code.
	AppUpdateCode = "TAP04I"
	// AppDeleteCode is the app.delete event code.
	AppDeleteCode = "TAP05I"

	// DatabaseSessionStartCode is the database session start event code.
	DatabaseSessionStartCode = "TDB00I"
	// DatabaseSessionStartFailureCode is the database session start failure event code.
	DatabaseSessionStartFailureCode = "TDB00W"
	// DatabaseSessionEndCode is the database session end event code.
	DatabaseSessionEndCode = "TDB01I"
	// DatabaseSessionQueryCode is the database query event code.
	DatabaseSessionQueryCode = "TDB02I"
	// DatabaseSessionQueryFailedCode is the database query failure event code.
	DatabaseSessionQueryFailedCode = "TDB02W"
	// DatabaseSessionMalformedPacketCode is the db.session.malformed_packet event code.
	DatabaseSessionMalformedPacketCode = "TDB06I"
	// DatabaseSessionPermissionUpdateCode is the db.session.permissions.update event code.
	DatabaseSessionPermissionUpdateCode = "TDB07I"
	// DatabaseSessionUserCreateCode is the db.session.user.create event code.
	DatabaseSessionUserCreateCode = "TDB08I"
	// DatabaseSessionUserCreateFailureCode is the db.session.user.create event failure code.
	DatabaseSessionUserCreateFailureCode = "TDB08W"
	// DatabaseSessionUserDeactivateCode is the db.session.user.deactivate event code.
	DatabaseSessionUserDeactivateCode = "TDB09I"
	// DatabaseSessionUserDeactivateFailureCode is the db.session.user.deactivate event failure code.
	DatabaseSessionUserDeactivateFailureCode = "TDB09W"
	// DatabaseSessionCommandResultCode is the db.session.result event code.
	DatabaseSessionCommandResultCode = "TDB10I"

	// PostgresParseCode is the db.session.postgres.statements.parse event code.
	PostgresParseCode = "TPG00I"
	// PostgresBindCode is the db.session.postgres.statements.bind event code.
	PostgresBindCode = "TPG01I"
	// PostgresExecuteCode is the db.session.postgres.statements.execute event code.
	PostgresExecuteCode = "TPG02I"
	// PostgresCloseCode is the db.session.postgres.statements.close event code.
	PostgresCloseCode = "TPG03I"
	// PostgresFunctionCallCode is the db.session.postgres.function event code.
	PostgresFunctionCallCode = "TPG04I"

	// MySQLStatementPrepareCode is the db.session.mysql.statements.prepare event code.
	MySQLStatementPrepareCode = "TMY00I"
	// MySQLStatementExecuteCode is the db.session.mysql.statements.execute event code.
	MySQLStatementExecuteCode = "TMY01I"
	// MySQLStatementSendLongDataCode is the db.session.mysql.statements.send_long_data event code.
	MySQLStatementSendLongDataCode = "TMY02I"
	// MySQLStatementCloseCode is the db.session.mysql.statements.close event code.
	MySQLStatementCloseCode = "TMY03I"
	// MySQLStatementResetCode is the db.session.mysql.statements.reset event code.
	MySQLStatementResetCode = "TMY04I"
	// MySQLStatementFetchCode is the db.session.mysql.statements.fetch event code.
	MySQLStatementFetchCode = "TMY05I"
	// MySQLStatementBulkExecuteCode is the db.session.mysql.statements.bulk_execute event code.
	MySQLStatementBulkExecuteCode = "TMY06I"
	// MySQLInitDBCode is the db.session.mysql.init_db event code.
	MySQLInitDBCode = "TMY07I"
	// MySQLCreateDBCode is the db.session.mysql.create_db event code.
	MySQLCreateDBCode = "TMY08I"
	// MySQLDropDBCode is the db.session.mysql.drop_db event code.
	MySQLDropDBCode = "TMY09I"
	// MySQLShutDownCode is the db.session.mysql.shut_down event code.
	MySQLShutDownCode = "TMY10I"
	// MySQLProcessKillCode is the db.session.mysql.process_kill event code.
	MySQLProcessKillCode = "TMY11I"
	// MySQLDebugCode is the db.session.mysql.debug event code.
	MySQLDebugCode = "TMY12I"
	// MySQLRefreshCode is the db.session.mysql.refresh event code.
	MySQLRefreshCode = "TMY13I"

	// SQLServerRPCRequestCode is the db.session.sqlserver.rpc_request event code.
	SQLServerRPCRequestCode = "TMS00I"

	// CassandraBatchEventCode is the db.session.cassandra.batch event code.
	CassandraBatchEventCode = "TCA01I"
	// CassandraPrepareEventCode is the db.session.cassandra.prepare event code.
	CassandraPrepareEventCode = "TCA02I"
	// CassandraExecuteEventCode is the db.session.cassandra.execute event code.
	CassandraExecuteEventCode = "TCA03I"
	// CassandraRegisterEventCode is the db.session.cassandra.register event code.
	CassandraRegisterEventCode = "TCA04I"

	// ElasticsearchRequestCode is the db.session.elasticsearch.request event code.
	ElasticsearchRequestCode = "TES00I"
	// ElasticsearchRequestFailureCode is the db.session.elasticsearch.request event failure code.
	ElasticsearchRequestFailureCode = "TES00E"

	// OpenSearchRequestCode is the db.session.opensearch.request event code.
	OpenSearchRequestCode = "TOS00I"
	// OpenSearchRequestFailureCode is the db.session.opensearch.request event failure code.
	OpenSearchRequestFailureCode = "TOS00E"

	// DynamoDBRequestCode is the db.session.dynamodb.request event code.
	DynamoDBRequestCode = "TDY01I"
	// DynamoDBRequestFailureCode is the db.session.dynamodb.request event failure code.
	// This is indicates that the database agent http transport failed to round trip the request.
	DynamoDBRequestFailureCode = "TDY01E"

	// SpannerRPCCode is the db.session.spanner.rpc event code.
	SpannerRPCCode = "TSPN001I"
	// SpannerRPCDeniedCode is the warning event code for a Spanner client RPC
	// that is denied.
	SpannerRPCDeniedCode = "TSPN001W"

	// DatabaseCreateCode is the db.create event code.
	DatabaseCreateCode = "TDB03I"
	// DatabaseUpdateCode is the db.update event code.
	DatabaseUpdateCode = "TDB04I"
	// DatabaseDeleteCode is the db.delete event code.
	DatabaseDeleteCode = "TDB05I"

	// DesktopSessionStartCode is the desktop session start event code.
	DesktopSessionStartCode = "TDP00I"
	// DesktopSessionStartFailureCode is event code for desktop sessions
	// that failed to start.
	DesktopSessionStartFailureCode = "TDP00W"
	// DesktopSessionEndCode is the desktop session end event code.
	DesktopSessionEndCode = "TDP01I"
	// DesktopClipboardSendCode is the desktop clipboard send code.
	DesktopClipboardSendCode = "TDP02I"
	// DesktopClipboardReceiveCode is the desktop clipboard receive code.
	DesktopClipboardReceiveCode = "TDP03I"
	// DesktopSharedDirectoryStartCode is the desktop directory start code.
	DesktopSharedDirectoryStartCode = "TDP04I"
	// DesktopSharedDirectoryStartFailureCode is the desktop directory start code
	// for when a start operation fails, or for when the internal cache state was corrupted
	// causing information loss, or for when the internal cache has exceeded its max size.
	DesktopSharedDirectoryStartFailureCode = "TDP04W"
	// DesktopSharedDirectoryReadCode is the desktop directory read code.
	DesktopSharedDirectoryReadCode = "TDP05I"
	// DesktopSharedDirectoryReadFailureCode is the desktop directory read code
	// for when a read operation fails, or for if the internal cache state was corrupted
	// causing information loss, or for when the internal cache has exceeded its max size.
	DesktopSharedDirectoryReadFailureCode = "TDP05W"
	// DesktopSharedDirectoryWriteCode is the desktop directory write code.
	DesktopSharedDirectoryWriteCode = "TDP06I"
	// DesktopSharedDirectoryWriteFailureCode is the desktop directory write code
	// for when a write operation fails, or for if the internal cache state was corrupted
	// causing information loss, or for when the internal cache has exceeded its max size.
	DesktopSharedDirectoryWriteFailureCode = "TDP06W"

	// SubsystemCode is the subsystem event code.
	SubsystemCode = "T3001I"
	// SubsystemFailureCode is the subsystem failure event code.
	SubsystemFailureCode = "T3001E"
	// ExecCode is the exec event code.
	ExecCode = "T3002I"
	// ExecFailureCode is the exec failure event code.
	ExecFailureCode = "T3002E"
	// PortForwardCode is the port forward event code.
	PortForwardCode = "T3003I"
	// PortForwardStopCode is the port forward stop event code.
	PortForwardStopCode = "T3003S"
	// PortForwardFailureCode is the port forward failure event code.
	PortForwardFailureCode = "T3003E"
	// SCPDownloadCode is the file download event code.
	SCPDownloadCode = "T3004I"
	// SCPDownloadFailureCode is the file download event failure code.
	SCPDownloadFailureCode = "T3004E"
	// SCPUploadCode is the file upload event code.
	SCPUploadCode = "T3005I"
	// SCPUploadFailureCode is the file upload failure event code.
	SCPUploadFailureCode = "T3005E"
	// ClientDisconnectCode is the client disconnect event code.
	ClientDisconnectCode = "T3006I"
	// AuthAttemptFailureCode is the auth attempt failure event code.
	AuthAttemptFailureCode = "T3007W"
	// X11ForwardCode is the x11 forward event code.
	X11ForwardCode = "T3008I"
	// X11ForwardFailureCode is the x11 forward failure event code.
	X11ForwardFailureCode = "T3008W"
	// KubeRequestCode is an event code for a generic kubernetes request.
	//
	// Note: some requests (like exec into a pod) use other codes (like
	// ExecCode).
	KubeRequestCode = "T3009I"
	// SCPDisallowedCode is the SCP disallowed event code.
	SCPDisallowedCode = "T3010E"

	// KubernetesClusterCreateCode is the kube.create event code.
	KubernetesClusterCreateCode = "T3010I"
	// KubernetesClusterUpdateCode is the kube.update event code.
	KubernetesClusterUpdateCode = "T3011I"
	// KubernetesClusterDeleteCode is the kube.delete event code.
	KubernetesClusterDeleteCode = "T3012I"

	// The following codes correspond to SFTP file operations.
	SFTPOpenCode           = "TS001I"
	SFTPOpenFailureCode    = "TS001E"
	SFTPSetstatCode        = "TS007I"
	SFTPSetstatFailureCode = "TS007E"
	SFTPOpendirCode        = "TS009I"
	SFTPOpendirFailureCode = "TS009E"
	SFTPReaddirCode        = "TS010I"
	SFTPReaddirFailureCode = "TS010E"
	SFTPRemoveCode         = "TS011I"
	SFTPRemoveFailureCode  = "TS011E"
	SFTPMkdirCode          = "TS012I"
	SFTPMkdirFailureCode   = "TS012E"
	SFTPRmdirCode          = "TS013I"
	SFTPRmdirFailureCode   = "TS013E"
	SFTPRenameCode         = "TS016I"
	SFTPRenameFailureCode  = "TS016E"
	SFTPSymlinkCode        = "TS018I"
	SFTPSymlinkFailureCode = "TS018E"
	SFTPLinkCode           = "TS019I"
	SFTPLinkFailureCode    = "TS019E"
	SFTPDisallowedCode     = "TS020E"
	// SFTPSummaryCode is the SFTP summary code.
	SFTPSummaryCode = "TS021I"

	// SessionCommandCode is a session command code.
	SessionCommandCode = "T4000I"
	// SessionDiskCode is a session disk code.
	SessionDiskCode = "T4001I"
	// SessionNetworkCode is a session network code.
	SessionNetworkCode = "T4002I"

	// AccessRequestCreateCode is the access request creation code.
	AccessRequestCreateCode = "T5000I"
	// AccessRequestUpdateCode is the access request state update code.
	AccessRequestUpdateCode = "T5001I"
	// AccessRequestReviewCode is the access review application code.
	AccessRequestReviewCode = "T5002I"
	// AccessRequestDeleteCode is the access request deleted code.
	AccessRequestDeleteCode = "T5003I"
	// AccessRequestResourceSearchCode is the access request resource search code.
	AccessRequestResourceSearchCode = "T5004I"
	// AccessRequestExpireCode is the access request expires code.
	AccessRequestExpireCode = "T5005I"

	// ResetPasswordTokenCreateCode is the token create event code.
	ResetPasswordTokenCreateCode = "T6000I"
	// RecoveryTokenCreateCode is the recovery token create event code.
	RecoveryTokenCreateCode = "T6001I"
	// PrivilegeTokenCreateCode is the privilege token create event code.
	PrivilegeTokenCreateCode = "T6002I"

	// TrustedClusterCreateCode is the event code for creating a trusted cluster.
	TrustedClusterCreateCode = "T7000I"
	// TrustedClusterDeleteCode is the event code for removing a trusted cluster.
	TrustedClusterDeleteCode = "T7001I"
	// TrustedClusterTokenCreateCode is the event code for creating new
	// provisioning token for a trusted cluster. Deprecated in favor of
	// [ProvisionTokenCreateEvent].
	TrustedClusterTokenCreateCode = "T7002I"

	// ProvisionTokenCreateCode is the event code for creating a provisioning
	// token, also known as Join Token. See
	// [github.com/gravitational/teleport/api/types.ProvisionToken].
	ProvisionTokenCreateCode = "TJT00I"

	// GithubConnectorCreatedCode is the Github connector created event code.
	GithubConnectorCreatedCode = "T8000I"
	// GithubConnectorDeletedCode is the Github connector deleted event code.
	GithubConnectorDeletedCode = "T8001I"
	// GithubConnectorUpdatedCode is the Github connector updated event code.
	GithubConnectorUpdatedCode = "T80002I"

	// OIDCConnectorCreatedCode is the OIDC connector created event code.
	OIDCConnectorCreatedCode = "T8100I"
	// OIDCConnectorDeletedCode is the OIDC connector deleted event code.
	OIDCConnectorDeletedCode = "T8101I"
	// OIDCConnectorUpdatedCode is the OIDC connector updated event code.
	OIDCConnectorUpdatedCode = "T8102I"

	// SAMLConnectorCreatedCode is the SAML connector created event code.
	SAMLConnectorCreatedCode = "T8200I"
	// SAMLConnectorDeletedCode is the SAML connector deleted event code.
	SAMLConnectorDeletedCode = "T8201I"
	// SAMLConnectorUpdatedCode is the SAML connector updated event code.
	SAMLConnectorUpdatedCode = "T8202I"

	// RoleCreatedCode is the role created event code.
	RoleCreatedCode = "T9000I"
	// RoleDeletedCode is the role deleted event code.
	RoleDeletedCode = "T9001I"
	// RoleUpdatedCode is the role created event code.
	RoleUpdatedCode = "T9002I"

	// BotJoinCode is the 'bot.join' event code.
	BotJoinCode = "TJ001I"
	// BotJoinFailureCode is the 'bot.join' event code for failures.
	BotJoinFailureCode = "TJ001E"
	// InstanceJoinCode is the 'node.join' event code.
	InstanceJoinCode = "TJ002I"
	// InstanceJoinFailureCode is the 'node.join' event code for failures.
	InstanceJoinFailureCode = "TJ002E"

	// BotCreateCode is the `bot.create` event code.
	BotCreateCode = "TB001I"
	// BotUpdateCode is the `bot.update` event code.
	BotUpdateCode = "TB002I"
	// BotDeleteCode is the `bot.delete` event code.
	BotDeleteCode = "TB003I"

	// LockCreatedCode is the lock created event code.
	LockCreatedCode = "TLK00I"
	// LockDeletedCode is the lock deleted event code.
	LockDeletedCode = "TLK01I"

	// CertificateCreateCode is the certificate issuance event code.
	CertificateCreateCode = "TC000I"

	// RenewableCertificateGenerationMismatchCode is the renewable cert
	// generation mismatch code.
	RenewableCertificateGenerationMismatchCode = "TCB00W"

	// UpgradeWindowStartUpdatedCode is the edit code of UpgradeWindowStartUpdateEvent.
	UpgradeWindowStartUpdatedCode = "TUW01I"

	// SSMRunSuccessCode is the discovery script success code.
	SSMRunSuccessCode = "TDS00I"
	// SSMRunFailCode is the discovery script success code.
	SSMRunFailCode = "TDS00W"

	// DeviceCreateCode is the device creation/registration code.
	DeviceCreateCode = "TV001I"
	// DeviceDeleteCode is the device deletion code.
	DeviceDeleteCode = "TV002I"
	// DeviceEnrollTokenCreateCode is the device enroll token creation code
	DeviceEnrollTokenCreateCode = "TV003I"
	// DeviceEnrollTokenSpentCode is the device enroll token spent code.
	DeviceEnrollTokenSpentCode = "TV004I"
	// DeviceEnrollCode is the device enrollment completion code.
	DeviceEnrollCode = "TV005I"
	// DeviceAuthenticateCode is the device authentication code.
	DeviceAuthenticateCode = "TV006I"
	// DeviceUpdateCode is the device update code.
	DeviceUpdateCode = "TV007I"
	// DeviceWebTokenCreateCode is the device web token creation code.
	DeviceWebTokenCreateCode = "TV008I"
	// DeviceAuthenticateConfirmCode is the device authentication confirm code.
	DeviceAuthenticateConfirmCode = "TV009I"

	// LoginRuleCreateCode is the login rule create code.
	LoginRuleCreateCode = "TLR00I"
	// LoginRuleDeleteCode is the login rule delete code.
	LoginRuleDeleteCode = "TLR01I"

	// SAMLIdPAuthAttemptCode is the SAML IdP auth attempt code.
	SAMLIdPAuthAttemptCode = "TSI000I"

	// SAMLIdPServiceProviderCreateCode is the SAML IdP service provider create code.
	SAMLIdPServiceProviderCreateCode = "TSI001I"

	// SAMLIdPServiceProviderCreateFailureCode is the SAML IdP service provider create failure code.
	SAMLIdPServiceProviderCreateFailureCode = "TSI001W"

	// SAMLIdPServiceProviderUpdateCode is the SAML IdP service provider update code.
	SAMLIdPServiceProviderUpdateCode = "TSI002I"

	// SAMLIdPServiceProviderUpdateFailureCode is the SAML IdP service provider update failure code.
	SAMLIdPServiceProviderUpdateFailureCode = "TSI002W"

	// SAMLIdPServiceProviderDeleteCode is the SAML IdP service provider delete code.
	SAMLIdPServiceProviderDeleteCode = "TSI003I"

	// SAMLIdPServiceProviderDeleteFailureCode is the SAML IdP service provider delete failure code.
	SAMLIdPServiceProviderDeleteFailureCode = "TSI003W"

	// SAMLIdPServiceProviderDeleteAllCode is the SAML IdP service provider delete all code.
	SAMLIdPServiceProviderDeleteAllCode = "TSI004I"

	// SAMLIdPServiceProviderDeleteAllFailureCode is the SAML IdP service provider delete all failure code.
	SAMLIdPServiceProviderDeleteAllFailureCode = "TSI004W"

	// OktaGroupsUpdateCode is the Okta groups updated code.
	OktaGroupsUpdateCode = "TOK001I"

	// OktaApplicationsUpdateCode is the Okta applications updated code.
	OktaApplicationsUpdateCode = "TOK002I"

	// OktaSyncFailureCode is the Okta synchronization failure code.
	OktaSyncFailureCode = "TOK003E"

	// OktaAssignmentProcessSuccessCode is the Okta assignment process success code.
	OktaAssignmentProcessSuccessCode = "TOK004I"

	// OktaAssignmentProcessFailureCode is the Okta assignment process failure code.
	OktaAssignmentProcessFailureCode = "TOK004E"

	// OktaAssignmentCleanupSuccessCode is the Okta assignment cleanup success code.
	OktaAssignmentCleanupSuccessCode = "TOK005I"

	// OktaAssignmentCleanupFailureCode is the Okta assignment cleanup failure code.
	OktaAssignmentCleanupFailureCode = "TOK005E"

	// OktaAccessListSyncSuccessCode is the Okta access list sync success code.
	OktaAccessListSyncSuccessCode = "TOK006I"

	// OktaAccessListSyncSuccessCode is the Okta access list sync failure code.
	OktaAccessListSyncFailureCode = "TOK006E"

	// OktaUserSyncSuccessCode is the Okta user sync success code.
	OktaUserSyncSuccessCode = "TOK007I"

	// OktaUserSyncSuccessCode is the Okta user sync failure code.
	OktaUserSyncFailureCode = "TOK007E"

	// AccessListCreateSuccessCode is the access list create success code.
	AccessListCreateSuccessCode = "TAL001I"

	// AccessListCreateFailureCode is the access list create failure code.
	AccessListCreateFailureCode = "TAL001E"

	// AccessListUpdateSuccessCode is the access list update success code.
	AccessListUpdateSuccessCode = "TAL002I"

	// AccessListUpdateFailureCode is the access list update failure code.
	AccessListUpdateFailureCode = "TAL002E"

	// AccessListDeleteSuccessCode is the access list delete success code.
	AccessListDeleteSuccessCode = "TAL003I"

	// AccessListDeleteFailureCode is the access list delete failure code.
	AccessListDeleteFailureCode = "TAL003E"

	// AccessListReviewSuccessCode is the access list review success code.
	AccessListReviewSuccessCode = "TAL004I"

	// AccessListReviewFailureCode is the access list review failure code.
	AccessListReviewFailureCode = "TAL004E"

	// AccessListMemberCreateSuccessCode is the access list member create success code.
	AccessListMemberCreateSuccessCode = "TAL005I"

	// AccessListMemberCreateFailureCode is the access list member create failure code.
	AccessListMemberCreateFailureCode = "TAL005E"

	// AccessListMemberUpdateSuccessCode is the access list member update success code.
	AccessListMemberUpdateSuccessCode = "TAL006I"

	// AccessListMemberUpdateFailureCode is the access list member update failure code.
	AccessListMemberUpdateFailureCode = "TAL006E"

	// AccessListMemberDeleteSuccessCode is the access list member delete success code.
	AccessListMemberDeleteSuccessCode = "TAL007I"

	// AccessListMemberDeleteFailureCode is the access list member delete failure code.
	AccessListMemberDeleteFailureCode = "TAL007E"

	// AccessListMemberDeleteAllForAccessListSuccessCode is the access list all member delete success code.
	AccessListMemberDeleteAllForAccessListSuccessCode = "TAL008I"

	// AccessListMemberDeleteAllForAccessListFailureCode is the access list member delete failure code.
	AccessListMemberDeleteAllForAccessListFailureCode = "TAL008E"

	// UserLoginAccessListInvalidCode is the user login access list invalid code. This event is a warning that an access list is invalid and was not applied upon the user's login.
	UserLoginAccessListInvalidCode = "TAL009W"

	// SecReportsAuditQueryRunCode is used when a custom Security Reports Query is run.
	SecReportsAuditQueryRunCode = "SRE001I"

	// SecReportsReportRunCode is used when a report in run.
	SecReportsReportRunCode = "SRE002I"

	// ExternalAuditStorageEnableCode is the External Audit Storage enabled code.
	ExternalAuditStorageEnableCode = "TEA001I"
	// ExternalAuditStorageDisableCode is the External Audit Storage disabled code.
	ExternalAuditStorageDisableCode = "TEA002I"

	// SPIFFESVIDIssuedSuccessCode is the SPIFFE SVID issued success code.
	SPIFFESVIDIssuedSuccessCode = "TSPIFFE000I"
	// SPIFFESVIDIssuedFailureCode is the SPIFFE SVID issued failure code.
	SPIFFESVIDIssuedFailureCode = "TSPIFFE000E"
	// SPIFFEFederationCreateCode is the SPIFFE Federation created code.
	SPIFFEFederationCreateCode = "TSPIFFE001I"
	// SPIFFEFederationDeleteCode is the SPIFFE Federation deleted code.
	SPIFFEFederationDeleteCode = "TSPIFFE002I"

	// AuthPreferenceUpdateCode is the auth preference updated event code.
	AuthPreferenceUpdateCode = "TCAUTH001I"
	// ClusterNetworkingConfigUpdateCode is the cluster networking config updated event code.
	ClusterNetworkingConfigUpdateCode = "TCNET002I"
	// SessionRecordingConfigUpdateCode is the session recording config updated event code.
	SessionRecordingConfigUpdateCode = "TCREC003I"
	// AccessGraphSettingsUpdateCode is the access graph settings updated event code.
	AccessGraphSettingsUpdateCode = "TCAGC003I"

	// AccessGraphAccessPathChangedCode is the access graph access path changed event code.
	AccessGraphAccessPathChangedCode = "TAG001I"

	// DiscoveryConfigCreateCode is the discovery config created event code.
	DiscoveryConfigCreateCode = "DC001I"
	// DiscoveryConfigUpdateCode is the discovery config updated event code.
	DiscoveryConfigUpdateCode = "DC002I"
	// DiscoveryConfigDeleteCode is the discovery config delete event code.
	DiscoveryConfigDeleteCode = "DC003I"
	// DiscoveryConfigDeleteAllCode is the discovery config delete all event code.
	DiscoveryConfigDeleteAllCode = "DC004I"

	// IntegrationCreateCode is the integration resource create event code.
	IntegrationCreateCode = "IG001I"
	// IntegrationUpdateCode is the integration resource update event code.
	IntegrationUpdateCode = "IG002I"
	// IntegrationDeleteCode is the integration resource delete event code.
	IntegrationDeleteCode = "IG003I"

	// PluginCreateCode is the plugin resource create event code.
	PluginCreateCode = "PG001I"
	// PluginUpdateCode is the plugin resource update event code.
	PluginUpdateCode = "PG002I"
	// PluginDeleteCode is the plugin resource delete event code.
	PluginDeleteCode = "PG003I"

	// StaticHostUserCreateCode is the static host user resource create event code.
	StaticHostUserCreateCode = "SHU001I"
	// StaticHostUserUpdateCode is the static host user resource update event code.
	StaticHostUserUpdateCode = "SHU002I"
	// StaticHostUserDeleteCode is the static host user resource delete event code.
	StaticHostUserDeleteCode = "SHU003I"

	// CrownJewelCreateCode is the crown jewel create event code.
	CrownJewelCreateCode = "CJ001I"
	// CrownJewelUpdateCode is the crown jewel update event code.
	CrownJewelUpdateCode = "CJ002I"
	// CrownJewelDeleteCode is the crown jewel delete event code.
	CrownJewelDeleteCode = "CJ003I"

	// UserTaskCreateCode is the user task create event code.
	UserTaskCreateCode = "UT001I"
	// UserTaskUpdateCode is the user task update event code.
	UserTaskUpdateCode = "UT002I"
	// UserTaskDeleteCode is the user task delete event code.
	UserTaskDeleteCode = "UT003I"

	// AutoUpdateConfigCreateCode is the auto update config create event code.
	AutoUpdateConfigCreateCode = "AUC001I"
	// AutoUpdateConfigUpdateCode is the auto update config update event code.
	AutoUpdateConfigUpdateCode = "AUC002I"
	// AutoUpdateConfigDeleteCode is the auto update config delete event code.
	AutoUpdateConfigDeleteCode = "AUC003I"

	// AutoUpdateVersionCreateCode is the auto update version create event code.
	AutoUpdateVersionCreateCode = "AUV001I"
	// AutoUpdateVersionUpdateCode is the auto update version update event code.
	AutoUpdateVersionUpdateCode = "AUV002I"
	// AutoUpdateVersionDeleteCode is the auto update version delete event code.
	AutoUpdateVersionDeleteCode = "AUV003I"

	// ContactCreateCode is the auto update version create event code.
	ContactCreateCode = "TCTC001I"
	// ContactDeleteCode is the auto update version delete event code.
	ContactDeleteCode = "TCTC002I"

	// WorkloadIdentityCreateCode is the workload identity create event code.
	WorkloadIdentityCreateCode = "WID001I"
	// WorkloadIdentityUpdateCode is the workload identity update event code.
	WorkloadIdentityUpdateCode = "WID002I"
	// WorkloadIdentityDeleteCode is the workload identity delete event code.
	WorkloadIdentityDeleteCode = "WID003I"

	// GitCommandCode is the git command event code
	GitCommandCode = "TGIT001I"
	// GitCommandFailureCode is the git command feature event code.
	GitCommandFailureCode = "TGIT001E"

	// StableUNIXUserCreateCode is the stable UNIX user create event code.
	StableUNIXUserCreateCode = "TSUU001I"

	// AWSICAccountSyncCode is the AWS Identity Center account sync event code.
	AWSICResourceSyncCode = "TAIC001I"
	// AWSICPermissionAssignmentCreateCode is the AWS Identity Center permission assignment create event code.
	AWSICPermissionAssignmentCreateCode = "TAIC002I"
	// AWSICPermissionAssignmentDeleteCode is the AWS Identity Center permission assignment delete event code.
	AWSICPermissionAssignmentDeleteCode = "TAIC003I"
	// SCIMUserCreateCode is the SCIM user provisioning create event code.
	SCIMUserCreateCode = "TSCP001I"
	// SCIMUserDeleteCode is the SCIM user provisioning delete event code.
	SCIMUserDeleteCode = "TSCP002I"
	// SCIMUserUpdateCode is the SCIM user provisioning update event code.
	SCIMUserUpdateCode = "TSCP003I"
	// SCIMGroupCreateCode is the SCIM group provisioning create event code.
	SCIMGroupCreateCode = "TSCP004I"
	// SCIMGroupDeleteCode is the SCIM group provisioning delete event code.
	SCIMGroupDeleteCode = "TSCP005I"
	// SCIMGroupUpdateCode is the SCIM group provisioning update event code.
	SCIMGroupUpdateCode = "TSCP006I"

	// UnknownCode is used when an event of unknown type is encountered.
	UnknownCode = apievents.UnknownCode
)

// After defining an event code, make sure to keep
// `web/packages/teleport/src/services/audit/types.ts` in sync and add an
// entry in the `eventsMap` in `lib/events/events_test.go`.
