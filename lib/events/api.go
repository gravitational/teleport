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
	"context"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/gravitational/trace"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// EventType is event type/kind
	EventType = "event"
	// EventID is a unique event identifier
	EventID = "uid"
	// EventCode is a code that uniquely identifies a particular event type
	EventCode = "code"
	// EventTime is event time
	EventTime = "time"
	// EventLogin is OS login
	EventLogin = "login"
	// EventUser is teleport user name
	EventUser = "user"
	// EventProtocol specifies protocol that was captured
	EventProtocol = "proto"
	// EventProtocolsSSH specifies SSH as a type of captured protocol
	EventProtocolSSH = "ssh"
	// EventProtocolKube specifies kubernetes as a type of captured protocol
	EventProtocolKube = "kube"
	// EventProtocolTDP specifies Teleport Desktop Protocol (TDP)
	// as a type of captured protocol
	EventProtocolTDP = "tdp"
	// LocalAddr is a target address on the host
	LocalAddr = "addr.local"
	// RemoteAddr is a client (user's) address
	RemoteAddr = "addr.remote"
	// EventCursor is an event ID (used as cursor value for enumeration, not stored)
	EventCursor = "id"

	// EventIndex is an event index as received from the logging server
	EventIndex = "ei"

	// EventNamespace is a namespace of the session event
	EventNamespace = "namespace"

	// SessionPrintEvent event happens every time a write occurs to
	// terminal I/O during a session
	SessionPrintEvent = "print"

	// SessionPrintEventBytes says how many bytes have been written into the session
	// during "print" event
	SessionPrintEventBytes = "bytes"

	// SessionEventTimestamp is an offset (in milliseconds) since the beginning of the
	// session when the terminal IO event happened
	SessionEventTimestamp = "ms"

	// SessionStartEvent indicates that session has been initiated
	// or updated by a joining party on the server
	SessionStartEvent = "session.start"

	// SessionEndEvent indicates that a session has ended
	SessionEndEvent = "session.end"

	// SessionUploadEvent indicates that session has been uploaded to the external storage
	SessionUploadEvent = "session.upload"

	// URL is used for a session upload URL
	URL = "url"

	// SessionEventID is a unique UUID of the session.
	SessionEventID = "sid"

	// SessionServerID is the UUID of the server the session occurred on.
	SessionServerID = "server_id"

	// SessionServerHostname is the hostname of the server the session occurred on.
	SessionServerHostname = "server_hostname"

	// SessionServerAddr is the address of the server the session occurred on.
	SessionServerAddr = "server_addr"

	// SessionStartTime is the timestamp at which the session began.
	SessionStartTime = "session_start"

	// SessionRecordingType is the type of session recording.
	// Possible values are node (default), proxy, node-sync, proxy-sync, or off.
	SessionRecordingType = "session_recording"

	// SessionEndTime is the timestamp at which the session ended.
	SessionEndTime = "session_stop"

	// SessionEnhancedRecording is used to indicate if the recording was an
	// enhanced recording or not.
	SessionEnhancedRecording = "enhanced_recording"

	// SessionInteractive is used to indicate if the session was interactive
	// (has PTY attached) or not (exec session).
	SessionInteractive = "interactive"

	// SessionParticipants is a list of participants in the session.
	SessionParticipants = "participants"

	// SessionServerLabels are the labels (static and dynamic) of the server the
	// session occurred on.
	SessionServerLabels = "server_labels"

	// SessionClusterName is the cluster name that the session occurred in
	SessionClusterName = "cluster_name"

	// SessionByteOffset is the number of bytes written to session stream since
	// the beginning
	SessionByteOffset = "offset"

	// SessionJoinEvent indicates that someone joined a session
	SessionJoinEvent = "session.join"
	// SessionLeaveEvent indicates that someone left a session
	SessionLeaveEvent = "session.leave"

	// Data transfer events.
	SessionDataEvent = "session.data"
	DataTransmitted  = "tx"
	DataReceived     = "rx"

	// ClientDisconnectEvent is emitted when client is disconnected
	// by the server due to inactivity or any other reason
	ClientDisconnectEvent = "client.disconnect"

	// Reason is a field that specifies reason for event, e.g. in disconnect
	// event it explains why server disconnected the client
	Reason = "reason"

	// UserLoginEvent indicates that a user logged into web UI or via tsh
	UserLoginEvent = "user.login"
	// LoginMethod is the event field indicating how the login was performed
	LoginMethod = "method"
	// LoginMethodLocal represents login with username/password
	LoginMethodLocal = "local"
	// LoginMethodClientCert represents login with client certificate
	LoginMethodClientCert = "client.cert"
	// LoginMethodOIDC represents login with OIDC
	LoginMethodOIDC = "oidc"
	// LoginMethodSAML represents login with SAML
	LoginMethodSAML = "saml"
	// LoginMethodGithub represents login with Github
	LoginMethodGithub = "github"
	// LoginMethodHeadless represents headless login request
	LoginMethodHeadless = "headless"

	// UserUpdatedEvent is emitted when the user is updated.
	UserUpdatedEvent = "user.update"

	// UserDeleteEvent is emitted when the user is deleted.
	UserDeleteEvent = "user.delete"

	// UserCreateEvent is emitted when the user is created.
	UserCreateEvent = "user.create"

	// UserPasswordChangeEvent is when the user changes their own password.
	UserPasswordChangeEvent = "user.password_change"

	// UserExpires is when the user will expire.
	UserExpires = "expires"

	// UserRoles is a list of roles for the user.
	UserRoles = "roles"

	// IdentityAttributes is a map of user attributes
	// received from identity provider
	IdentityAttributes = "attributes"

	// UserConnector is the connector used to create the user.
	UserConnector = "connector"

	// AccessRequestCreateEvent is emitted when a new access request is created.
	AccessRequestCreateEvent = "access_request.create"
	// AccessRequestUpdateEvent is emitted when a request's state is updated.
	AccessRequestUpdateEvent = "access_request.update"
	// AccessRequestReviewEvent is emitted when a review is applied to a request.
	AccessRequestReviewEvent = "access_request.review"
	// AccessRequestDeleteEvent is emitted when a new access request is deleted.
	AccessRequestDeleteEvent = "access_request.delete"
	// AccessRequestResourceSearch is emitted when a user searches for
	// resources as part of a search-based access request.
	AccessRequestResourceSearch = "access_request.search"
	// AccessRequestDelegator is used by teleport plugins to indicate the identity
	// which caused them to update state.
	AccessRequestDelegator = "delegator"
	// AccessRequestState is the state of a request.
	AccessRequestState = "state"
	// AccessRequestID is the ID of an access request.
	AccessRequestID = "id"

	// BillingCardCreateEvent is emitted when a user creates a new credit card.
	BillingCardCreateEvent = "billing.create_card"
	// BillingCardDeleteEvent is emitted when a user deletes a credit card.
	BillingCardDeleteEvent = "billing.delete_card"
	// BillingCardUpdateEvent is emitted when a user updates an existing credit card.
	BillingCardUpdateEvent = "billing.update_card"
	// BillingInformationUpdateEvent is emitted when a user updates their billing information.
	BillingInformationUpdateEvent = "billing.update_info"

	// UpdatedBy indicates the user who modified some resource:
	//  - updating a request state
	//  - updating a user record
	UpdatedBy = "updated_by"

	// RecoveryTokenCreateEvent is emitted when a new recovery token is created.
	RecoveryTokenCreateEvent = "recovery_token.create"
	// ResetPasswordTokenCreateEvent is emitted when a new reset password token is created.
	ResetPasswordTokenCreateEvent = "reset_password_token.create"
	// ResetPasswordTokenTTL is TTL of reset password token.
	ResetPasswordTokenTTL = "ttl"
	// PrivilegeTokenCreateEvent is emitted when a new user privilege token is created.
	PrivilegeTokenCreateEvent = "privilege_token.create"

	// FieldName contains name, e.g. resource name, etc.
	FieldName = "name"

	// ExecEvent is an exec command executed by script or user on
	// the server side
	ExecEvent        = "exec"
	ExecEventCommand = "command"
	ExecEventCode    = "exitCode"
	ExecEventError   = "exitError"

	// SubsystemEvent is the result of the execution of a subsystem.
	SubsystemEvent = "subsystem"
	SubsystemName  = "name"
	SubsystemError = "exitError"

	// X11 forwarding event
	X11ForwardEvent   = "x11-forward"
	X11ForwardSuccess = "success"
	X11ForwardErr     = "error"

	// Port forwarding event
	PortForwardEvent           = "port"
	PortForwardLocalEvent      = "port.local"
	PortForwardRemoteEvent     = "port.remote"
	PortForwardRemoteConnEvent = "port.remote_conn"
	PortForwardAddr            = "addr"
	PortForwardSuccess         = "success"
	PortForwardErr             = "error"

	// AuthAttemptEvent is authentication attempt that either
	// succeeded or failed based on event status
	AuthAttemptEvent   = "auth"
	AuthAttemptSuccess = "success"
	AuthAttemptErr     = "error"
	AuthAttemptMessage = "message"

	// SCPEvent means data transfer that occurred on the server
	SCPEvent          = "scp"
	SCPPath           = "path"
	SCPLengh          = "len"
	SCPAction         = "action"
	SCPActionUpload   = "upload"
	SCPActionDownload = "download"

	// SFTPEvent means a user attempted a file operation
	SFTPEvent = "sftp"
	SFTPPath  = "path"

	// ResizeEvent means that some user resized PTY on the client
	ResizeEvent  = "resize"
	TerminalSize = "size" // expressed as 'W:H'

	// SessionUploadIndex is a very large number of the event index
	// to indicate that this is the last event in the chain
	// used for the last event of the sesion - session upload
	SessionUploadIndex = math.MaxInt32
	// SessionDataIndex is a very large number of the event index
	// to indicate one of the last session events, used to report
	// data transfer
	SessionDataIndex = math.MaxInt32 - 1

	// SessionCommandEvent is emitted when an executable is run within a session.
	SessionCommandEvent = "session.command"

	// SessionDiskEvent is emitted when a file is opened within an session.
	SessionDiskEvent = "session.disk"

	// SessionNetworkEvent is emitted when a network connection is initiated with a
	// session.
	SessionNetworkEvent = "session.network"

	// PID is the ID of the process.
	PID = "pid"

	// PPID is the PID of the parent process.
	PPID = "ppid"

	// CgroupID is the internal cgroupv2 ID of the event.
	CgroupID = "cgroup_id"

	// Program is name of the executable.
	Program = "program"

	// Path is the full path to the executable.
	Path = "path"

	// Argv is the list of arguments to the program. Note, the first element does
	// not contain the name of the process.
	Argv = "argv"

	// ReturnCode is the return code of execve.
	ReturnCode = "return_code"

	// Flags are the flags passed to open.
	Flags = "flags"

	// SrcAddr is the source IP address of the connection.
	SrcAddr = "src_addr"

	// DstAddr is the destination IP address of the connection.
	DstAddr = "dst_addr"

	// DstPort is the destination port of the connection.
	DstPort = "dst_port"

	// TCPVersion is the version of TCP (4 or 6).
	TCPVersion = "version"

	// RoleCreatedEvent fires when role is created or upserted.
	RoleCreatedEvent = "role.created"
	// RoleUpdatedEvent fires when role is updated.
	RoleUpdatedEvent = "role.updated"
	// RoleDeletedEvent fires when role is deleted.
	RoleDeletedEvent = "role.deleted"

	// TrustedClusterCreateEvent is the event for creating a trusted cluster.
	TrustedClusterCreateEvent = "trusted_cluster.create"
	// TrustedClusterDeleteEvent is the event for removing a trusted cluster.
	TrustedClusterDeleteEvent = "trusted_cluster.delete"
	// TrustedClusterTokenCreateEvent is the event for creating new provisioning
	// token for a trusted cluster. Deprecated in favor of
	// [ProvisionTokenCreateEvent].
	TrustedClusterTokenCreateEvent = "trusted_cluster_token.create"

	// ProvisionTokenCreateEvent is the event for creating a provisioning token,
	// also known as Join Token. See [types.ProvisionToken].
	ProvisionTokenCreateEvent = "join_token.create"

	// GithubConnectorCreatedEvent fires when a Github connector is created.
	GithubConnectorCreatedEvent = "github.created"
	// GithubConnectorUpdatedEvent fires when a Github connector is updated.
	GithubConnectorUpdatedEvent = "github.updated"
	// GithubConnectorDeletedEvent fires when a Github connector is deleted.
	GithubConnectorDeletedEvent = "github.deleted"
	// OIDCConnectorCreatedEvent fires when OIDC connector is created.
	OIDCConnectorCreatedEvent = "oidc.created"
	// OIDCConnectorUpdatedEvent fires when OIDC connector is updated.
	OIDCConnectorUpdatedEvent = "oidc.updated"
	// OIDCConnectorDeletedEvent fires when OIDC connector is deleted.
	OIDCConnectorDeletedEvent = "oidc.deleted"
	// SAMLConnectorCreatedEvent fires when SAML connector is created.
	SAMLConnectorCreatedEvent = "saml.created"
	// SAMLConnectorUpdatedEvent fires when SAML connector is updated.
	SAMLConnectorUpdatedEvent = "saml.updated"
	// SAMLConnectorDeletedEvent fires when SAML connector is deleted.
	SAMLConnectorDeletedEvent = "saml.deleted"

	// SessionRejectedEvent fires when a user's attempt to create an authenticated
	// session has been rejected due to exceeding a session control limit.
	SessionRejectedEvent = "session.rejected"

	// SessionConnectEvent is emitted when any ssh connection is made
	SessionConnectEvent = "session.connect"

	// AppCreateEvent is emitted when an application resource is created.
	AppCreateEvent = "app.create"
	// AppUpdateEvent is emitted when an application resource is updated.
	AppUpdateEvent = "app.update"
	// AppDeleteEvent is emitted when an application resource is deleted.
	AppDeleteEvent = "app.delete"

	// AppSessionStartEvent is emitted when a user is issued an application certificate.
	AppSessionStartEvent = "app.session.start"
	// AppSessionEndEvent is emitted when a user connects to a TCP application.
	AppSessionEndEvent = "app.session.end"

	// AppSessionChunkEvent is emitted at the start of a 5 minute chunk on each
	// proxy. This chunk is used to buffer 5 minutes of audit events at a time
	// for applications.
	AppSessionChunkEvent = "app.session.chunk"

	// AppSessionRequestEvent is an HTTP request and response.
	AppSessionRequestEvent = "app.session.request"

	// AppSessionDynamoDBRequestEvent is emitted when DynamoDB client sends
	// a request via app access session.
	AppSessionDynamoDBRequestEvent = "app.session.dynamodb.request"

	// DatabaseCreateEvent is emitted when a database resource is created.
	DatabaseCreateEvent = "db.create"
	// DatabaseUpdateEvent is emitted when a database resource is updated.
	DatabaseUpdateEvent = "db.update"
	// DatabaseDeleteEvent is emitted when a database resource is deleted.
	DatabaseDeleteEvent = "db.delete"

	// DatabaseSessionStartEvent is emitted when a database client attempts
	// to connect to a database.
	DatabaseSessionStartEvent = "db.session.start"
	// DatabaseSessionUserCreateEvent is emitted after provisioning new database user.
	DatabaseSessionUserCreateEvent = "db.session.user.create"
	// DatabaseSessionUserDeactivateEvent is emitted after disabling/deleting the auto-provisioned database user.
	DatabaseSessionUserDeactivateEvent = "db.session.user.deactivate"
	// DatabaseSessionPermissionsUpdateEvent is emitted after assigning
	// the auto-provisioned database user permissions.
	DatabaseSessionPermissionsUpdateEvent = "db.session.permissions.update"
	// DatabaseSessionEndEvent is emitted when a database client disconnects
	// from a database.
	DatabaseSessionEndEvent = "db.session.end"
	// DatabaseSessionQueryEvent is emitted when a database client executes
	// a query.
	DatabaseSessionQueryEvent = "db.session.query"
	// DatabaseSessionQueryFailedEvent is emitted when database client's request
	// to execute a database query/command was unsuccessful.
	DatabaseSessionQueryFailedEvent = "db.session.query.failed"
	// DatabaseSessionCommandResult is emitted when a database returns a
	// query/command result.
	DatabaseSessionCommandResultEvent = "db.session.result"

	// DatabaseSessionPostgresParseEvent is emitted when a Postgres client
	// creates a prepared statement using extended query protocol.
	DatabaseSessionPostgresParseEvent = "db.session.postgres.statements.parse"
	// DatabaseSessionPostgresBindEvent is emitted when a Postgres client
	// readies a prepared statement for execution and binds it to parameters.
	DatabaseSessionPostgresBindEvent = "db.session.postgres.statements.bind"
	// DatabaseSessionPostgresExecuteEvent is emitted when a Postgres client
	// executes a previously bound prepared statement.
	DatabaseSessionPostgresExecuteEvent = "db.session.postgres.statements.execute"
	// DatabaseSessionPostgresCloseEvent is emitted when a Postgres client
	// closes an existing prepared statement.
	DatabaseSessionPostgresCloseEvent = "db.session.postgres.statements.close"
	// DatabaseSessionPostgresFunctionEvent is emitted when a Postgres client
	// calls an internal function.
	DatabaseSessionPostgresFunctionEvent = "db.session.postgres.function"

	// DatabaseSessionMySQLStatementPrepareEvent is emitted when a MySQL client
	// creates a prepared statement using the prepared statement protocol.
	DatabaseSessionMySQLStatementPrepareEvent = "db.session.mysql.statements.prepare"
	// DatabaseSessionMySQLStatementExecuteEvent is emitted when a MySQL client
	// executes a prepared statement using the prepared statement protocol.
	DatabaseSessionMySQLStatementExecuteEvent = "db.session.mysql.statements.execute"
	// DatabaseSessionMySQLStatementSendLongDataEvent is emitted when a MySQL
	// client sends long bytes stream using the prepared statement protocol.
	DatabaseSessionMySQLStatementSendLongDataEvent = "db.session.mysql.statements.send_long_data"
	// DatabaseSessionMySQLStatementCloseEvent is emitted when a MySQL client
	// deallocates a prepared statement using the prepared statement protocol.
	DatabaseSessionMySQLStatementCloseEvent = "db.session.mysql.statements.close"
	// DatabaseSessionMySQLStatementResetEvent is emitted when a MySQL client
	// resets the data of a prepared statement using the prepared statement
	// protocol.
	DatabaseSessionMySQLStatementResetEvent = "db.session.mysql.statements.reset"
	// DatabaseSessionMySQLStatementFetchEvent is emitted when a MySQL client
	// fetches rows from a prepared statement using the prepared statement
	// protocol.
	DatabaseSessionMySQLStatementFetchEvent = "db.session.mysql.statements.fetch"
	// DatabaseSessionMySQLStatementBulkExecuteEvent is emitted when a MySQL
	// client executes a bulk insert of a prepared statement using the prepared
	// statement protocol.
	DatabaseSessionMySQLStatementBulkExecuteEvent = "db.session.mysql.statements.bulk_execute"

	// DatabaseSessionMySQLInitDBEvent is emitted when a MySQL client changes
	// the default schema for the connection.
	DatabaseSessionMySQLInitDBEvent = "db.session.mysql.init_db"
	// DatabaseSessionMySQLCreateDBEvent is emitted when a MySQL client creates
	// a schema.
	DatabaseSessionMySQLCreateDBEvent = "db.session.mysql.create_db"
	// DatabaseSessionMySQLDropDBEvent is emitted when a MySQL client drops a
	// schema.
	DatabaseSessionMySQLDropDBEvent = "db.session.mysql.drop_db"
	// DatabaseSessionMySQLShutDownEvent is emitted when a MySQL client asks
	// the server to shut down.
	DatabaseSessionMySQLShutDownEvent = "db.session.mysql.shut_down"
	// DatabaseSessionMySQLProcessKillEvent is emitted when a MySQL client asks
	// the server to terminate a connection.
	DatabaseSessionMySQLProcessKillEvent = "db.session.mysql.process_kill"
	// DatabaseSessionMySQLDebugEvent is emitted when a MySQL client asks the
	// server to dump internal debug info to stdout.
	DatabaseSessionMySQLDebugEvent = "db.session.mysql.debug"
	// DatabaseSessionMySQLRefreshEvent is emitted when a MySQL client sends
	// refresh commands.
	DatabaseSessionMySQLRefreshEvent = "db.session.mysql.refresh"

	// DatabaseSessionSQLServerRPCRequestEvent is emitted when MSServer client sends
	// RPC request command.
	DatabaseSessionSQLServerRPCRequestEvent = "db.session.sqlserver.rpc_request"

	// DatabaseSessionElasticsearchRequestEvent is emitted when Elasticsearch client sends
	// a generic request.
	DatabaseSessionElasticsearchRequestEvent = "db.session.elasticsearch.request"

	// DatabaseSessionOpenSearchRequestEvent is emitted when OpenSearch client sends
	// a request.
	DatabaseSessionOpenSearchRequestEvent = "db.session.opensearch.request"

	// DatabaseSessionDynamoDBRequestEvent is emitted when DynamoDB client sends
	// a request via database-access.
	DatabaseSessionDynamoDBRequestEvent = "db.session.dynamodb.request"

	// DatabaseSessionMalformedPacketEvent is emitted when SQL packet is malformed.
	DatabaseSessionMalformedPacketEvent = "db.session.malformed_packet"

	// DatabaseSessionCassandraBatchEvent is emitted when a Cassandra client executes a batch of queries.
	DatabaseSessionCassandraBatchEvent = "db.session.cassandra.batch"
	// DatabaseSessionCassandraPrepareEvent is emitted when a Cassandra client sends prepare packet.
	DatabaseSessionCassandraPrepareEvent = "db.session.cassandra.prepare"
	// DatabaseSessionCassandraExecuteEvent is emitted when a Cassandra client sends executed packet.
	DatabaseSessionCassandraExecuteEvent = "db.session.cassandra.execute"
	// DatabaseSessionCassandraRegisterEvent is emitted when a Cassandra client sends the register packet.
	DatabaseSessionCassandraRegisterEvent = "db.session.cassandra.register"

	// DatabaseSessionSpannerRPCEvent is emitted when a Spanner client
	// calls a Spanner RPC.
	DatabaseSessionSpannerRPCEvent = "db.session.spanner.rpc"

	// SessionRejectedReasonMaxConnections indicates that a session.rejected event
	// corresponds to enforcement of the max_connections control.
	SessionRejectedReasonMaxConnections = "max_connections limit reached"
	// SessionRejectedReasonMaxSessions indicates that a session.rejected event
	// corresponds to enforcement of the max_sessions control.
	SessionRejectedReasonMaxSessions = "max_sessions limit reached"

	// Maximum is an event field specifying a maximal value (e.g. the value
	// of `max_connections` for a `session.rejected` event).
	Maximum = "max"

	// KubeRequestEvent fires when a proxy handles a generic kubernetes
	// request.
	KubeRequestEvent = "kube.request"

	// KubernetesClusterCreateEvent is emitted when a kubernetes cluster resource is created.
	KubernetesClusterCreateEvent = "kube.create"
	// KubernetesClusterUpdateEvent is emitted when a kubernetes cluster resource is updated.
	KubernetesClusterUpdateEvent = "kube.update"
	// KubernetesClusterDeleteEvent is emitted when a kubernetes cluster resource is deleted.
	KubernetesClusterDeleteEvent = "kube.delete"

	// MFADeviceAddEvent is an event type for users adding MFA devices.
	MFADeviceAddEvent = "mfa.add"
	// MFADeviceDeleteEvent is an event type for users deleting MFA devices.
	MFADeviceDeleteEvent = "mfa.delete"

	// LockCreatedEvent fires when a lock is created/updated.
	LockCreatedEvent = "lock.created"
	// LockDeletedEvent fires when a lock is deleted.
	LockDeletedEvent = "lock.deleted"

	// RecoveryCodeGeneratedEvent is an event type for generating a user's recovery tokens.
	RecoveryCodeGeneratedEvent = "recovery_code.generated"
	// RecoveryCodeUsedEvent is an event type when a recovery token was used.
	RecoveryCodeUsedEvent = "recovery_code.used"

	// WindowsDesktopSessionStartEvent is emitted when a user attempts
	// to connect to a desktop.
	WindowsDesktopSessionStartEvent = "windows.desktop.session.start"
	// WindowsDesktopSessionEndEvent is emitted when a user disconnects
	// from a desktop.
	WindowsDesktopSessionEndEvent = "windows.desktop.session.end"

	// CertificateCreateEvent is emitted when a certificate is issued.
	CertificateCreateEvent = "cert.create"

	// RenewableCertificateGenerationMismatchEvent is emitted when a renewable
	// certificate's generation counter is invalid.
	RenewableCertificateGenerationMismatchEvent = "cert.generation_mismatch"

	// CertificateTypeUser is the CertificateType for certificate events pertaining to user certificates.
	CertificateTypeUser = "user"

	// DesktopRecordingEvent is emitted as a desktop access session is recorded.
	DesktopRecordingEvent = "desktop.recording"
	// DesktopClipboardReceiveEvent is emitted when Teleport receives
	// clipboard data from a remote desktop.
	DesktopClipboardReceiveEvent = "desktop.clipboard.receive"
	// DesktopClipboardSendEvent is emitted when local clipboard data
	// is sent to Teleport.
	DesktopClipboardSendEvent = "desktop.clipboard.send"
	// DesktopSharedDirectoryStartEvent is emitted when when Teleport
	// successfully begins sharing a new directory to a remote desktop.
	DesktopSharedDirectoryStartEvent = "desktop.directory.share"
	// DesktopSharedDirectoryReadEvent is emitted when data is read from a shared directory.
	DesktopSharedDirectoryReadEvent = "desktop.directory.read"
	// DesktopSharedDirectoryWriteEvent is emitted when data is written to a shared directory.
	DesktopSharedDirectoryWriteEvent = "desktop.directory.write"
	// UpgradeWindowStartUpdateEvent is emitted when the upgrade window start time
	// is updated. Used only for teleport cloud.
	UpgradeWindowStartUpdateEvent = "upgradewindowstart.update"

	// SessionRecordingAccessEvent is emitted when a session recording is accessed
	SessionRecordingAccessEvent = "session.recording.access"

	// SSMRunEvent is emitted when a run of an install script
	// completes on a discovered EC2 node
	SSMRunEvent = "ssm.run"

	// DeviceEvent is the catch-all event for Device Trust events.
	// Deprecated: Use one of the more specific event codes below.
	DeviceEvent = "device"
	// DeviceCreateEvent is emitted on device registration.
	// This is an inventory management event.
	DeviceCreateEvent = "device.create"
	// DeviceDeleteEvent is emitted on device deletion.
	// This is an inventory management event.
	DeviceDeleteEvent = "device.delete"
	// DeviceUpdateEvent is emitted on device updates.
	// This is an inventory management event.
	DeviceUpdateEvent = "device.update"
	// DeviceEnrollEvent is emitted when a device is enrolled.
	// Enrollment events are issued due to end-user action, using the trusted
	// device itself.
	DeviceEnrollEvent = "device.enroll"
	// DeviceAuthenticateEvent is emitted when a device is authenticated.
	// Authentication events are issued due to end-user action, using the trusted
	// device itself.
	DeviceAuthenticateEvent = "device.authenticate"
	// DeviceEnrollTokenCreateEvent is emitted when a new enrollment token is
	// issued for a device.
	// Device enroll tokens are issued by either a device admin or during
	// client-side auto-enrollment.
	DeviceEnrollTokenCreateEvent = "device.token.create"
	// DeviceWebTokenCreateEvent is emitted when a new device web token is issued.
	// Device web tokens are issued during Web login for users that own a suitable
	// trusted device.
	// Tokens are spent in exchange for a single on-behalf-of device
	// authentication attempt.
	DeviceWebTokenCreateEvent = "device.webtoken.create"
	// DeviceAuthenticateConfirmEvent is emitted when a device web authentication
	// attempt is confirmed (via the ConfirmDeviceWebAuthentication RPC).
	// A confirmed web authentication means the WebSession itself now holds
	// augmented TLS and SSH certificates.
	DeviceAuthenticateConfirmEvent = "device.authenticate.confirm"

	// BotJoinEvent is emitted when a bot joins
	BotJoinEvent = "bot.join"
	// BotCreateEvent is emitted when a bot is created
	BotCreateEvent = "bot.create"
	// BotUpdateEvent is emitted when a bot is updated
	BotUpdateEvent = "bot.update"
	// BotDeleteEvent is emitted when a bot is deleted
	BotDeleteEvent = "bot.delete"

	// InstanceJoinEvent is emitted when an instance joins
	InstanceJoinEvent = "instance.join"

	// LoginRuleCreateEvent is emitted when a login rule is created or updated.
	LoginRuleCreateEvent = "login_rule.create"
	// LoginRuleDeleteEvent is emitted when a login rule is deleted.
	LoginRuleDeleteEvent = "login_rule.delete"

	// SAMLIdPAuthAttemptEvent is emitted when a user has attempted to authorize against the SAML IdP.
	SAMLIdPAuthAttemptEvent = "saml.idp.auth"

	// SAMLIdPServiceProviderCreateEvent is emitted when a service provider has been created.
	SAMLIdPServiceProviderCreateEvent = "saml.idp.service.provider.create"

	// SAMLIdPServiceProviderUpdateEvent is emitted when a service provider has been updated.
	SAMLIdPServiceProviderUpdateEvent = "saml.idp.service.provider.update"

	// SAMLIdPServiceProviderDeleteEvent is emitted when a service provider has been deleted.
	SAMLIdPServiceProviderDeleteEvent = "saml.idp.service.provider.delete"

	// SAMLIdPServiceProviderDeleteAllEvent is emitted when all service providers have been deleted.
	SAMLIdPServiceProviderDeleteAllEvent = "saml.idp.service.provider.delete_all"

	// OktaGroupsUpdate event is emitted when the groups synced from Okta have been updated.
	OktaGroupsUpdateEvent = "okta.groups.update"

	// OktaApplicationsUpdateEvent is emitted when the applications synced from Okta have been updated.
	OktaApplicationsUpdateEvent = "okta.applications.update"

	// OktaSyncFailureEvent is emitted when the Okta synchronization fails.
	OktaSyncFailureEvent = "okta.sync.failure"

	// OktaAssignmentProcessEvent is emitted when an assignment is processed.
	OktaAssignmentProcessEvent = "okta.assignment.process"

	// OktaAssignmentCleanupEvent is emitted when an assignment is cleaned up.
	OktaAssignmentCleanupEvent = "okta.assignment.cleanup"

	// OktaAccessListSyncEvent is emitted when an access list synchronization has completed.
	OktaAccessListSyncEvent = "okta.access_list.sync"

	// OktaUserSyncEvent is emitted when an access list synchronization has completed.
	OktaUserSyncEvent = "okta.user.sync"

	// AccessListCreateEvent is emitted when an access list is created.
	AccessListCreateEvent = "access_list.create"

	// AccessListUpdateEvent is emitted when an access list is updated.
	AccessListUpdateEvent = "access_list.update"

	// AccessListDeleteEvent is emitted when an access list is deleted.
	AccessListDeleteEvent = "access_list.delete"

	// AccessListReviewEvent is emitted when an access list is reviewed.
	AccessListReviewEvent = "access_list.review"

	// AccessListMemberCreateEvent is emitted when a member is added to an access list.
	AccessListMemberCreateEvent = "access_list.member.create"

	// AccessListMemberUpdateEvent is emitted when a member is updated in an access list.
	AccessListMemberUpdateEvent = "access_list.member.update"

	// AccessListMemberDeleteEvent is emitted when a member is deleted from an access list.
	AccessListMemberDeleteEvent = "access_list.member.delete"

	// AccessListMemberDeleteAllForAccessListEvent is emitted when all members are deleted from an access list.
	AccessListMemberDeleteAllForAccessListEvent = "access_list.member.delete_all_for_access_list"

	// UserLoginAccessListInvalidEvent is emitted when a user logs in as a member of an invalid access list, causing the access list to be skipped.
	UserLoginAccessListInvalidEvent = "user_login.invalid_access_list"

	// UnknownEvent is any event received that isn't recognized as any other event type.
	UnknownEvent = apievents.UnknownEvent

	// SecReportsAuditQueryRunEvent is emitted when a security report query is run.
	SecReportsAuditQueryRunEvent = "secreports.audit.query.run"

	// SecReportsReportRunEvent is emitted when a security report is run.
	SecReportsReportRunEvent = "secreports.report.run"

	// ExternalAuditStorageEnableEvent is emitted when External Audit Storage is
	// enabled.
	ExternalAuditStorageEnableEvent = "external_audit_storage.enable"
	// ExternalAuditStorageDisableEvent is emitted when External Audit Storage is
	// disabled.
	ExternalAuditStorageDisableEvent = "external_audit_storage.disable"

	// CreateMFAAuthChallengeEvent is emitted when an MFA auth challenge is created.
	CreateMFAAuthChallengeEvent = "mfa_auth_challenge.create"

	// ValidateMFAAuthResponseEvent is emitted when an MFA auth challenge is validated.
	ValidateMFAAuthResponseEvent = "mfa_auth_challenge.validate"

	// SPIFFESVIDIssuedEvent is emitted when a SPIFFE SVID is issued.
	SPIFFESVIDIssuedEvent = "spiffe.svid.issued"
	// SPIFFEFederationCreateEvent is emitted when a SPIFFE federation is created.
	SPIFFEFederationCreateEvent = "spiffe.federation.create"
	// SPIFFEFederationDeleteEvent is emitted when a SPIFFE federation is deleted.
	SPIFFEFederationDeleteEvent = "spiffe.federation.delete"

	// AuthPreferenceUpdateEvent is emitted when a user updates the cluster authentication preferences.
	AuthPreferenceUpdateEvent = "auth_preference.update"
	// ClusterNetworkingConfigUpdateEvent is emitted when a user updates the cluster networking configuration.
	ClusterNetworkingConfigUpdateEvent = "cluster_networking_config.update"
	// SessionRecordingConfigUpdateEvent is emitted when a user updates the cluster session recording configuration.
	SessionRecordingConfigUpdateEvent = "session_recording_config.update"
	// AccessGraphSettingsUpdateEvent is emitted when a user updates the access graph settings configuration.
	AccessGraphSettingsUpdateEvent = "access_graph_settings.update"

	// AccessGraphAccessPathChangedEvent is emitted when an access path is changed in the access graph
	// and an identity/resource is affected.
	AccessGraphAccessPathChangedEvent = "access_graph.access_path_changed"
	// TODO(jakule): Remove once e is updated to the new name.
	AccessGraphAccessPathChanged = AccessGraphAccessPathChangedEvent

	// DiscoveryConfigCreatedEvent is emitted when a discovery config is created.
	DiscoveryConfigCreateEvent = "discovery_config.create"
	// DiscoveryConfigUpdatedEvent is emitted when a discovery config is updated.
	DiscoveryConfigUpdateEvent = "discovery_config.update"
	// DiscoveryConfigDeletedEvent is emitted when a discovery config is deleted.
	DiscoveryConfigDeleteEvent = "discovery_config.delete"
	// DiscoveryConfigDeletedAllEvent is emitted when all discovery configs are deleted.
	DiscoveryConfigDeleteAllEvent = "discovery_config.delete_all"

	// IntegrationCreateEvent is emitted when an integration resource is created.
	IntegrationCreateEvent = "integration.create"
	// IntegrationUpdateEvent is emitted when an integration resource is updated.
	IntegrationUpdateEvent = "integration.update"
	// IntegrationDeleteEvent is emitted when an integration resource is deleted.
	IntegrationDeleteEvent = "integration.delete"

	// PluginCreateEvent is emitted when a plugin resource is created.
	PluginCreateEvent = "plugin.create"
	// PluginUpdateEvent is emitted when a plugin resource is updated.
	PluginUpdateEvent = "plugin.update"
	// PluginDeleteEvent is emitted when a plugin resource is deleted.
	PluginDeleteEvent = "plugin.delete"

	// StaticHostUserCreateEvent is emitted when a static host user resource is created.
	StaticHostUserCreateEvent = "static_host_user.create"
	// StaticHostUserUpdateEvent is emitted when a static host user resource is updated.
	StaticHostUserUpdateEvent = "static_host_user.update"
	// StaticHostUserDeleteEvent is emitted when a static host user resource is deleted.
	StaticHostUserDeleteEvent = "static_host_user.delete"

	// CrownJewelCreateEvent is emitted when a crown jewel resource is created.
	CrownJewelCreateEvent = "access_graph.crown_jewel.create"
	// CrownJewelUpdateEvent is emitted when a crown jewel resource is updated.
	CrownJewelUpdateEvent = "access_graph.crown_jewel.update"
	// CrownJewelDeleteEvent is emitted when a crown jewel resource is deleted.
	CrownJewelDeleteEvent = "access_graph.crown_jewel.delete"

	// UserTaskCreateEvent is emitted when a user task resource is created.
	UserTaskCreateEvent = "user_task.create"
	// UserTaskUpdateEvent is emitted when a user task resource is updated.
	UserTaskUpdateEvent = "user_task.update"
	// UserTaskDeleteEvent is emitted when a user task resource is deleted.
	UserTaskDeleteEvent = "user_task.delete"

	// AutoUpdateConfigCreateEvent is emitted when a AutoUpdateConfig resource is created.
	AutoUpdateConfigCreateEvent = "auto_update_config.create"
	// AutoUpdateConfigUpdateEvent is emitted when a AutoUpdateConfig resource is updated.
	AutoUpdateConfigUpdateEvent = "auto_update_config.update"
	// AutoUpdateConfigDeleteEvent is emitted when a AutoUpdateConfig resource is deleted.
	AutoUpdateConfigDeleteEvent = "auto_update_config.delete"

	// AutoUpdateVersionCreateEvent is emitted when a AutoUpdateVersion resource is created.
	AutoUpdateVersionCreateEvent = "auto_update_version.create"
	// AutoUpdateVersionUpdateEvent is emitted when a AutoUpdateVersion resource is updated.
	AutoUpdateVersionUpdateEvent = "auto_update_version.update"
	// AutoUpdateVersionDeleteEvent is emitted when a AutoUpdateVersion resource is deleted.
	AutoUpdateVersionDeleteEvent = "auto_update_version.delete"

	// WorkloadIdentityCreateEvent is emitted when a WorkloadIdentity resource is created.
	WorkloadIdentityCreateEvent = "workload_identity.create"
	// WorkloadIdentityUpdateEvent is emitted when a WorkloadIdentity resource is updated.
	WorkloadIdentityUpdateEvent = "workload_identity.update"
	// WorkloadIdentityDeleteEvent is emitted when a WorkloadIdentity resource is deleted.
	WorkloadIdentityDeleteEvent = "workload_identity.delete"

	// ContactCreateEvent is emitted when a Contact resource is created.
	ContactCreateEvent = "contact.create"
	// ContactDeleteEvent is emitted when a Contact resource is deleted.
	ContactDeleteEvent = "contact.delete"

	// WorkloadIdentityX509RevocationCreateEvent is emitted when a
	// WorkloadIdentityX509Revocation resource is created.
	WorkloadIdentityX509RevocationCreateEvent = "workload_identity_x509_revocation.create"
	// WorkloadIdentityX509RevocationUpdateEvent is emitted when a
	// WorkloadIdentityX509Revocation resource is updated.
	WorkloadIdentityX509RevocationUpdateEvent = "workload_identity_x509_revocation.update"
	// WorkloadIdentityX509RevocationDeleteEvent is emitted when a
	// WorkloadIdentityX509Revocation resource is deleted.
	WorkloadIdentityX509RevocationDeleteEvent = "workload_identity_x509_revocation.delete"
)

// Add an entry to eventsMap in lib/events/events_test.go when you add
// a new event name here.

const (
	// MaxChunkBytes defines the maximum size of a session stream chunk that
	// can be requested via AuditLog.GetSessionChunk(). Set to 5MB
	MaxChunkBytes = 1024 * 1024 * 5
)

const (
	// V1 is the V1 version of slice chunks API,
	// it is 0 because it was not defined before
	V1 = 0
	// V2 is the V2 version of slice chunks  API
	V2 = 2
	// V3 is almost like V2, but it assumes
	// that session recordings are being uploaded
	// at the end of the session, so it skips writing session event index
	// on the fly
	V3 = 3
)

// SessionRecordingEvents is a list of events that are related to session
// recorings.
var SessionRecordingEvents = []string{
	SessionEndEvent,
	WindowsDesktopSessionEndEvent,
	DatabaseSessionEndEvent,
}

// ServerMetadataGetter represents interface
// that provides information about its server id
type ServerMetadataGetter interface {
	// GetServerID returns event server ID
	GetServerID() string

	// GetServerNamespace returns event server namespace
	GetServerNamespace() string

	// GetClusterName returns the originating teleport cluster name
	GetClusterName() string

	// GetForwardedBy returns the ID of the server that forwarded this event.
	GetForwardedBy() string
}

// ServerMetadataSetter represents interface
// that provides information about its server id
type ServerMetadataSetter interface {
	// SetServerID sets server ID of the event
	SetServerID(string)

	// SetServerNamespace returns event server namespace
	SetServerNamespace(string)
}

// SessionMetadataGetter represents interface
// that provides information about events' session metadata
type SessionMetadataGetter interface {
	// GetSessionID returns event session ID
	GetSessionID() string
}

// SessionMetadataSetter represents interface
// that sets session metadata
type SessionMetadataSetter interface {
	// SetSessionID sets event session ID
	SetSessionID(string)

	// SetClusterName sets teleport cluster name
	SetClusterName(string)
}

// Streamer creates and resumes event streams for session IDs
type Streamer interface {
	// CreateAuditStream creates event stream
	CreateAuditStream(context.Context, session.ID) (apievents.Stream, error)
	// ResumeAuditStream resumes the stream for session upload that
	// has not been completed yet.
	ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error)
}

// StreamPart represents uploaded stream part
type StreamPart struct {
	// Number is a part number
	Number int64
	// ETag is a part e-tag
	ETag string
	// LastModified is the time of last modification of this part (if
	// available).
	LastModified time.Time
}

// StreamUpload represents stream multipart upload
type StreamUpload struct {
	// ID is unique upload ID
	ID string
	// SessionID is a session ID of the upload
	SessionID session.ID
	// Initiated contains the timestamp of when the upload
	// was initiated, not always initialized
	Initiated time.Time
}

// String returns user friendly representation of the upload
func (u StreamUpload) String() string {
	return fmt.Sprintf("Upload(session=%v, id=%v, initiated=%v)", u.SessionID, u.ID, u.Initiated)
}

// CheckAndSetDefaults checks and sets default values
func (u *StreamUpload) CheckAndSetDefaults() error {
	if u.ID == "" {
		return trace.BadParameter("missing parameter ID")
	}
	if u.SessionID == "" {
		return trace.BadParameter("missing parameter SessionID")
	}
	return nil
}

// MultipartUploader handles multipart uploads and downloads for session streams
type MultipartUploader interface {
	// CreateUpload creates a multipart upload
	CreateUpload(ctx context.Context, sessionID session.ID) (*StreamUpload, error)
	// CompleteUpload completes the upload
	CompleteUpload(ctx context.Context, upload StreamUpload, parts []StreamPart) error
	// ReserveUploadPart reserves an upload part. Reserve is used to identify
	// upload errors beforehand.
	ReserveUploadPart(ctx context.Context, upload StreamUpload, partNumber int64) error
	// UploadPart uploads part and returns the part
	UploadPart(ctx context.Context, upload StreamUpload, partNumber int64, partBody io.ReadSeeker) (*StreamPart, error)
	// ListParts returns all uploaded parts for the completed upload in sorted order
	ListParts(ctx context.Context, upload StreamUpload) ([]StreamPart, error)
	// ListUploads lists uploads that have been initiated but not completed with
	// earlier uploads returned first
	ListUploads(ctx context.Context) ([]StreamUpload, error)
	// GetUploadMetadata gets the upload metadata
	GetUploadMetadata(sessionID session.ID) UploadMetadata
}

// UploadMetadata contains data about the session upload
type UploadMetadata struct {
	// URL is the url at which the session recording is located
	// it is free-form and uploader-specific
	URL string
	// SessionID is the event session ID
	SessionID session.ID
}

// UploadMetadataGetter gets the metadata for session upload
type UploadMetadataGetter interface {
	GetUploadMetadata(sid session.ID) UploadMetadata
}

// SessionEventPreparer will set necessary event fields for session-related
// events and must be called before the event is used, regardless
// of whether the event will be recorded, emitted, or both.
type SessionEventPreparer interface {
	PrepareSessionEvent(event apievents.AuditEvent) (apievents.PreparedSessionEvent, error)
}

// SessionRecorder records session events. It can be used both as a
// [io.Writer] when recording raw session data and as a [apievents.Recorder]
// when recording session events.
type SessionRecorder interface {
	io.Writer
	apievents.Stream
}

// SessionPreparerRecorder sets necessary session event fields and records them.
type SessionPreparerRecorder interface {
	SessionEventPreparer
	SessionRecorder
}

// StreamEmitter supports emitting single events to the audit log
// and streaming events to a session recording.
type StreamEmitter interface {
	apievents.Emitter
	Streamer
}

// AuditLogSessionStreamer is the primary (and the only external-facing)
// interface for AuditLogger and SessionStreamer.
type AuditLogSessionStreamer interface {
	AuditLogger
	SessionStreamer
}

// SessionStreamer supports streaming session chunks or events.
type SessionStreamer interface {
	// GetSessionChunk returns a reader which can be used to read a byte stream
	// of a recorded session starting from 'offsetBytes' (pass 0 to start from the
	// beginning) up to maxBytes bytes.
	//
	// If maxBytes > MaxChunkBytes, it gets rounded down to MaxChunkBytes
	GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error)

	// Returns all events that happen during a session sorted by time
	// (oldest first).
	//
	// after is used to return events after a specified cursor ID
	GetSessionEvents(namespace string, sid session.ID, after int) ([]EventFields, error)

	// StreamSessionEvents streams all events from a given session recording. An
	// error is returned on the first channel if one is encountered. Otherwise
	// the event channel is closed when the stream ends. The event channel is
	// not closed on error to prevent race conditions in downstream select
	// statements. Both returned channels must be driven until the event channel
	// is exhausted or the error channel reports an error, or until the context
	// is canceled.
	StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error)
}

type SearchEventsRequest struct {
	// From is oldest date of returned events, can be zero.
	From time.Time
	// To is the newest date of returned events.
	To time.Time
	// EventTypes is optional, if not set, returns all events.
	EventTypes []string
	// Limit is the maximum amount of events returned.
	Limit int
	// Order specifies an ascending or descending order of events.
	Order types.EventOrder
	// StartKey is used to resume a query in order to enable pagination.
	// If the previous response had LastKey set then this should be
	// set to its value. Otherwise leave empty.
	StartKey string
}

type SearchSessionEventsRequest struct {
	// From is oldest date of returned events, can be zero.
	From time.Time
	// To is the newest date of returned events.
	To time.Time
	// Limit is the maximum amount of events returned.
	Limit int
	// Order specifies an ascending or descending order of events.
	Order types.EventOrder
	// StartKey is used to resume a query in order to enable pagination.
	// If the previous response had LastKey set then this should be
	// set to its value. Otherwise leave empty.
	StartKey string
	// Cond can be used to pass additional expression to query, can be empty.
	Cond *types.WhereExpr
	// SessionID is optional parameter to return session events only to given session.
	SessionID string
}

// AuditLogger defines which methods need to implemented by audit loggers.
type AuditLogger interface {
	// Closer releases connection and resources associated with log if any
	io.Closer

	// Emitter emits an audit event
	apievents.Emitter

	// SearchEvents is a flexible way to find events.
	//
	// Event types to filter can be specified and pagination is handled by an iterator key that allows
	// a query to be resumed.
	//
	// The only mandatory requirement is a date range (UTC).
	//
	// This function may never return more than 1 MiB of event data.
	SearchEvents(ctx context.Context, req SearchEventsRequest) ([]apievents.AuditEvent, string, error)

	// SearchSessionEvents is a flexible way to find session events.
	// Only session.end events are returned by this function.
	// This is used to find completed sessions.
	//
	// Event types to filter can be specified and pagination is handled by an iterator key that allows
	// a query to be resumed.
	//
	// This function may never return more than 1 MiB of event data.
	SearchSessionEvents(ctx context.Context, req SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error)

	// ExportUnstructuredEvents exports events from a given event chunk returned by GetEventExportChunks. This API prioritizes
	// performance over ordering and filtering, and is intended for bulk export of events.
	ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured]

	// GetEventExportChunks returns a stream of event chunks that can be exported via ExportUnstructuredEvents. The returned
	// list isn't ordered and polling for new chunks requires re-consuming the entire stream from the beginning.
	GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk]
}

// EventFields instance is attached to every logged event
type EventFields utils.Fields

// String returns a string representation of an event structure
func (f EventFields) AsString() string {
	return fmt.Sprintf("%s: login=%s, id=%v, bytes=%v",
		f.GetString(EventType),
		f.GetString(EventLogin),
		f.GetInt(EventCursor),
		f.GetInt(SessionPrintEventBytes))
}

// GetType returns the type (string) of the event
func (f EventFields) GetType() string {
	return f.GetString(EventType)
}

// GetID returns the unique event ID
func (f EventFields) GetID() string {
	return f.GetString(EventID)
}

// GetCode returns the event code
func (f EventFields) GetCode() string {
	return f.GetString(EventCode)
}

// GetTimestamp returns the event timestamp (when it was emitted)
func (f EventFields) GetTimestamp() time.Time {
	return f.GetTime(EventTime)
}

// GetString returns a string representation of a logged field
func (f EventFields) GetString(key string) string {
	return utils.Fields(f).GetString(key)
}

// GetString returns a slice-of-strings representation of a logged field.
func (f EventFields) GetStrings(key string) []string {
	return utils.Fields(f).GetStrings(key)
}

// GetInt returns an int representation of a logged field
func (f EventFields) GetInt(key string) int {
	return utils.Fields(f).GetInt(key)
}

// GetTime returns a time.Time representation of a logged field
func (f EventFields) GetTime(key string) time.Time {
	return utils.Fields(f).GetTime(key)
}

// HasField returns true if the field exists in the event.
func (f EventFields) HasField(key string) bool {
	return utils.Fields(f).HasField(key)
}
