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
		e = &events.UserUpdate{}
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
	case RoleUpdatedEvent:
		e = &events.RoleUpdate{}
	case RoleDeletedEvent:
		e = &events.RoleDelete{}
	case TrustedClusterCreateEvent:
		e = &events.TrustedClusterCreate{}
	case TrustedClusterDeleteEvent:
		e = &events.TrustedClusterDelete{}
	case TrustedClusterTokenCreateEvent:
		//nolint:staticcheck // We still need to support viewing the deprecated event
		// type for backwards compatibility.
		e = &events.TrustedClusterTokenCreate{}
	case ProvisionTokenCreateEvent:
		e = &events.ProvisionTokenCreate{}
	case GithubConnectorCreatedEvent:
		e = &events.GithubConnectorCreate{}
	case GithubConnectorUpdatedEvent:
		e = &events.GithubConnectorUpdate{}
	case GithubConnectorDeletedEvent:
		e = &events.GithubConnectorDelete{}
	case OIDCConnectorCreatedEvent:
		e = &events.OIDCConnectorCreate{}
	case OIDCConnectorUpdatedEvent:
		e = &events.OIDCConnectorUpdate{}
	case OIDCConnectorDeletedEvent:
		e = &events.OIDCConnectorDelete{}
	case SAMLConnectorCreatedEvent:
		e = &events.SAMLConnectorCreate{}
	case SAMLConnectorUpdatedEvent:
		e = &events.SAMLConnectorUpdate{}
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
	case DatabaseSessionPermissionsUpdateEvent:
		e = &events.DatabasePermissionUpdate{}
	case DatabaseSessionUserCreateEvent:
		e = &events.DatabaseUserCreate{}
	case DatabaseSessionUserDeactivateEvent:
		e = &events.DatabaseUserDeactivate{}
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
	case BotCreateEvent:
		e = &events.BotCreate{}
	case BotUpdateEvent:
		e = &events.BotUpdate{}
	case BotDeleteEvent:
		e = &events.BotDelete{}
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
	case OktaAccessListSyncEvent:
		e = &events.OktaAccessListSync{}
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
	case CreateMFAAuthChallengeEvent:
		e = &events.CreateMFAAuthChallenge{}
	case ValidateMFAAuthResponseEvent:
		e = &events.ValidateMFAAuthResponse{}
	case SPIFFESVIDIssuedEvent:
		e = &events.SPIFFESVIDIssued{}
	case AuthPreferenceUpdateEvent:
		e = &events.AuthPreferenceUpdate{}
	case ClusterNetworkingConfigUpdateEvent:
		e = &events.ClusterNetworkingConfigUpdate{}
	case SessionRecordingConfigUpdateEvent:
		e = &events.SessionRecordingConfigUpdate{}

	case UnknownEvent:
		e = &events.Unknown{}

	case CassandraBatchEventCode:
		e = &events.CassandraBatch{}
	case CassandraRegisterEventCode:
		e = &events.CassandraRegister{}
	case CassandraPrepareEventCode:
		e = &events.CassandraPrepare{}
	case CassandraExecuteEventCode:
		e = &events.CassandraExecute{}

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
