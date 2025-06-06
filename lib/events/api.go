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

import (
	"github.com/gravitational/teleport/lib/eventsclient"
)

const (
	// EventType is event type/kind
	EventType = eventsclient.EventType
	// EventID is a unique event identifier
	EventID = eventsclient.EventID
	// EventCode is a code that uniquely identifies a particular event type
	EventCode = eventsclient.EventCode
	// EventTime is event time
	EventTime = eventsclient.EventTime
	// EventLogin is OS login
	EventLogin = eventsclient.EventLogin
	// EventProtocolsSSH specifies SSH as a type of captured protocol
	EventProtocolSSH = eventsclient.EventProtocolSSH
	// EventProtocolKube specifies kubernetes as a type of captured protocol
	EventProtocolKube = eventsclient.EventProtocolKube
	// EventProtocolTDP specifies Teleport Desktop Protocol (TDP)
	// as a type of captured protocol
	EventProtocolTDP = eventsclient.EventProtocolTDP
	// LocalAddr is a target address on the host
	LocalAddr = eventsclient.LocalAddr
	// RemoteAddr is a client (user's) address
	RemoteAddr = eventsclient.RemoteAddr
	// EventCursor is an event ID (used as cursor value for enumeration, not stored)
	EventCursor = eventsclient.EventCursor

	// EventIndex is an event index as received from the logging server
	EventIndex = eventsclient.EventIndex

	// EventNamespace is a namespace of the session event
	EventNamespace = eventsclient.EventNamespace

	// SessionPrintEvent event happens every time a write occurs to
	// terminal I/O during a session
	SessionPrintEvent = eventsclient.SessionPrintEvent

	// SessionPrintEventBytes says how many bytes have been written into the session
	// during "print" event
	SessionPrintEventBytes = eventsclient.SessionPrintEventBytes

	// SessionStartEvent indicates that session has been initiated
	// or updated by a joining party on the server
	SessionStartEvent = eventsclient.SessionStartEvent

	// SessionEndEvent indicates that a session has ended
	SessionEndEvent = eventsclient.SessionEndEvent

	// SessionUploadEvent indicates that session has been uploaded to the external storage
	SessionUploadEvent = eventsclient.SessionUploadEvent

	// URL is used for a session upload URL
	URL = eventsclient.URL

	// SessionEventID is a unique UUID of the session.
	SessionEventID = eventsclient.SessionEventID

	// SessionParticipants is a list of participants in the session.
	SessionParticipants = eventsclient.SessionParticipants

	// SessionJoinEvent indicates that someone joined a session
	SessionJoinEvent = eventsclient.SessionJoinEvent
	// SessionLeaveEvent indicates that someone left a session
	SessionLeaveEvent = eventsclient.SessionLeaveEvent

	// Data transfer events.
	SessionDataEvent = eventsclient.SessionDataEvent
	DataTransmitted  = eventsclient.DataTransmitted
	DataReceived     = eventsclient.DataReceived

	// ClientDisconnectEvent is emitted when client is disconnected
	// by the server due to inactivity or any other reason
	ClientDisconnectEvent = eventsclient.ClientDisconnectEvent

	// Reason is a field that specifies reason for event, e.g. in disconnect
	// event it explains why server disconnected the client
	Reason = eventsclient.Reason

	// UserLoginEvent indicates that a user logged into web UI or via tsh
	UserLoginEvent = eventsclient.UserLoginEvent
	// LoginMethod is the event field indicating how the login was performed
	LoginMethod = eventsclient.LoginMethod
	// LoginMethodLocal represents login with username/password
	LoginMethodLocal = eventsclient.LoginMethodLocal
	// LoginMethodClientCert represents login with client certificate
	LoginMethodClientCert = eventsclient.LoginMethodClientCert
	// LoginMethodOIDC represents login with OIDC
	LoginMethodOIDC = eventsclient.LoginMethodOIDC
	// LoginMethodSAML represents login with SAML
	LoginMethodSAML = eventsclient.LoginMethodSAML
	// LoginMethodGithub represents login with Github
	LoginMethodGithub = eventsclient.LoginMethodGithub
	// LoginMethodHeadless represents headless login request
	LoginMethodHeadless = eventsclient.LoginMethodHeadless

	// UserUpdatedEvent is emitted when the user is updated.
	UserUpdatedEvent = eventsclient.UserUpdatedEvent

	// UserDeleteEvent is emitted when the user is deleted.
	UserDeleteEvent = eventsclient.UserDeleteEvent

	// UserCreateEvent is emitted when the user is created.
	UserCreateEvent = eventsclient.UserCreateEvent

	// UserPasswordChangeEvent is when the user changes their own password.
	UserPasswordChangeEvent = eventsclient.UserPasswordChangeEvent

	// AccessRequestCreateEvent is emitted when a new access request is created.
	AccessRequestCreateEvent = eventsclient.AccessRequestCreateEvent
	// AccessRequestUpdateEvent is emitted when a request's state is updated.
	AccessRequestUpdateEvent = eventsclient.AccessRequestUpdateEvent
	// AccessRequestReviewEvent is emitted when a review is applied to a request.
	AccessRequestReviewEvent = eventsclient.AccessRequestReviewEvent
	// AccessRequestExpirEvent is emitted when an access request expires.
	AccessRequestExpireEvent = eventsclient.AccessRequestExpireEvent
	// AccessRequestDeleteEvent is emitted when a new access request is deleted.
	AccessRequestDeleteEvent = eventsclient.AccessRequestDeleteEvent
	// AccessRequestResourceSearch is emitted when a user searches for
	// resources as part of a search-based access request.
	AccessRequestResourceSearch = eventsclient.AccessRequestResourceSearch
	// AccessRequestID is the ID of an access request.
	AccessRequestID = eventsclient.AccessRequestID

	// BillingCardCreateEvent is emitted when a user creates a new credit card.
	BillingCardCreateEvent = eventsclient.BillingCardCreateEvent
	// BillingCardDeleteEvent is emitted when a user deletes a credit card.
	BillingCardDeleteEvent = eventsclient.BillingCardDeleteEvent
	// BillingCardUpdateEvent is emitted when a user updates an existing credit card.
	BillingCardUpdateEvent = eventsclient.BillingCardUpdateEvent
	// BillingInformationUpdateEvent is emitted when a user updates their billing information.
	BillingInformationUpdateEvent = eventsclient.BillingInformationUpdateEvent

	// UpdatedBy indicates the user who modified some resource:
	//  - updating a request state
	//  - updating a user record
	UpdatedBy = eventsclient.UpdatedBy

	// RecoveryTokenCreateEvent is emitted when a new recovery token is created.
	RecoveryTokenCreateEvent = eventsclient.RecoveryTokenCreateEvent
	// ResetPasswordTokenCreateEvent is emitted when a new reset password token is created.
	ResetPasswordTokenCreateEvent = eventsclient.ResetPasswordTokenCreateEvent
	// PrivilegeTokenCreateEvent is emitted when a new user privilege token is created.
	PrivilegeTokenCreateEvent = eventsclient.PrivilegeTokenCreateEvent

	// ExecEvent is an exec command executed by script or user on
	// the server side
	ExecEvent        = eventsclient.ExecEvent
	ExecEventCommand = eventsclient.ExecEventCommand

	// SubsystemEvent is the result of the execution of a subsystem.
	SubsystemEvent = eventsclient.SubsystemEvent

	// X11 forwarding event
	X11ForwardEvent = eventsclient.X11ForwardEvent

	// Port forwarding event
	PortForwardEvent           = eventsclient.PortForwardEvent
	PortForwardLocalEvent      = eventsclient.PortForwardLocalEvent
	PortForwardRemoteEvent     = eventsclient.PortForwardRemoteEvent
	PortForwardRemoteConnEvent = eventsclient.PortForwardRemoteConnEvent

	// AuthAttemptEvent is authentication attempt that either
	// succeeded or failed based on event status
	AuthAttemptEvent = eventsclient.AuthAttemptEvent

	// SCPEvent means data transfer that occurred on the server
	SCPEvent          = eventsclient.SCPEvent
	SCPActionUpload   = eventsclient.SCPActionUpload
	SCPActionDownload = eventsclient.SCPActionDownload

	// SFTPEvent means a user attempted a file operation
	SFTPEvent = eventsclient.SFTPEvent
	SFTPPath  = eventsclient.SFTPPath
	// SFTPSummaryEvent is emitted at the end of an SFTP transfer.
	SFTPSummaryEvent = eventsclient.SFTPSummaryEvent

	// ResizeEvent means that some user resized PTY on the client
	ResizeEvent  = eventsclient.ResizeEvent
	TerminalSize = eventsclient.TerminalSize // expressed as 'W:H'

	// SessionUploadIndex is a very large number of the event index
	// to indicate that this is the last event in the chain
	// used for the last event of the sesion - session upload
	SessionUploadIndex = eventsclient.SessionUploadIndex
	// SessionDataIndex is a very large number of the event index
	// to indicate one of the last session events, used to report
	// data transfer
	SessionDataIndex = eventsclient.SessionDataIndex

	// SessionCommandEvent is emitted when an executable is run within a session.
	SessionCommandEvent = eventsclient.SessionCommandEvent

	// SessionDiskEvent is emitted when a file is opened within an session.
	SessionDiskEvent = eventsclient.SessionDiskEvent

	// SessionNetworkEvent is emitted when a network connection is initiated with a
	// session.
	SessionNetworkEvent = eventsclient.SessionNetworkEvent

	// Path is the full path to the executable.
	Path = eventsclient.Path

	// RoleCreatedEvent fires when role is created or upserted.
	RoleCreatedEvent = eventsclient.RoleCreatedEvent
	// RoleUpdatedEvent fires when role is updated.
	RoleUpdatedEvent = eventsclient.RoleUpdatedEvent
	// RoleDeletedEvent fires when role is deleted.
	RoleDeletedEvent = eventsclient.RoleDeletedEvent

	// TrustedClusterCreateEvent is the event for creating a trusted cluster.
	TrustedClusterCreateEvent = eventsclient.TrustedClusterCreateEvent
	// TrustedClusterDeleteEvent is the event for removing a trusted cluster.
	TrustedClusterDeleteEvent = eventsclient.TrustedClusterDeleteEvent
	// TrustedClusterTokenCreateEvent is the event for creating new provisioning
	// token for a trusted cluster. Deprecated in favor of
	// [ProvisionTokenCreateEvent].
	TrustedClusterTokenCreateEvent = eventsclient.TrustedClusterTokenCreateEvent

	// ProvisionTokenCreateEvent is the event for creating a provisioning token,
	// also known as Join Token. See [types.ProvisionToken].
	ProvisionTokenCreateEvent = eventsclient.ProvisionTokenCreateEvent

	// GithubConnectorCreatedEvent fires when a Github connector is created.
	GithubConnectorCreatedEvent = eventsclient.GithubConnectorCreatedEvent
	// GithubConnectorUpdatedEvent fires when a Github connector is updated.
	GithubConnectorUpdatedEvent = eventsclient.GithubConnectorUpdatedEvent
	// GithubConnectorDeletedEvent fires when a Github connector is deleted.
	GithubConnectorDeletedEvent = eventsclient.GithubConnectorDeletedEvent
	// OIDCConnectorCreatedEvent fires when OIDC connector is created.
	OIDCConnectorCreatedEvent = eventsclient.OIDCConnectorCreatedEvent
	// OIDCConnectorUpdatedEvent fires when OIDC connector is updated.
	OIDCConnectorUpdatedEvent = eventsclient.OIDCConnectorUpdatedEvent
	// OIDCConnectorDeletedEvent fires when OIDC connector is deleted.
	OIDCConnectorDeletedEvent = eventsclient.OIDCConnectorDeletedEvent
	// SAMLConnectorCreatedEvent fires when SAML connector is created.
	SAMLConnectorCreatedEvent = eventsclient.SAMLConnectorCreatedEvent
	// SAMLConnectorUpdatedEvent fires when SAML connector is updated.
	SAMLConnectorUpdatedEvent = eventsclient.SAMLConnectorUpdatedEvent
	// SAMLConnectorDeletedEvent fires when SAML connector is deleted.
	SAMLConnectorDeletedEvent = eventsclient.SAMLConnectorDeletedEvent

	// SessionRejectedEvent fires when a user's attempt to create an authenticated
	// session has been rejected due to exceeding a session control limit.
	SessionRejectedEvent = eventsclient.SessionRejectedEvent

	// SessionConnectEvent is emitted when any ssh connection is made
	SessionConnectEvent = eventsclient.SessionConnectEvent

	// AppCreateEvent is emitted when an application resource is created.
	AppCreateEvent = eventsclient.AppCreateEvent
	// AppUpdateEvent is emitted when an application resource is updated.
	AppUpdateEvent = eventsclient.AppUpdateEvent
	// AppDeleteEvent is emitted when an application resource is deleted.
	AppDeleteEvent = eventsclient.AppDeleteEvent

	// AppSessionStartEvent is emitted when a user is issued an application certificate.
	AppSessionStartEvent = eventsclient.AppSessionStartEvent
	// AppSessionEndEvent is emitted when a user connects to a TCP application.
	AppSessionEndEvent = eventsclient.AppSessionEndEvent

	// AppSessionChunkEvent is emitted at the start of a 5 minute chunk on each
	// proxy. This chunk is used to buffer 5 minutes of audit events at a time
	// for applications.
	AppSessionChunkEvent = eventsclient.AppSessionChunkEvent

	// AppSessionRequestEvent is an HTTP request and response.
	AppSessionRequestEvent = eventsclient.AppSessionRequestEvent

	// AppSessionDynamoDBRequestEvent is emitted when DynamoDB client sends
	// a request via app access session.
	AppSessionDynamoDBRequestEvent = eventsclient.AppSessionDynamoDBRequestEvent

	// DatabaseCreateEvent is emitted when a database resource is created.
	DatabaseCreateEvent = eventsclient.DatabaseCreateEvent
	// DatabaseUpdateEvent is emitted when a database resource is updated.
	DatabaseUpdateEvent = eventsclient.DatabaseUpdateEvent
	// DatabaseDeleteEvent is emitted when a database resource is deleted.
	DatabaseDeleteEvent = eventsclient.DatabaseDeleteEvent

	// DatabaseSessionStartEvent is emitted when a database client attempts
	// to connect to a database.
	DatabaseSessionStartEvent = eventsclient.DatabaseSessionStartEvent
	// DatabaseSessionUserCreateEvent is emitted after provisioning new database user.
	DatabaseSessionUserCreateEvent = eventsclient.DatabaseSessionUserCreateEvent
	// DatabaseSessionUserDeactivateEvent is emitted after disabling/deleting the auto-provisioned database user.
	DatabaseSessionUserDeactivateEvent = eventsclient.DatabaseSessionUserDeactivateEvent
	// DatabaseSessionPermissionsUpdateEvent is emitted after assigning
	// the auto-provisioned database user permissions.
	DatabaseSessionPermissionsUpdateEvent = eventsclient.DatabaseSessionPermissionsUpdateEvent
	// DatabaseSessionEndEvent is emitted when a database client disconnects
	// from a database.
	DatabaseSessionEndEvent = eventsclient.DatabaseSessionEndEvent
	// DatabaseSessionQueryEvent is emitted when a database client executes
	// a query.
	DatabaseSessionQueryEvent = eventsclient.DatabaseSessionQueryEvent
	// DatabaseSessionQueryFailedEvent is emitted when database client's request
	// to execute a database query/command was unsuccessful.
	DatabaseSessionQueryFailedEvent = eventsclient.DatabaseSessionQueryFailedEvent
	// DatabaseSessionCommandResult is emitted when a database returns a
	// query/command result.
	DatabaseSessionCommandResultEvent = eventsclient.DatabaseSessionCommandResultEvent

	// DatabaseSessionPostgresParseEvent is emitted when a Postgres client
	// creates a prepared statement using extended query protocol.
	DatabaseSessionPostgresParseEvent = eventsclient.DatabaseSessionPostgresParseEvent
	// DatabaseSessionPostgresBindEvent is emitted when a Postgres client
	// readies a prepared statement for execution and binds it to parameters.
	DatabaseSessionPostgresBindEvent = eventsclient.DatabaseSessionPostgresBindEvent
	// DatabaseSessionPostgresExecuteEvent is emitted when a Postgres client
	// executes a previously bound prepared statement.
	DatabaseSessionPostgresExecuteEvent = eventsclient.DatabaseSessionPostgresExecuteEvent
	// DatabaseSessionPostgresCloseEvent is emitted when a Postgres client
	// closes an existing prepared statement.
	DatabaseSessionPostgresCloseEvent = eventsclient.DatabaseSessionPostgresCloseEvent
	// DatabaseSessionPostgresFunctionEvent is emitted when a Postgres client
	// calls an internal function.
	DatabaseSessionPostgresFunctionEvent = eventsclient.DatabaseSessionPostgresFunctionEvent

	// DatabaseSessionMySQLStatementPrepareEvent is emitted when a MySQL client
	// creates a prepared statement using the prepared statement protocol.
	DatabaseSessionMySQLStatementPrepareEvent = eventsclient.DatabaseSessionMySQLStatementPrepareEvent
	// DatabaseSessionMySQLStatementExecuteEvent is emitted when a MySQL client
	// executes a prepared statement using the prepared statement protocol.
	DatabaseSessionMySQLStatementExecuteEvent = eventsclient.DatabaseSessionMySQLStatementExecuteEvent
	// DatabaseSessionMySQLStatementSendLongDataEvent is emitted when a MySQL
	// client sends long bytes stream using the prepared statement protocol.
	DatabaseSessionMySQLStatementSendLongDataEvent = eventsclient.DatabaseSessionMySQLStatementSendLongDataEvent
	// DatabaseSessionMySQLStatementCloseEvent is emitted when a MySQL client
	// deallocates a prepared statement using the prepared statement protocol.
	DatabaseSessionMySQLStatementCloseEvent = eventsclient.DatabaseSessionMySQLStatementCloseEvent
	// DatabaseSessionMySQLStatementResetEvent is emitted when a MySQL client
	// resets the data of a prepared statement using the prepared statement
	// protocol.
	DatabaseSessionMySQLStatementResetEvent = eventsclient.DatabaseSessionMySQLStatementResetEvent
	// DatabaseSessionMySQLStatementFetchEvent is emitted when a MySQL client
	// fetches rows from a prepared statement using the prepared statement
	// protocol.
	DatabaseSessionMySQLStatementFetchEvent = eventsclient.DatabaseSessionMySQLStatementFetchEvent
	// DatabaseSessionMySQLStatementBulkExecuteEvent is emitted when a MySQL
	// client executes a bulk insert of a prepared statement using the prepared
	// statement protocol.
	DatabaseSessionMySQLStatementBulkExecuteEvent = eventsclient.DatabaseSessionMySQLStatementBulkExecuteEvent

	// DatabaseSessionMySQLInitDBEvent is emitted when a MySQL client changes
	// the default schema for the connection.
	DatabaseSessionMySQLInitDBEvent = eventsclient.DatabaseSessionMySQLInitDBEvent
	// DatabaseSessionMySQLCreateDBEvent is emitted when a MySQL client creates
	// a schema.
	DatabaseSessionMySQLCreateDBEvent = eventsclient.DatabaseSessionMySQLCreateDBEvent
	// DatabaseSessionMySQLDropDBEvent is emitted when a MySQL client drops a
	// schema.
	DatabaseSessionMySQLDropDBEvent = eventsclient.DatabaseSessionMySQLDropDBEvent
	// DatabaseSessionMySQLShutDownEvent is emitted when a MySQL client asks
	// the server to shut down.
	DatabaseSessionMySQLShutDownEvent = eventsclient.DatabaseSessionMySQLShutDownEvent
	// DatabaseSessionMySQLProcessKillEvent is emitted when a MySQL client asks
	// the server to terminate a connection.
	DatabaseSessionMySQLProcessKillEvent = eventsclient.DatabaseSessionMySQLProcessKillEvent
	// DatabaseSessionMySQLDebugEvent is emitted when a MySQL client asks the
	// server to dump internal debug info to stdout.
	DatabaseSessionMySQLDebugEvent = eventsclient.DatabaseSessionMySQLDebugEvent
	// DatabaseSessionMySQLRefreshEvent is emitted when a MySQL client sends
	// refresh commands.
	DatabaseSessionMySQLRefreshEvent = eventsclient.DatabaseSessionMySQLRefreshEvent

	// DatabaseSessionSQLServerRPCRequestEvent is emitted when MSServer client sends
	// RPC request command.
	DatabaseSessionSQLServerRPCRequestEvent = eventsclient.DatabaseSessionSQLServerRPCRequestEvent

	// DatabaseSessionElasticsearchRequestEvent is emitted when Elasticsearch client sends
	// a generic request.
	DatabaseSessionElasticsearchRequestEvent = eventsclient.DatabaseSessionElasticsearchRequestEvent

	// DatabaseSessionOpenSearchRequestEvent is emitted when OpenSearch client sends
	// a request.
	DatabaseSessionOpenSearchRequestEvent = eventsclient.DatabaseSessionOpenSearchRequestEvent

	// DatabaseSessionDynamoDBRequestEvent is emitted when DynamoDB client sends
	// a request via database-access.
	DatabaseSessionDynamoDBRequestEvent = eventsclient.DatabaseSessionDynamoDBRequestEvent

	// DatabaseSessionMalformedPacketEvent is emitted when SQL packet is malformed.
	DatabaseSessionMalformedPacketEvent = eventsclient.DatabaseSessionMalformedPacketEvent

	// DatabaseSessionCassandraBatchEvent is emitted when a Cassandra client executes a batch of queries.
	DatabaseSessionCassandraBatchEvent = eventsclient.DatabaseSessionCassandraBatchEvent
	// DatabaseSessionCassandraPrepareEvent is emitted when a Cassandra client sends prepare packet.
	DatabaseSessionCassandraPrepareEvent = eventsclient.DatabaseSessionCassandraPrepareEvent
	// DatabaseSessionCassandraExecuteEvent is emitted when a Cassandra client sends executed packet.
	DatabaseSessionCassandraExecuteEvent = eventsclient.DatabaseSessionCassandraExecuteEvent
	// DatabaseSessionCassandraRegisterEvent is emitted when a Cassandra client sends the register packet.
	DatabaseSessionCassandraRegisterEvent = eventsclient.DatabaseSessionCassandraRegisterEvent

	// DatabaseSessionSpannerRPCEvent is emitted when a Spanner client
	// calls a Spanner RPC.
	DatabaseSessionSpannerRPCEvent = eventsclient.DatabaseSessionSpannerRPCEvent

	// SessionRejectedReasonMaxSessions indicates that a session.rejected event
	// corresponds to enforcement of the max_sessions control.
	SessionRejectedReasonMaxSessions = eventsclient.SessionRejectedReasonMaxSessions

	// KubeRequestEvent fires when a proxy handles a generic kubernetes
	// request.
	KubeRequestEvent = eventsclient.KubeRequestEvent

	// KubernetesClusterCreateEvent is emitted when a kubernetes cluster resource is created.
	KubernetesClusterCreateEvent = eventsclient.KubernetesClusterCreateEvent
	// KubernetesClusterUpdateEvent is emitted when a kubernetes cluster resource is updated.
	KubernetesClusterUpdateEvent = eventsclient.KubernetesClusterUpdateEvent
	// KubernetesClusterDeleteEvent is emitted when a kubernetes cluster resource is deleted.
	KubernetesClusterDeleteEvent = eventsclient.KubernetesClusterDeleteEvent

	// MFADeviceAddEvent is an event type for users adding MFA devices.
	MFADeviceAddEvent = eventsclient.MFADeviceAddEvent
	// MFADeviceDeleteEvent is an event type for users deleting MFA devices.
	MFADeviceDeleteEvent = eventsclient.MFADeviceDeleteEvent

	// LockCreatedEvent fires when a lock is created/updated.
	LockCreatedEvent = eventsclient.LockCreatedEvent
	// LockDeletedEvent fires when a lock is deleted.
	LockDeletedEvent = eventsclient.LockDeletedEvent

	// RecoveryCodeGeneratedEvent is an event type for generating a user's recovery tokens.
	RecoveryCodeGeneratedEvent = eventsclient.RecoveryCodeGeneratedEvent
	// RecoveryCodeUsedEvent is an event type when a recovery token was used.
	RecoveryCodeUsedEvent = eventsclient.RecoveryCodeUsedEvent

	// WindowsDesktopSessionStartEvent is emitted when a user attempts
	// to connect to a desktop.
	WindowsDesktopSessionStartEvent = eventsclient.WindowsDesktopSessionStartEvent
	// WindowsDesktopSessionEndEvent is emitted when a user disconnects
	// from a desktop.
	WindowsDesktopSessionEndEvent = eventsclient.WindowsDesktopSessionEndEvent

	// CertificateCreateEvent is emitted when a certificate is issued.
	CertificateCreateEvent = eventsclient.CertificateCreateEvent

	// RenewableCertificateGenerationMismatchEvent is emitted when a renewable
	// certificate's generation counter is invalid.
	RenewableCertificateGenerationMismatchEvent = eventsclient.RenewableCertificateGenerationMismatchEvent

	// CertificateTypeUser is the CertificateType for certificate events pertaining to user certificates.
	CertificateTypeUser = eventsclient.CertificateTypeUser

	// DesktopRecordingEvent is emitted as a desktop access session is recorded.
	DesktopRecordingEvent = eventsclient.DesktopRecordingEvent
	// DesktopClipboardReceiveEvent is emitted when Teleport receives
	// clipboard data from a remote desktop.
	DesktopClipboardReceiveEvent = eventsclient.DesktopClipboardReceiveEvent
	// DesktopClipboardSendEvent is emitted when local clipboard data
	// is sent to Teleport.
	DesktopClipboardSendEvent = eventsclient.DesktopClipboardSendEvent
	// DesktopSharedDirectoryStartEvent is emitted when when Teleport
	// successfully begins sharing a new directory to a remote desktop.
	DesktopSharedDirectoryStartEvent = eventsclient.DesktopSharedDirectoryStartEvent
	// DesktopSharedDirectoryReadEvent is emitted when data is read from a shared directory.
	DesktopSharedDirectoryReadEvent = eventsclient.DesktopSharedDirectoryReadEvent
	// DesktopSharedDirectoryWriteEvent is emitted when data is written to a shared directory.
	DesktopSharedDirectoryWriteEvent = eventsclient.DesktopSharedDirectoryWriteEvent
	// UpgradeWindowStartUpdateEvent is emitted when the upgrade window start time
	// is updated. Used only for teleport cloud.
	UpgradeWindowStartUpdateEvent = eventsclient.UpgradeWindowStartUpdateEvent

	// SessionRecordingAccessEvent is emitted when a session recording is accessed
	SessionRecordingAccessEvent = eventsclient.SessionRecordingAccessEvent

	// SSMRunEvent is emitted when a run of an install script
	// completes on a discovered EC2 node
	SSMRunEvent = eventsclient.SSMRunEvent

	// DeviceEvent is the catch-all event for Device Trust events.
	// Deprecated: Use one of the more specific event codes below.
	DeviceEvent = eventsclient.DeviceEvent
	// DeviceCreateEvent is emitted on device registration.
	// This is an inventory management event.
	DeviceCreateEvent = eventsclient.DeviceCreateEvent
	// DeviceDeleteEvent is emitted on device deletion.
	// This is an inventory management event.
	DeviceDeleteEvent = eventsclient.DeviceDeleteEvent
	// DeviceUpdateEvent is emitted on device updates.
	// This is an inventory management event.
	DeviceUpdateEvent = eventsclient.DeviceUpdateEvent
	// DeviceEnrollEvent is emitted when a device is enrolled.
	// Enrollment events are issued due to end-user action, using the trusted
	// device itself.
	DeviceEnrollEvent = eventsclient.DeviceEnrollEvent
	// DeviceAuthenticateEvent is emitted when a device is authenticated.
	// Authentication events are issued due to end-user action, using the trusted
	// device itself.
	DeviceAuthenticateEvent = eventsclient.DeviceAuthenticateEvent
	// DeviceEnrollTokenCreateEvent is emitted when a new enrollment token is
	// issued for a device.
	// Device enroll tokens are issued by either a device admin or during
	// client-side auto-enrollment.
	DeviceEnrollTokenCreateEvent = eventsclient.DeviceEnrollTokenCreateEvent
	// DeviceWebTokenCreateEvent is emitted when a new device web token is issued.
	// Device web tokens are issued during Web login for users that own a suitable
	// trusted device.
	// Tokens are spent in exchange for a single on-behalf-of device
	// authentication attempt.
	DeviceWebTokenCreateEvent = eventsclient.DeviceWebTokenCreateEvent
	// DeviceAuthenticateConfirmEvent is emitted when a device web authentication
	// attempt is confirmed (via the ConfirmDeviceWebAuthentication RPC).
	// A confirmed web authentication means the WebSession itself now holds
	// augmented TLS and SSH certificates.
	DeviceAuthenticateConfirmEvent = eventsclient.DeviceAuthenticateConfirmEvent

	// BotJoinEvent is emitted when a bot joins
	BotJoinEvent = eventsclient.BotJoinEvent
	// BotCreateEvent is emitted when a bot is created
	BotCreateEvent = eventsclient.BotCreateEvent
	// BotUpdateEvent is emitted when a bot is updated
	BotUpdateEvent = eventsclient.BotUpdateEvent
	// BotDeleteEvent is emitted when a bot is deleted
	BotDeleteEvent = eventsclient.BotDeleteEvent

	// InstanceJoinEvent is emitted when an instance joins
	InstanceJoinEvent = eventsclient.InstanceJoinEvent

	// LoginRuleCreateEvent is emitted when a login rule is created or updated.
	LoginRuleCreateEvent = eventsclient.LoginRuleCreateEvent
	// LoginRuleDeleteEvent is emitted when a login rule is deleted.
	LoginRuleDeleteEvent = eventsclient.LoginRuleDeleteEvent

	// SAMLIdPAuthAttemptEvent is emitted when a user has attempted to authorize against the SAML IdP.
	SAMLIdPAuthAttemptEvent = eventsclient.SAMLIdPAuthAttemptEvent

	// SAMLIdPServiceProviderCreateEvent is emitted when a service provider has been created.
	SAMLIdPServiceProviderCreateEvent = eventsclient.SAMLIdPServiceProviderCreateEvent

	// SAMLIdPServiceProviderUpdateEvent is emitted when a service provider has been updated.
	SAMLIdPServiceProviderUpdateEvent = eventsclient.SAMLIdPServiceProviderUpdateEvent

	// SAMLIdPServiceProviderDeleteEvent is emitted when a service provider has been deleted.
	SAMLIdPServiceProviderDeleteEvent = eventsclient.SAMLIdPServiceProviderDeleteEvent

	// SAMLIdPServiceProviderDeleteAllEvent is emitted when all service providers have been deleted.
	SAMLIdPServiceProviderDeleteAllEvent = eventsclient.SAMLIdPServiceProviderDeleteAllEvent

	// OktaGroupsUpdate event is emitted when the groups synced from Okta have been updated.
	OktaGroupsUpdateEvent = eventsclient.OktaGroupsUpdateEvent

	// OktaApplicationsUpdateEvent is emitted when the applications synced from Okta have been updated.
	OktaApplicationsUpdateEvent = eventsclient.OktaApplicationsUpdateEvent

	// OktaSyncFailureEvent is emitted when the Okta synchronization fails.
	OktaSyncFailureEvent = eventsclient.OktaSyncFailureEvent

	// OktaAssignmentProcessEvent is emitted when an assignment is processed.
	OktaAssignmentProcessEvent = eventsclient.OktaAssignmentProcessEvent

	// OktaAssignmentCleanupEvent is emitted when an assignment is cleaned up.
	OktaAssignmentCleanupEvent = eventsclient.OktaAssignmentCleanupEvent

	// OktaAccessListSyncEvent is emitted when an access list synchronization has completed.
	OktaAccessListSyncEvent = eventsclient.OktaAccessListSyncEvent

	// OktaUserSyncEvent is emitted when an access list synchronization has completed.
	OktaUserSyncEvent = eventsclient.OktaUserSyncEvent

	// AccessListCreateEvent is emitted when an access list is created.
	AccessListCreateEvent = eventsclient.AccessListCreateEvent

	// AccessListUpdateEvent is emitted when an access list is updated.
	AccessListUpdateEvent = eventsclient.AccessListUpdateEvent

	// AccessListDeleteEvent is emitted when an access list is deleted.
	AccessListDeleteEvent = eventsclient.AccessListDeleteEvent

	// AccessListReviewEvent is emitted when an access list is reviewed.
	AccessListReviewEvent = eventsclient.AccessListReviewEvent

	// AccessListMemberCreateEvent is emitted when a member is added to an access list.
	AccessListMemberCreateEvent = eventsclient.AccessListMemberCreateEvent

	// AccessListMemberUpdateEvent is emitted when a member is updated in an access list.
	AccessListMemberUpdateEvent = eventsclient.AccessListMemberUpdateEvent

	// AccessListMemberDeleteEvent is emitted when a member is deleted from an access list.
	AccessListMemberDeleteEvent = eventsclient.AccessListMemberDeleteEvent

	// AccessListMemberDeleteAllForAccessListEvent is emitted when all members are deleted from an access list.
	AccessListMemberDeleteAllForAccessListEvent = eventsclient.AccessListMemberDeleteAllForAccessListEvent

	// UserLoginAccessListInvalidEvent is emitted when a user logs in as a member of an invalid access list, causing the access list to be skipped.
	UserLoginAccessListInvalidEvent = eventsclient.UserLoginAccessListInvalidEvent

	// UnknownEvent is any event received that isn't recognized as any other event type.
	UnknownEvent = eventsclient.UnknownEvent

	// SecReportsAuditQueryRunEvent is emitted when a security report query is run.
	SecReportsAuditQueryRunEvent = eventsclient.SecReportsAuditQueryRunEvent

	// SecReportsReportRunEvent is emitted when a security report is run.
	SecReportsReportRunEvent = eventsclient.SecReportsReportRunEvent

	// ExternalAuditStorageEnableEvent is emitted when External Audit Storage is
	// enabled.
	ExternalAuditStorageEnableEvent = eventsclient.ExternalAuditStorageEnableEvent
	// ExternalAuditStorageDisableEvent is emitted when External Audit Storage is
	// disabled.
	ExternalAuditStorageDisableEvent = eventsclient.ExternalAuditStorageDisableEvent

	// CreateMFAAuthChallengeEvent is emitted when an MFA auth challenge is created.
	CreateMFAAuthChallengeEvent = eventsclient.CreateMFAAuthChallengeEvent

	// ValidateMFAAuthResponseEvent is emitted when an MFA auth challenge is validated.
	ValidateMFAAuthResponseEvent = eventsclient.ValidateMFAAuthResponseEvent

	// SPIFFESVIDIssuedEvent is emitted when a SPIFFE SVID is issued.
	SPIFFESVIDIssuedEvent = eventsclient.SPIFFESVIDIssuedEvent
	// SPIFFEFederationCreateEvent is emitted when a SPIFFE federation is created.
	SPIFFEFederationCreateEvent = eventsclient.SPIFFEFederationCreateEvent
	// SPIFFEFederationDeleteEvent is emitted when a SPIFFE federation is deleted.
	SPIFFEFederationDeleteEvent = eventsclient.SPIFFEFederationDeleteEvent

	// AuthPreferenceUpdateEvent is emitted when a user updates the cluster authentication preferences.
	AuthPreferenceUpdateEvent = eventsclient.AuthPreferenceUpdateEvent
	// ClusterNetworkingConfigUpdateEvent is emitted when a user updates the cluster networking configuration.
	ClusterNetworkingConfigUpdateEvent = eventsclient.ClusterNetworkingConfigUpdateEvent
	// SessionRecordingConfigUpdateEvent is emitted when a user updates the cluster session recording configuration.
	SessionRecordingConfigUpdateEvent = eventsclient.SessionRecordingConfigUpdateEvent
	// AccessGraphSettingsUpdateEvent is emitted when a user updates the access graph settings configuration.
	AccessGraphSettingsUpdateEvent = eventsclient.AccessGraphSettingsUpdateEvent

	// AccessGraphAccessPathChangedEvent is emitted when an access path is changed in the access graph
	// and an identity/resource is affected.
	AccessGraphAccessPathChangedEvent = eventsclient.AccessGraphAccessPathChangedEvent
	// TODO(jakule): Remove once e is updated to the new name.
	AccessGraphAccessPathChanged = AccessGraphAccessPathChangedEvent

	// DiscoveryConfigCreatedEvent is emitted when a discovery config is created.
	DiscoveryConfigCreateEvent = eventsclient.DiscoveryConfigCreateEvent
	// DiscoveryConfigUpdatedEvent is emitted when a discovery config is updated.
	DiscoveryConfigUpdateEvent = eventsclient.DiscoveryConfigUpdateEvent
	// DiscoveryConfigDeletedEvent is emitted when a discovery config is deleted.
	DiscoveryConfigDeleteEvent = eventsclient.DiscoveryConfigDeleteEvent
	// DiscoveryConfigDeletedAllEvent is emitted when all discovery configs are deleted.
	DiscoveryConfigDeleteAllEvent = eventsclient.DiscoveryConfigDeleteAllEvent

	// IntegrationCreateEvent is emitted when an integration resource is created.
	IntegrationCreateEvent = eventsclient.IntegrationCreateEvent
	// IntegrationUpdateEvent is emitted when an integration resource is updated.
	IntegrationUpdateEvent = eventsclient.IntegrationUpdateEvent
	// IntegrationDeleteEvent is emitted when an integration resource is deleted.
	IntegrationDeleteEvent = eventsclient.IntegrationDeleteEvent

	// PluginCreateEvent is emitted when a plugin resource is created.
	PluginCreateEvent = eventsclient.PluginCreateEvent
	// PluginUpdateEvent is emitted when a plugin resource is updated.
	PluginUpdateEvent = eventsclient.PluginUpdateEvent
	// PluginDeleteEvent is emitted when a plugin resource is deleted.
	PluginDeleteEvent = eventsclient.PluginDeleteEvent

	// StaticHostUserCreateEvent is emitted when a static host user resource is created.
	StaticHostUserCreateEvent = eventsclient.StaticHostUserCreateEvent
	// StaticHostUserUpdateEvent is emitted when a static host user resource is updated.
	StaticHostUserUpdateEvent = eventsclient.StaticHostUserUpdateEvent
	// StaticHostUserDeleteEvent is emitted when a static host user resource is deleted.
	StaticHostUserDeleteEvent = eventsclient.StaticHostUserDeleteEvent

	// CrownJewelCreateEvent is emitted when a crown jewel resource is created.
	CrownJewelCreateEvent = eventsclient.CrownJewelCreateEvent
	// CrownJewelUpdateEvent is emitted when a crown jewel resource is updated.
	CrownJewelUpdateEvent = eventsclient.CrownJewelUpdateEvent
	// CrownJewelDeleteEvent is emitted when a crown jewel resource is deleted.
	CrownJewelDeleteEvent = eventsclient.CrownJewelDeleteEvent

	// UserTaskCreateEvent is emitted when a user task resource is created.
	UserTaskCreateEvent = eventsclient.UserTaskCreateEvent
	// UserTaskUpdateEvent is emitted when a user task resource is updated.
	UserTaskUpdateEvent = eventsclient.UserTaskUpdateEvent
	// UserTaskDeleteEvent is emitted when a user task resource is deleted.
	UserTaskDeleteEvent = eventsclient.UserTaskDeleteEvent

	// AutoUpdateConfigCreateEvent is emitted when a AutoUpdateConfig resource is created.
	AutoUpdateConfigCreateEvent = eventsclient.AutoUpdateConfigCreateEvent
	// AutoUpdateConfigUpdateEvent is emitted when a AutoUpdateConfig resource is updated.
	AutoUpdateConfigUpdateEvent = eventsclient.AutoUpdateConfigUpdateEvent
	// AutoUpdateConfigDeleteEvent is emitted when a AutoUpdateConfig resource is deleted.
	AutoUpdateConfigDeleteEvent = eventsclient.AutoUpdateConfigDeleteEvent

	// AutoUpdateVersionCreateEvent is emitted when a AutoUpdateVersion resource is created.
	AutoUpdateVersionCreateEvent = eventsclient.AutoUpdateVersionCreateEvent
	// AutoUpdateVersionUpdateEvent is emitted when a AutoUpdateVersion resource is updated.
	AutoUpdateVersionUpdateEvent = eventsclient.AutoUpdateVersionUpdateEvent
	// AutoUpdateVersionDeleteEvent is emitted when a AutoUpdateVersion resource is deleted.
	AutoUpdateVersionDeleteEvent = eventsclient.AutoUpdateVersionDeleteEvent

	// AutoUpdateAgentRolloutTriggerEvent is emitted when one or many groups
	// from AutoUpdateAgentRollout resource are manually triggered.
	AutoUpdateAgentRolloutTriggerEvent = eventsclient.AutoUpdateAgentRolloutTriggerEvent
	// AutoUpdateAgentRolloutForceDoneEvent is emitted when one or many groups
	// from AutoUpdateAgentRollout resource are manually forced to a done state.
	AutoUpdateAgentRolloutForceDoneEvent = eventsclient.AutoUpdateAgentRolloutForceDoneEvent
	// AutoUpdateAgentRolloutRollbackEvent is emitted when one or many groups
	// from AutoUpdateAgentRollout resource are manually rolledback.
	AutoUpdateAgentRolloutRollbackEvent = eventsclient.AutoUpdateAgentRolloutRollbackEvent

	// ContactCreateEvent is emitted when a Contact resource is created.
	ContactCreateEvent = eventsclient.ContactCreateEvent
	// ContactDeleteEvent is emitted when a Contact resource is deleted.
	ContactDeleteEvent = eventsclient.ContactDeleteEvent

	// WorkloadIdentityCreateEvent is emitted when a WorkloadIdentity resource is created.
	WorkloadIdentityCreateEvent = eventsclient.WorkloadIdentityCreateEvent
	// WorkloadIdentityUpdateEvent is emitted when a WorkloadIdentity resource is updated.
	WorkloadIdentityUpdateEvent = eventsclient.WorkloadIdentityUpdateEvent
	// WorkloadIdentityDeleteEvent is emitted when a WorkloadIdentity resource is deleted.
	WorkloadIdentityDeleteEvent = eventsclient.WorkloadIdentityDeleteEvent

	// WorkloadIdentityX509RevocationCreateEvent is emitted when a
	// WorkloadIdentityX509Revocation resource is created.
	WorkloadIdentityX509RevocationCreateEvent = eventsclient.WorkloadIdentityX509RevocationCreateEvent
	// WorkloadIdentityX509RevocationUpdateEvent is emitted when a
	// WorkloadIdentityX509Revocation resource is updated.
	WorkloadIdentityX509RevocationUpdateEvent = eventsclient.WorkloadIdentityX509RevocationUpdateEvent
	// WorkloadIdentityX509RevocationDeleteEvent is emitted when a
	// WorkloadIdentityX509Revocation resource is deleted.
	WorkloadIdentityX509RevocationDeleteEvent = eventsclient.WorkloadIdentityX509RevocationDeleteEvent
	// WorkloadIdentityX509IssuerOverrideCreateEvent is emitted when a
	// workload_identity_x509_issuer_override is written.
	WorkloadIdentityX509IssuerOverrideCreateEvent = eventsclient.WorkloadIdentityX509IssuerOverrideCreateEvent
	// WorkloadIdentityX509IssuerOverrideDeleteEvent is emitted when a
	// workload_identity_x509_issuer_override is deleted.
	WorkloadIdentityX509IssuerOverrideDeleteEvent = eventsclient.WorkloadIdentityX509IssuerOverrideDeleteEvent

	// SigstorePolicyCreateEvent is emitted when a SigstorePolicy resource is created.
	SigstorePolicyCreateEvent = eventsclient.SigstorePolicyCreateEvent
	// SigstorePolicyUpdateEvent is emitted when a SigstorePolicy resource is updated.
	SigstorePolicyUpdateEvent = eventsclient.SigstorePolicyUpdateEvent
	// SigstorePolicyDeleteEvent is emitted when a SigstorePolicy resource is deleted.
	SigstorePolicyDeleteEvent = eventsclient.SigstorePolicyDeleteEvent

	// GitCommandEvent is emitted when a Git command is executed.
	GitCommandEvent = eventsclient.GitCommandEvent

	// StableUNIXUserCreateEvent is emitted when a stable UNIX user is created.
	StableUNIXUserCreateEvent = eventsclient.StableUNIXUserCreateEvent

	// AWSICResourceSyncSuccessEvent is emitted when AWS Identity Center resources are imported
	// and reconciled to Teleport.
	AWSICResourceSyncSuccessEvent = eventsclient.AWSICResourceSyncSuccessEvent
	// AWSICResourceSyncFailureEvent is emitted when AWS Identity Center resources sync failed.
	AWSICResourceSyncFailureEvent = eventsclient.AWSICResourceSyncFailureEvent

	// HealthCheckConfigCreateEvent is emitted when a health check config
	// resource is created.
	HealthCheckConfigCreateEvent = eventsclient.HealthCheckConfigCreateEvent
	// HealthCheckConfigUpdateEvent is emitted when a health check config
	// resource is updated.
	HealthCheckConfigUpdateEvent = eventsclient.HealthCheckConfigUpdateEvent
	// HealthCheckConfigDeleteEvent is emitted when a health check config
	// resource is deleted.
	HealthCheckConfigDeleteEvent = eventsclient.HealthCheckConfigDeleteEvent

	// MCPSessionStartEvent is emitted when a user starts a MCP session.
	MCPSessionStartEvent = eventsclient.MCPSessionStartEvent
	// MCPSessionEndEvent is emitted when an MCP session ends.
	MCPSessionEndEvent = eventsclient.MCPSessionEndEvent
	// MCPSessionRequestEvent is emitted when a request is sent by client during
	// a MCP session.
	MCPSessionRequestEvent = eventsclient.MCPSessionRequestEvent
	// MCPSessionNotificationEvent is emitted when a notification is sent by
	// client during a MCP session.
	MCPSessionNotificationEvent = eventsclient.MCPSessionNotificationEvent
)

// Add an entry to eventsMap in lib/events/events_test.go when you add
// a new event name here.

const (
	// V1 is the V1 version of slice chunks API,
	// it is 0 because it was not defined before
	V1 = eventsclient.V1
	// V2 is the V2 version of slice chunks  API
	V2 = eventsclient.V2
	// V3 is almost like V2, but it assumes
	// that session recordings are being uploaded
	// at the end of the session, so it skips writing session event index
	// on the fly
	V3 = eventsclient.V3
)

// SessionRecordingEvents is a list of events that are related to session
// recorings.
var SessionRecordingEvents = eventsclient.SessionRecordingEvents

// ServerMetadataGetter represents interface
// that provides information about its server id
type ServerMetadataGetter = eventsclient.ServerMetadataGetter

// ServerMetadataSetter represents interface
// that provides information about its server id
type ServerMetadataSetter = eventsclient.ServerMetadataSetter

// SessionMetadataGetter represents interface
// that provides information about events' session metadata
type SessionMetadataGetter = eventsclient.SessionMetadataGetter

// SessionMetadataSetter represents interface
// that sets session metadata
type SessionMetadataSetter = eventsclient.SessionMetadataSetter

// Streamer creates and resumes event streams for session IDs
type Streamer = eventsclient.Streamer

// StreamPart represents uploaded stream part
type StreamPart = eventsclient.StreamPart

// StreamUpload represents stream multipart upload
type StreamUpload = eventsclient.StreamUpload

// MultipartUploader handles multipart uploads and downloads for session streams
type MultipartUploader = eventsclient.MultipartUploader

// UploadMetadata contains data about the session upload
type UploadMetadata = eventsclient.UploadMetadata

// UploadMetadataGetter gets the metadata for session upload
type UploadMetadataGetter = eventsclient.UploadMetadataGetter

// SessionEventPreparer will set necessary event fields for session-related
// events and must be called before the event is used, regardless
// of whether the event will be recorded, emitted, or both.
type SessionEventPreparer = eventsclient.SessionEventPreparer

// SessionRecorder records session events. It can be used both as a
// [io.Writer] when recording raw session data and as a [apievents.Recorder]
// when recording session events.
type SessionRecorder = eventsclient.SessionRecorder

// SessionPreparerRecorder sets necessary session event fields and records them.
type SessionPreparerRecorder = eventsclient.SessionPreparerRecorder

// StreamEmitter supports emitting single events to the audit log
// and streaming events to a session recording.
type StreamEmitter = eventsclient.StreamEmitter

// AuditLogSessionStreamer is the primary (and the only external-facing)
// interface for AuditLogger and SessionStreamer.
type AuditLogSessionStreamer = eventsclient.AuditLogSessionStreamer

// SessionStreamer supports streaming session chunks or events.
type SessionStreamer = eventsclient.SessionStreamer

type SearchEventsRequest = eventsclient.SearchEventsRequest

type SearchSessionEventsRequest = eventsclient.SearchSessionEventsRequest

// AuditLogger defines which methods need to implemented by audit loggers.
type AuditLogger = eventsclient.AuditLogger

// EventFields instance is attached to every logged event
type EventFields = eventsclient.EventFields
