/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package events

import (
	"encoding/json"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
)

// FromEventFields converts from the typed dynamic representation
// to the new typed interface-style representation.
//
// This is mainly used to convert from the backend format used by
// our various event backends.
func FromEventFields(fields EventFields) (events.AuditEvent, error) {
	data, err := json.Marshal(fields)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	getFieldEmpty := func(field string) string {
		i, ok := fields[field]
		if !ok {
			return ""
		}
		s, _ := i.(string)
		return s
	}

	eventType := getFieldEmpty(EventType)
	var e events.AuditEvent

	switch eventType {
	case SessionPrintEvent:
		e = &events.SessionPrint{}
	case SessionStartEvent:
		e = &events.SessionStart{}
	case SessionEndEvent:
		e = &events.SessionEnd{}
	case SessionUploadEvent:
		e = &events.SessionUpload{}
	case SessionJoinEvent:
		e = &events.SessionJoin{}
	case SessionLeaveEvent:
		e = &events.SessionLeave{}
	case SessionDataEvent:
		e = &events.SessionData{}
	case ClientDisconnectEvent:
		e = &events.ClientDisconnect{}
	case UserLoginEvent:
		e = &events.UserLogin{}
	case UserDeleteEvent:
		e = &events.UserDelete{}
	case UserCreateEvent:
		e = &events.UserCreate{}
	case UserUpdatedEvent:
		// note: user.update is a custom code applied on top of the same data as the user.create event
		//       and they are thus functionally identical. There exists no direct gRPC version of user.update.
		e = &events.UserCreate{}
	case UserPasswordChangeEvent:
		e = &events.UserPasswordChange{}
	case AccessRequestCreateEvent:
		e = &events.AccessRequestCreate{}
	case AccessRequestReviewEvent:
		// note: access_request.review is a custom code applied on top of the same data as the access_request.create event
		//       and they are thus functionally identical. There exists no direct gRPC version of access_request.review.
		e = &events.AccessRequestCreate{}
	case AccessRequestUpdateEvent:
		e = &events.AccessRequestCreate{}
	case AccessRequestResourceSearch:
		e = &events.AccessRequestResourceSearch{}
	case BillingCardCreateEvent:
		e = &events.BillingCardCreate{}
	case BillingCardUpdateEvent:
		e = &events.BillingCardCreate{}
	case BillingCardDeleteEvent:
		e = &events.BillingCardDelete{}
	case BillingInformationUpdateEvent:
		e = &events.BillingInformationUpdate{}
	case ResetPasswordTokenCreateEvent:
		e = &events.UserTokenCreate{}
	case ExecEvent:
		e = &events.Exec{}
	case SubsystemEvent:
		e = &events.Subsystem{}
	case X11ForwardEvent:
		e = &events.X11Forward{}
	case PortForwardEvent:
		e = &events.PortForward{}
	case AuthAttemptEvent:
		e = &events.AuthAttempt{}
	case SCPEvent:
		e = &events.SCP{}
	case ResizeEvent:
		e = &events.Resize{}
	case SessionCommandEvent:
		e = &events.SessionCommand{}
	case SessionDiskEvent:
		e = &events.SessionDisk{}
	case SessionNetworkEvent:
		e = &events.SessionNetwork{}
	case RoleCreatedEvent:
		e = &events.RoleCreate{}
	case RoleDeletedEvent:
		e = &events.RoleDelete{}
	case TrustedClusterCreateEvent:
		e = &events.TrustedClusterCreate{}
	case TrustedClusterDeleteEvent:
		e = &events.TrustedClusterDelete{}
	case TrustedClusterTokenCreateEvent:
		//nolint:staticcheck // We still need to support viewing the deprecated event
		//type for backwards compatibility.
		e = &events.TrustedClusterTokenCreate{}
	case ProvisionTokenCreateEvent:
		e = &events.ProvisionTokenCreate{}
	case GithubConnectorCreatedEvent:
		e = &events.GithubConnectorCreate{}
	case GithubConnectorDeletedEvent:
		e = &events.GithubConnectorDelete{}
	case OIDCConnectorCreatedEvent:
		e = &events.OIDCConnectorCreate{}
	case OIDCConnectorDeletedEvent:
		e = &events.OIDCConnectorDelete{}
	case SAMLConnectorCreatedEvent:
		e = &events.SAMLConnectorCreate{}
	case SAMLConnectorDeletedEvent:
		e = &events.SAMLConnectorDelete{}
	case SessionRejectedEvent:
		e = &events.SessionReject{}
	case AppSessionStartEvent:
		e = &events.AppSessionStart{}
	case AppSessionEndEvent:
		e = &events.AppSessionEnd{}
	case AppSessionChunkEvent:
		e = &events.AppSessionChunk{}
	case AppSessionRequestEvent:
		e = &events.AppSessionRequest{}
	case AppSessionDynamoDBRequestEvent:
		e = &events.AppSessionDynamoDBRequest{}
	case AppCreateEvent:
		e = &events.AppCreate{}
	case AppUpdateEvent:
		e = &events.AppUpdate{}
	case AppDeleteEvent:
		e = &events.AppDelete{}
	case DatabaseCreateEvent:
		e = &events.DatabaseCreate{}
	case DatabaseUpdateEvent:
		e = &events.DatabaseUpdate{}
	case DatabaseDeleteEvent:
		e = &events.DatabaseDelete{}
	case DatabaseSessionStartEvent:
		e = &events.DatabaseSessionStart{}
	case DatabaseSessionEndEvent:
		e = &events.DatabaseSessionEnd{}
	case DatabaseSessionQueryEvent, DatabaseSessionQueryFailedEvent:
		e = &events.DatabaseSessionQuery{}
	case DatabaseSessionMalformedPacketEvent:
		e = &events.DatabaseSessionMalformedPacket{}
	case DatabaseSessionPostgresParseEvent:
		e = &events.PostgresParse{}
	case DatabaseSessionPostgresBindEvent:
		e = &events.PostgresBind{}
	case DatabaseSessionPostgresExecuteEvent:
		e = &events.PostgresExecute{}
	case DatabaseSessionPostgresCloseEvent:
		e = &events.PostgresClose{}
	case DatabaseSessionPostgresFunctionEvent:
		e = &events.PostgresFunctionCall{}
	case DatabaseSessionMySQLStatementPrepareEvent:
		e = &events.MySQLStatementPrepare{}
	case DatabaseSessionMySQLStatementExecuteEvent:
		e = &events.MySQLStatementExecute{}
	case DatabaseSessionMySQLStatementSendLongDataEvent:
		e = &events.MySQLStatementSendLongData{}
	case DatabaseSessionMySQLStatementCloseEvent:
		e = &events.MySQLStatementClose{}
	case DatabaseSessionMySQLStatementResetEvent:
		e = &events.MySQLStatementReset{}
	case DatabaseSessionMySQLStatementFetchEvent:
		e = &events.MySQLStatementFetch{}
	case DatabaseSessionMySQLStatementBulkExecuteEvent:
		e = &events.MySQLStatementBulkExecute{}
	case DatabaseSessionMySQLInitDBEvent:
		e = &events.MySQLInitDB{}
	case DatabaseSessionMySQLCreateDBEvent:
		e = &events.MySQLCreateDB{}
	case DatabaseSessionMySQLDropDBEvent:
		e = &events.MySQLDropDB{}
	case DatabaseSessionMySQLShutDownEvent:
		e = &events.MySQLShutDown{}
	case DatabaseSessionMySQLProcessKillEvent:
		e = &events.MySQLProcessKill{}
	case DatabaseSessionMySQLDebugEvent:
		e = &events.MySQLDebug{}
	case DatabaseSessionMySQLRefreshEvent:
		e = &events.MySQLRefresh{}
	case DatabaseSessionSQLServerRPCRequestEvent:
		e = &events.SQLServerRPCRequest{}
	case DatabaseSessionElasticsearchRequestEvent:
		e = &events.ElasticsearchRequest{}
	case DatabaseSessionOpenSearchRequestEvent:
		e = &events.OpenSearchRequest{}
	case DatabaseSessionDynamoDBRequestEvent:
		e = &events.DynamoDBRequest{}
	case KubeRequestEvent:
		e = &events.KubeRequest{}
	case MFADeviceAddEvent:
		e = &events.MFADeviceAdd{}
	case MFADeviceDeleteEvent:
		e = &events.MFADeviceDelete{}
	case DeviceEvent: // Kept for backwards compatibility.
		e = &events.DeviceEvent{}
	case DeviceCreateEvent, DeviceDeleteEvent, DeviceUpdateEvent,
		DeviceEnrollEvent, DeviceAuthenticateEvent,
		DeviceEnrollTokenCreateEvent:
		e = &events.DeviceEvent2{}
	case LockCreatedEvent:
		e = &events.LockCreate{}
	case LockDeletedEvent:
		e = &events.LockDelete{}
	case RecoveryCodeGeneratedEvent:
		e = &events.RecoveryCodeGenerate{}
	case RecoveryCodeUsedEvent:
		e = &events.RecoveryCodeUsed{}
	case RecoveryTokenCreateEvent:
		e = &events.UserTokenCreate{}
	case PrivilegeTokenCreateEvent:
		e = &events.UserTokenCreate{}
	case WindowsDesktopSessionStartEvent:
		e = &events.WindowsDesktopSessionStart{}
	case WindowsDesktopSessionEndEvent:
		e = &events.WindowsDesktopSessionEnd{}
	case DesktopRecordingEvent:
		e = &events.DesktopRecording{}
	case DesktopClipboardSendEvent:
		e = &events.DesktopClipboardSend{}
	case DesktopClipboardReceiveEvent:
		e = &events.DesktopClipboardReceive{}
	case SessionConnectEvent:
		e = &events.SessionConnect{}
	case AccessRequestDeleteEvent:
		e = &events.AccessRequestDelete{}
	case CertificateCreateEvent:
		e = &events.CertificateCreate{}
	case RenewableCertificateGenerationMismatchEvent:
		e = &events.RenewableCertificateGenerationMismatch{}
	case SFTPEvent:
		e = &events.SFTP{}
	case UpgradeWindowStartUpdateEvent:
		e = &events.UpgradeWindowStartUpdate{}
	case SessionRecordingAccessEvent:
		e = &events.SessionRecordingAccess{}
	case SSMRunEvent:
		e = &events.SSMRun{}
	case KubernetesClusterCreateEvent:
		e = &events.KubernetesClusterCreate{}
	case KubernetesClusterUpdateEvent:
		e = &events.KubernetesClusterUpdate{}
	case KubernetesClusterDeleteEvent:
		e = &events.KubernetesClusterDelete{}
	case DesktopSharedDirectoryStartEvent:
		e = &events.DesktopSharedDirectoryStart{}
	case DesktopSharedDirectoryReadEvent:
		e = &events.DesktopSharedDirectoryRead{}
	case DesktopSharedDirectoryWriteEvent:
		e = &events.DesktopSharedDirectoryWrite{}
	case BotJoinEvent:
		e = &events.BotJoin{}
	case InstanceJoinEvent:
		e = &events.InstanceJoin{}
	case LoginRuleCreateEvent:
		e = &events.LoginRuleCreate{}
	case LoginRuleDeleteEvent:
		e = &events.LoginRuleDelete{}
	case SAMLIdPAuthAttemptEvent:
		e = &events.SAMLIdPAuthAttempt{}
	case SAMLIdPServiceProviderCreateEvent:
		e = &events.SAMLIdPServiceProviderCreate{}
	case SAMLIdPServiceProviderUpdateEvent:
		e = &events.SAMLIdPServiceProviderUpdate{}
	case SAMLIdPServiceProviderDeleteEvent:
		e = &events.SAMLIdPServiceProviderDelete{}
	case SAMLIdPServiceProviderDeleteAllEvent:
		e = &events.SAMLIdPServiceProviderDeleteAll{}
	case OktaGroupsUpdateEvent:
		e = &events.OktaResourcesUpdate{}
	case OktaApplicationsUpdateEvent:
		e = &events.OktaResourcesUpdate{}
	case OktaSyncFailureEvent:
		e = &events.OktaSyncFailure{}
	case OktaAssignmentProcessEvent:
		e = &events.OktaAssignmentResult{}
	case OktaAssignmentCleanupEvent:
		e = &events.OktaAssignmentResult{}
	case AccessListCreateEvent:
		e = &events.AccessListCreate{}
	case AccessListUpdateEvent:
		e = &events.AccessListUpdate{}
	case AccessListDeleteEvent:
		e = &events.AccessListDelete{}
	case AccessListReviewEvent:
		e = &events.AccessListReview{}
	case AccessListMemberCreateEvent:
		e = &events.AccessListMemberCreate{}
	case AccessListMemberUpdateEvent:
		e = &events.AccessListMemberUpdate{}
	case AccessListMemberDeleteEvent:
		e = &events.AccessListMemberDelete{}
	case AccessListMemberDeleteAllForAccessListEvent:
		e = &events.AccessListMemberDeleteAllForAccessList{}
	case SecReportsAuditQueryRunEvent:
		e = &events.AuditQueryRun{}
	case SecReportsReportRunEvent:
		e = &events.SecurityReportRun{}
	case ExternalAuditStorageEnableEvent:
		e = &events.ExternalAuditStorageEnable{}
	case ExternalAuditStorageDisableEvent:
		e = &events.ExternalAuditStorageDisable{}

	case UnknownEvent:
		e = &events.Unknown{}

	case DatabaseSessionCassandraBatchEvent:
		e = &events.CassandraBatch{}
	case DatabaseSessionCassandraRegisterEvent:
		e = &events.CassandraRegister{}
	case DatabaseSessionCassandraPrepareEvent:
		e = &events.CassandraPrepare{}
	case DatabaseSessionCassandraExecuteEvent:
		e = &events.CassandraExecute{}

	case DiscoveryConfigCreateEvent:
		e = &events.DiscoveryConfigCreate{}
	case DiscoveryConfigUpdateEvent:
		e = &events.DiscoveryConfigUpdate{}
	case DiscoveryConfigDeleteEvent:
		e = &events.DiscoveryConfigDelete{}
	case DiscoveryConfigDeleteAllEvent:
		e = &events.DiscoveryConfigDeleteAll{}

	case IntegrationCreateEvent:
		e = &events.IntegrationCreate{}
	case IntegrationUpdateEvent:
		e = &events.IntegrationUpdate{}
	case IntegrationDeleteEvent:
		e = &events.IntegrationDelete{}

	case AutoUpdateConfigCreateEvent:
		e = &events.AutoUpdateConfigCreate{}
	case AutoUpdateConfigUpdateEvent:
		e = &events.AutoUpdateConfigUpdate{}
	case AutoUpdateConfigDeleteEvent:
		e = &events.AutoUpdateConfigDelete{}

	case AutoUpdateVersionCreateEvent:
		e = &events.AutoUpdateVersionCreate{}
	case AutoUpdateVersionUpdateEvent:
		e = &events.AutoUpdateVersionUpdate{}
	case AutoUpdateVersionDeleteEvent:
		e = &events.AutoUpdateVersionDelete{}
	default:
		log.Errorf("Attempted to convert dynamic event of unknown type %q into protobuf event.", eventType)
		unknown := &events.Unknown{}
		if err := utils.FastUnmarshal(data, unknown); err != nil {
			return nil, trace.Wrap(err)
		}

		unknown.Type = UnknownEvent
		unknown.Code = UnknownCode
		unknown.UnknownType = eventType
		unknown.UnknownCode = getFieldEmpty(EventCode)
		unknown.Data = string(data)
		return unknown, nil
	}

	if err := utils.FastUnmarshal(data, e); err != nil {
		return nil, trace.Wrap(err)
	}

	return e, nil
}

// GetSessionID pulls the session ID from the events that have a
// SessionMetadata. For other events an empty string is returned.
func GetSessionID(event events.AuditEvent) string {
	var sessionID string

	if g, ok := event.(SessionMetadataGetter); ok {
		sessionID = g.GetSessionID()
	}

	return sessionID
}

// GetTeleportUser pulls the teleport user from the events that have a
// UserMetadata. For other events an empty string is returned.
func GetTeleportUser(event events.AuditEvent) string {
	type userGetter interface {
		GetUser() string
	}
	if g, ok := event.(userGetter); ok {
		return g.GetUser()
	}
	return ""
}

// ToEventFields converts from the typed interface-style event representation
// to the old dynamic map style representation in order to provide outer compatibility
// with existing public API routes when the backend is updated with the typed events.
func ToEventFields(event events.AuditEvent) (EventFields, error) {
	var fields EventFields
	if err := apiutils.ObjectToStruct(event, &fields); err != nil {
		return nil, trace.Wrap(err)
	}

	return fields, nil
}
