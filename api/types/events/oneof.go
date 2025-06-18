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
	"context"
	"encoding/json"
	"log/slog"
	"reflect"

	"github.com/gravitational/trace"
)

// MustToOneOf converts audit event to OneOf
// or panics, used in tests
func MustToOneOf(in AuditEvent) *OneOf {
	out, err := ToOneOf(in)
	if err != nil {
		panic(err)
	}
	return out
}

// ToOneOf converts audit event to union type of the events
func ToOneOf(in AuditEvent) (*OneOf, error) {
	out := OneOf{}

	switch e := in.(type) {
	case *UserLogin:
		out.Event = &OneOf_UserLogin{
			UserLogin: e,
		}
	case *UserCreate:
		out.Event = &OneOf_UserCreate{
			UserCreate: e,
		}
	case *UserUpdate:
		out.Event = &OneOf_UserUpdate{
			UserUpdate: e,
		}
	case *UserDelete:
		out.Event = &OneOf_UserDelete{
			UserDelete: e,
		}
	case *UserPasswordChange:
		out.Event = &OneOf_UserPasswordChange{
			UserPasswordChange: e,
		}
	case *SessionStart:
		out.Event = &OneOf_SessionStart{
			SessionStart: e,
		}
	case *SessionJoin:
		out.Event = &OneOf_SessionJoin{
			SessionJoin: e,
		}
	case *SessionPrint:
		out.Event = &OneOf_SessionPrint{
			SessionPrint: e,
		}
	case *SessionReject:
		out.Event = &OneOf_SessionReject{
			SessionReject: e,
		}
	case *Resize:
		out.Event = &OneOf_Resize{
			Resize: e,
		}
	case *SessionEnd:
		out.Event = &OneOf_SessionEnd{
			SessionEnd: e,
		}
	case *SessionCommand:
		out.Event = &OneOf_SessionCommand{
			SessionCommand: e,
		}
	case *SessionDisk:
		out.Event = &OneOf_SessionDisk{
			SessionDisk: e,
		}
	case *SessionNetwork:
		out.Event = &OneOf_SessionNetwork{
			SessionNetwork: e,
		}
	case *SessionData:
		out.Event = &OneOf_SessionData{
			SessionData: e,
		}
	case *SessionLeave:
		out.Event = &OneOf_SessionLeave{
			SessionLeave: e,
		}
	case *PortForward:
		out.Event = &OneOf_PortForward{
			PortForward: e,
		}
	case *X11Forward:
		out.Event = &OneOf_X11Forward{
			X11Forward: e,
		}
	case *Subsystem:
		out.Event = &OneOf_Subsystem{
			Subsystem: e,
		}
	case *SCP:
		out.Event = &OneOf_SCP{
			SCP: e,
		}
	case *Exec:
		out.Event = &OneOf_Exec{
			Exec: e,
		}
	case *ClientDisconnect:
		out.Event = &OneOf_ClientDisconnect{
			ClientDisconnect: e,
		}
	case *AuthAttempt:
		out.Event = &OneOf_AuthAttempt{
			AuthAttempt: e,
		}
	case *AccessRequestCreate:
		out.Event = &OneOf_AccessRequestCreate{
			AccessRequestCreate: e,
		}
	case *AccessRequestExpire:
		out.Event = &OneOf_AccessRequestExpire{
			AccessRequestExpire: e,
		}
	case *AccessRequestResourceSearch:
		out.Event = &OneOf_AccessRequestResourceSearch{
			AccessRequestResourceSearch: e,
		}
	case *RoleCreate:
		out.Event = &OneOf_RoleCreate{
			RoleCreate: e,
		}
	case *RoleUpdate:
		out.Event = &OneOf_RoleUpdate{
			RoleUpdate: e,
		}
	case *RoleDelete:
		out.Event = &OneOf_RoleDelete{
			RoleDelete: e,
		}
	case *UserTokenCreate:
		out.Event = &OneOf_UserTokenCreate{
			UserTokenCreate: e,
		}
	case *TrustedClusterCreate:
		out.Event = &OneOf_TrustedClusterCreate{
			TrustedClusterCreate: e,
		}
	case *TrustedClusterDelete:
		out.Event = &OneOf_TrustedClusterDelete{
			TrustedClusterDelete: e,
		}
	case *TrustedClusterTokenCreate:
		out.Event = &OneOf_TrustedClusterTokenCreate{
			TrustedClusterTokenCreate: e,
		}
	case *ProvisionTokenCreate:
		out.Event = &OneOf_ProvisionTokenCreate{
			ProvisionTokenCreate: e,
		}
	case *GithubConnectorCreate:
		out.Event = &OneOf_GithubConnectorCreate{
			GithubConnectorCreate: e,
		}
	case *GithubConnectorUpdate:
		out.Event = &OneOf_GithubConnectorUpdate{
			GithubConnectorUpdate: e,
		}
	case *GithubConnectorDelete:
		out.Event = &OneOf_GithubConnectorDelete{
			GithubConnectorDelete: e,
		}
	case *OIDCConnectorCreate:
		out.Event = &OneOf_OIDCConnectorCreate{
			OIDCConnectorCreate: e,
		}
	case *OIDCConnectorUpdate:
		out.Event = &OneOf_OIDCConnectorUpdate{
			OIDCConnectorUpdate: e,
		}
	case *OIDCConnectorDelete:
		out.Event = &OneOf_OIDCConnectorDelete{
			OIDCConnectorDelete: e,
		}
	case *SAMLConnectorCreate:
		out.Event = &OneOf_SAMLConnectorCreate{
			SAMLConnectorCreate: e,
		}
	case *SAMLConnectorUpdate:
		out.Event = &OneOf_SAMLConnectorUpdate{
			SAMLConnectorUpdate: e,
		}
	case *SAMLConnectorDelete:
		out.Event = &OneOf_SAMLConnectorDelete{
			SAMLConnectorDelete: e,
		}
	case *KubeRequest:
		out.Event = &OneOf_KubeRequest{
			KubeRequest: e,
		}
	case *AppSessionStart:
		out.Event = &OneOf_AppSessionStart{
			AppSessionStart: e,
		}
	case *AppSessionEnd:
		out.Event = &OneOf_AppSessionEnd{
			AppSessionEnd: e,
		}
	case *AppSessionChunk:
		out.Event = &OneOf_AppSessionChunk{
			AppSessionChunk: e,
		}
	case *AppSessionRequest:
		out.Event = &OneOf_AppSessionRequest{
			AppSessionRequest: e,
		}
	case *AppSessionDynamoDBRequest:
		out.Event = &OneOf_AppSessionDynamoDBRequest{
			AppSessionDynamoDBRequest: e,
		}
	case *AppCreate:
		out.Event = &OneOf_AppCreate{
			AppCreate: e,
		}
	case *AppUpdate:
		out.Event = &OneOf_AppUpdate{
			AppUpdate: e,
		}
	case *AppDelete:
		out.Event = &OneOf_AppDelete{
			AppDelete: e,
		}
	case *DatabaseCreate:
		out.Event = &OneOf_DatabaseCreate{
			DatabaseCreate: e,
		}
	case *DatabaseUpdate:
		out.Event = &OneOf_DatabaseUpdate{
			DatabaseUpdate: e,
		}
	case *DatabaseDelete:
		out.Event = &OneOf_DatabaseDelete{
			DatabaseDelete: e,
		}
	case *DatabaseSessionStart:
		out.Event = &OneOf_DatabaseSessionStart{
			DatabaseSessionStart: e,
		}
	case *DatabaseSessionEnd:
		out.Event = &OneOf_DatabaseSessionEnd{
			DatabaseSessionEnd: e,
		}
	case *DatabaseSessionQuery:
		out.Event = &OneOf_DatabaseSessionQuery{
			DatabaseSessionQuery: e,
		}
	case *DatabasePermissionUpdate:
		out.Event = &OneOf_DatabasePermissionUpdate{
			DatabasePermissionUpdate: e,
		}
	case *PostgresParse:
		out.Event = &OneOf_PostgresParse{
			PostgresParse: e,
		}
	case *PostgresBind:
		out.Event = &OneOf_PostgresBind{
			PostgresBind: e,
		}
	case *PostgresExecute:
		out.Event = &OneOf_PostgresExecute{
			PostgresExecute: e,
		}
	case *PostgresClose:
		out.Event = &OneOf_PostgresClose{
			PostgresClose: e,
		}
	case *PostgresFunctionCall:
		out.Event = &OneOf_PostgresFunctionCall{
			PostgresFunctionCall: e,
		}
	case *SessionUpload:
		out.Event = &OneOf_SessionUpload{
			SessionUpload: e,
		}
	case *MFADeviceAdd:
		out.Event = &OneOf_MFADeviceAdd{
			MFADeviceAdd: e,
		}
	case *MFADeviceDelete:
		out.Event = &OneOf_MFADeviceDelete{
			MFADeviceDelete: e,
		}
	case *DeviceEvent:
		out.Event = &OneOf_DeviceEvent{
			DeviceEvent: e,
		}
	case *DeviceEvent2:
		out.Event = &OneOf_DeviceEvent2{
			DeviceEvent2: e,
		}
	case *BillingCardCreate:
		out.Event = &OneOf_BillingCardCreate{
			BillingCardCreate: e,
		}
	case *BillingCardDelete:
		out.Event = &OneOf_BillingCardDelete{
			BillingCardDelete: e,
		}
	case *LockCreate:
		out.Event = &OneOf_LockCreate{
			LockCreate: e,
		}
	case *LockDelete:
		out.Event = &OneOf_LockDelete{
			LockDelete: e,
		}
	case *BillingInformationUpdate:
		out.Event = &OneOf_BillingInformationUpdate{
			BillingInformationUpdate: e,
		}
	case *RecoveryCodeGenerate:
		out.Event = &OneOf_RecoveryCodeGenerate{
			RecoveryCodeGenerate: e,
		}
	case *RecoveryCodeUsed:
		out.Event = &OneOf_RecoveryCodeUsed{
			RecoveryCodeUsed: e,
		}
	case *WindowsDesktopSessionStart:
		out.Event = &OneOf_WindowsDesktopSessionStart{
			WindowsDesktopSessionStart: e,
		}
	case *WindowsDesktopSessionEnd:
		out.Event = &OneOf_WindowsDesktopSessionEnd{
			WindowsDesktopSessionEnd: e,
		}
	case *SessionConnect:
		out.Event = &OneOf_SessionConnect{
			SessionConnect: e,
		}
	case *AccessRequestDelete:
		out.Event = &OneOf_AccessRequestDelete{
			AccessRequestDelete: e,
		}
	case *CertificateCreate:
		out.Event = &OneOf_CertificateCreate{
			CertificateCreate: e,
		}
	case *DesktopRecording:
		out.Event = &OneOf_DesktopRecording{
			DesktopRecording: e,
		}
	case *DesktopClipboardReceive:
		out.Event = &OneOf_DesktopClipboardReceive{
			DesktopClipboardReceive: e,
		}
	case *DesktopClipboardSend:
		out.Event = &OneOf_DesktopClipboardSend{
			DesktopClipboardSend: e,
		}
	case *MySQLStatementPrepare:
		out.Event = &OneOf_MySQLStatementPrepare{
			MySQLStatementPrepare: e,
		}
	case *MySQLStatementExecute:
		out.Event = &OneOf_MySQLStatementExecute{
			MySQLStatementExecute: e,
		}
	case *MySQLStatementSendLongData:
		out.Event = &OneOf_MySQLStatementSendLongData{
			MySQLStatementSendLongData: e,
		}
	case *MySQLStatementClose:
		out.Event = &OneOf_MySQLStatementClose{
			MySQLStatementClose: e,
		}
	case *MySQLStatementReset:
		out.Event = &OneOf_MySQLStatementReset{
			MySQLStatementReset: e,
		}
	case *MySQLStatementFetch:
		out.Event = &OneOf_MySQLStatementFetch{
			MySQLStatementFetch: e,
		}
	case *MySQLStatementBulkExecute:
		out.Event = &OneOf_MySQLStatementBulkExecute{
			MySQLStatementBulkExecute: e,
		}
	case *MySQLInitDB:
		out.Event = &OneOf_MySQLInitDB{
			MySQLInitDB: e,
		}
	case *MySQLCreateDB:
		out.Event = &OneOf_MySQLCreateDB{
			MySQLCreateDB: e,
		}
	case *MySQLDropDB:
		out.Event = &OneOf_MySQLDropDB{
			MySQLDropDB: e,
		}
	case *MySQLShutDown:
		out.Event = &OneOf_MySQLShutDown{
			MySQLShutDown: e,
		}
	case *MySQLProcessKill:
		out.Event = &OneOf_MySQLProcessKill{
			MySQLProcessKill: e,
		}
	case *MySQLDebug:
		out.Event = &OneOf_MySQLDebug{
			MySQLDebug: e,
		}
	case *MySQLRefresh:
		out.Event = &OneOf_MySQLRefresh{
			MySQLRefresh: e,
		}
	case *SQLServerRPCRequest:
		out.Event = &OneOf_SQLServerRPCRequest{
			SQLServerRPCRequest: e,
		}
	case *ElasticsearchRequest:
		out.Event = &OneOf_ElasticsearchRequest{
			ElasticsearchRequest: e,
		}
	case *OpenSearchRequest:
		out.Event = &OneOf_OpenSearchRequest{
			OpenSearchRequest: e,
		}
	case *DynamoDBRequest:
		out.Event = &OneOf_DynamoDBRequest{
			DynamoDBRequest: e,
		}
	case *DatabaseSessionMalformedPacket:
		out.Event = &OneOf_DatabaseSessionMalformedPacket{
			DatabaseSessionMalformedPacket: e,
		}
	case *RenewableCertificateGenerationMismatch:
		out.Event = &OneOf_RenewableCertificateGenerationMismatch{
			RenewableCertificateGenerationMismatch: e,
		}
	case *SFTP:
		out.Event = &OneOf_SFTP{
			SFTP: e,
		}
	case *UpgradeWindowStartUpdate:
		out.Event = &OneOf_UpgradeWindowStartUpdate{
			UpgradeWindowStartUpdate: e,
		}
	case *SessionRecordingAccess:
		out.Event = &OneOf_SessionRecordingAccess{
			SessionRecordingAccess: e,
		}
	case *SSMRun:
		out.Event = &OneOf_SSMRun{
			SSMRun: e,
		}
	case *Unknown:
		out.Event = &OneOf_Unknown{
			Unknown: e,
		}
	case *CassandraBatch:
		out.Event = &OneOf_CassandraBatch{
			CassandraBatch: e,
		}
	case *CassandraPrepare:
		out.Event = &OneOf_CassandraPrepare{
			CassandraPrepare: e,
		}
	case *CassandraRegister:
		out.Event = &OneOf_CassandraRegister{
			CassandraRegister: e,
		}
	case *CassandraExecute:
		out.Event = &OneOf_CassandraExecute{
			CassandraExecute: e,
		}
	case *KubernetesClusterCreate:
		out.Event = &OneOf_KubernetesClusterCreate{
			KubernetesClusterCreate: e,
		}
	case *KubernetesClusterUpdate:
		out.Event = &OneOf_KubernetesClusterUpdate{
			KubernetesClusterUpdate: e,
		}
	case *KubernetesClusterDelete:
		out.Event = &OneOf_KubernetesClusterDelete{
			KubernetesClusterDelete: e,
		}
	case *DesktopSharedDirectoryStart:
		out.Event = &OneOf_DesktopSharedDirectoryStart{
			DesktopSharedDirectoryStart: e,
		}
	case *DesktopSharedDirectoryRead:
		out.Event = &OneOf_DesktopSharedDirectoryRead{
			DesktopSharedDirectoryRead: e,
		}
	case *DesktopSharedDirectoryWrite:
		out.Event = &OneOf_DesktopSharedDirectoryWrite{
			DesktopSharedDirectoryWrite: e,
		}
	case *BotJoin:
		out.Event = &OneOf_BotJoin{
			BotJoin: e,
		}
	case *InstanceJoin:
		out.Event = &OneOf_InstanceJoin{
			InstanceJoin: e,
		}
	case *BotCreate:
		out.Event = &OneOf_BotCreate{
			BotCreate: e,
		}
	case *BotUpdate:
		out.Event = &OneOf_BotUpdate{
			BotUpdate: e,
		}
	case *BotDelete:
		out.Event = &OneOf_BotDelete{
			BotDelete: e,
		}
	case *LoginRuleCreate:
		out.Event = &OneOf_LoginRuleCreate{
			LoginRuleCreate: e,
		}
	case *LoginRuleDelete:
		out.Event = &OneOf_LoginRuleDelete{
			LoginRuleDelete: e,
		}
	case *SAMLIdPAuthAttempt:
		out.Event = &OneOf_SAMLIdPAuthAttempt{
			SAMLIdPAuthAttempt: e,
		}
	case *SAMLIdPServiceProviderCreate:
		out.Event = &OneOf_SAMLIdPServiceProviderCreate{
			SAMLIdPServiceProviderCreate: e,
		}
	case *SAMLIdPServiceProviderUpdate:
		out.Event = &OneOf_SAMLIdPServiceProviderUpdate{
			SAMLIdPServiceProviderUpdate: e,
		}
	case *SAMLIdPServiceProviderDelete:
		out.Event = &OneOf_SAMLIdPServiceProviderDelete{
			SAMLIdPServiceProviderDelete: e,
		}
	case *SAMLIdPServiceProviderDeleteAll:
		out.Event = &OneOf_SAMLIdPServiceProviderDeleteAll{
			SAMLIdPServiceProviderDeleteAll: e,
		}
	case *OktaResourcesUpdate:
		out.Event = &OneOf_OktaResourcesUpdate{
			OktaResourcesUpdate: e,
		}
	case *OktaSyncFailure:
		out.Event = &OneOf_OktaSyncFailure{
			OktaSyncFailure: e,
		}
	case *OktaAssignmentResult:
		out.Event = &OneOf_OktaAssignmentResult{
			OktaAssignmentResult: e,
		}
	case *AccessListCreate:
		out.Event = &OneOf_AccessListCreate{
			AccessListCreate: e,
		}
	case *AccessListUpdate:
		out.Event = &OneOf_AccessListUpdate{
			AccessListUpdate: e,
		}
	case *AccessListDelete:
		out.Event = &OneOf_AccessListDelete{
			AccessListDelete: e,
		}
	case *AccessListReview:
		out.Event = &OneOf_AccessListReview{
			AccessListReview: e,
		}
	case *AccessListMemberCreate:
		out.Event = &OneOf_AccessListMemberCreate{
			AccessListMemberCreate: e,
		}
	case *AccessListMemberUpdate:
		out.Event = &OneOf_AccessListMemberUpdate{
			AccessListMemberUpdate: e,
		}
	case *AccessListMemberDelete:
		out.Event = &OneOf_AccessListMemberDelete{
			AccessListMemberDelete: e,
		}
	case *AccessListMemberDeleteAllForAccessList:
		out.Event = &OneOf_AccessListMemberDeleteAllForAccessList{
			AccessListMemberDeleteAllForAccessList: e,
		}
	case *UserLoginAccessListInvalid:
		out.Event = &OneOf_UserLoginAccessListInvalid{
			UserLoginAccessListInvalid: e,
		}
	case *AuditQueryRun:
		out.Event = &OneOf_AuditQueryRun{
			AuditQueryRun: e,
		}
	case *SecurityReportRun:
		out.Event = &OneOf_SecurityReportRun{
			SecurityReportRun: e,
		}
	case *ExternalAuditStorageEnable:
		out.Event = &OneOf_ExternalAuditStorageEnable{
			ExternalAuditStorageEnable: e,
		}
	case *ExternalAuditStorageDisable:
		out.Event = &OneOf_ExternalAuditStorageDisable{
			ExternalAuditStorageDisable: e,
		}
	case *CreateMFAAuthChallenge:
		out.Event = &OneOf_CreateMFAAuthChallenge{
			CreateMFAAuthChallenge: e,
		}
	case *ValidateMFAAuthResponse:
		out.Event = &OneOf_ValidateMFAAuthResponse{
			ValidateMFAAuthResponse: e,
		}
	case *OktaAccessListSync:
		out.Event = &OneOf_OktaAccessListSync{
			OktaAccessListSync: e,
		}
	case *OktaUserSync:
		out.Event = &OneOf_OktaUserSync{
			OktaUserSync: e,
		}
	case *SPIFFESVIDIssued:
		out.Event = &OneOf_SPIFFESVIDIssued{
			SPIFFESVIDIssued: e,
		}
	case *AuthPreferenceUpdate:
		out.Event = &OneOf_AuthPreferenceUpdate{
			AuthPreferenceUpdate: e,
		}
	case *ClusterNetworkingConfigUpdate:
		out.Event = &OneOf_ClusterNetworkingConfigUpdate{
			ClusterNetworkingConfigUpdate: e,
		}
	case *SessionRecordingConfigUpdate:
		out.Event = &OneOf_SessionRecordingConfigUpdate{
			SessionRecordingConfigUpdate: e,
		}
	case *AccessGraphSettingsUpdate:
		out.Event = &OneOf_AccessGraphSettingsUpdate{
			AccessGraphSettingsUpdate: e,
		}
	case *DatabaseUserCreate:
		out.Event = &OneOf_DatabaseUserCreate{
			DatabaseUserCreate: e,
		}
	case *DatabaseUserDeactivate:
		out.Event = &OneOf_DatabaseUserDeactivate{
			DatabaseUserDeactivate: e,
		}
	case *AccessPathChanged:
		out.Event = &OneOf_AccessPathChanged{
			AccessPathChanged: e,
		}
	case *SpannerRPC:
		out.Event = &OneOf_SpannerRPC{
			SpannerRPC: e,
		}
	case *DatabaseSessionCommandResult:
		out.Event = &OneOf_DatabaseSessionCommandResult{
			DatabaseSessionCommandResult: e,
		}
	case *DiscoveryConfigCreate:
		out.Event = &OneOf_DiscoveryConfigCreate{
			DiscoveryConfigCreate: e,
		}
	case *DiscoveryConfigUpdate:
		out.Event = &OneOf_DiscoveryConfigUpdate{
			DiscoveryConfigUpdate: e,
		}
	case *DiscoveryConfigDelete:
		out.Event = &OneOf_DiscoveryConfigDelete{
			DiscoveryConfigDelete: e,
		}
	case *DiscoveryConfigDeleteAll:
		out.Event = &OneOf_DiscoveryConfigDeleteAll{
			DiscoveryConfigDeleteAll: e,
		}
	case *IntegrationCreate:
		out.Event = &OneOf_IntegrationCreate{
			IntegrationCreate: e,
		}
	case *IntegrationUpdate:
		out.Event = &OneOf_IntegrationUpdate{
			IntegrationUpdate: e,
		}
	case *IntegrationDelete:
		out.Event = &OneOf_IntegrationDelete{
			IntegrationDelete: e,
		}
	case *SPIFFEFederationCreate:
		out.Event = &OneOf_SPIFFEFederationCreate{
			SPIFFEFederationCreate: e,
		}
	case *SPIFFEFederationDelete:
		out.Event = &OneOf_SPIFFEFederationDelete{
			SPIFFEFederationDelete: e,
		}

	case *PluginCreate:
		out.Event = &OneOf_PluginCreate{
			PluginCreate: e,
		}
	case *PluginUpdate:
		out.Event = &OneOf_PluginUpdate{
			PluginUpdate: e,
		}
	case *PluginDelete:
		out.Event = &OneOf_PluginDelete{
			PluginDelete: e,
		}
	case *StaticHostUserCreate:
		out.Event = &OneOf_StaticHostUserCreate{
			StaticHostUserCreate: e,
		}
	case *StaticHostUserUpdate:
		out.Event = &OneOf_StaticHostUserUpdate{
			StaticHostUserUpdate: e,
		}
	case *StaticHostUserDelete:
		out.Event = &OneOf_StaticHostUserDelete{
			StaticHostUserDelete: e,
		}
	case *CrownJewelCreate:
		out.Event = &OneOf_CrownJewelCreate{
			CrownJewelCreate: e,
		}
	case *CrownJewelUpdate:
		out.Event = &OneOf_CrownJewelUpdate{
			CrownJewelUpdate: e,
		}
	case *CrownJewelDelete:
		out.Event = &OneOf_CrownJewelDelete{
			CrownJewelDelete: e,
		}
	case *UserTaskCreate:
		out.Event = &OneOf_UserTaskCreate{
			UserTaskCreate: e,
		}
	case *UserTaskUpdate:
		out.Event = &OneOf_UserTaskUpdate{
			UserTaskUpdate: e,
		}
	case *UserTaskDelete:
		out.Event = &OneOf_UserTaskDelete{
			UserTaskDelete: e,
		}
	case *SFTPSummary:
		out.Event = &OneOf_SFTPSummary{
			SFTPSummary: e,
		}
	case *AutoUpdateConfigCreate:
		out.Event = &OneOf_AutoUpdateConfigCreate{
			AutoUpdateConfigCreate: e,
		}
	case *AutoUpdateConfigUpdate:
		out.Event = &OneOf_AutoUpdateConfigUpdate{
			AutoUpdateConfigUpdate: e,
		}
	case *AutoUpdateConfigDelete:
		out.Event = &OneOf_AutoUpdateConfigDelete{
			AutoUpdateConfigDelete: e,
		}
	case *AutoUpdateVersionCreate:
		out.Event = &OneOf_AutoUpdateVersionCreate{
			AutoUpdateVersionCreate: e,
		}
	case *AutoUpdateVersionUpdate:
		out.Event = &OneOf_AutoUpdateVersionUpdate{
			AutoUpdateVersionUpdate: e,
		}
	case *AutoUpdateVersionDelete:
		out.Event = &OneOf_AutoUpdateVersionDelete{
			AutoUpdateVersionDelete: e,
		}
	case *AutoUpdateAgentRolloutTrigger:
		out.Event = &OneOf_AutoUpdateAgentRolloutTrigger{
			AutoUpdateAgentRolloutTrigger: e,
		}
	case *AutoUpdateAgentRolloutForceDone:
		out.Event = &OneOf_AutoUpdateAgentRolloutForceDone{
			AutoUpdateAgentRolloutForceDone: e,
		}
	case *AutoUpdateAgentRolloutRollback:
		out.Event = &OneOf_AutoUpdateAgentRolloutRollback{
			AutoUpdateAgentRolloutRollback: e,
		}
	case *ContactCreate:
		out.Event = &OneOf_ContactCreate{
			ContactCreate: e,
		}
	case *ContactDelete:
		out.Event = &OneOf_ContactDelete{
			ContactDelete: e,
		}

	case *WorkloadIdentityCreate:
		out.Event = &OneOf_WorkloadIdentityCreate{
			WorkloadIdentityCreate: e,
		}
	case *WorkloadIdentityUpdate:
		out.Event = &OneOf_WorkloadIdentityUpdate{
			WorkloadIdentityUpdate: e,
		}
	case *WorkloadIdentityDelete:
		out.Event = &OneOf_WorkloadIdentityDelete{
			WorkloadIdentityDelete: e,
		}
	case *GitCommand:
		out.Event = &OneOf_GitCommand{
			GitCommand: e,
		}
	case *StableUNIXUserCreate:
		out.Event = &OneOf_StableUNIXUserCreate{
			StableUNIXUserCreate: e,
		}
	case *WorkloadIdentityX509RevocationCreate:
		out.Event = &OneOf_WorkloadIdentityX509RevocationCreate{
			WorkloadIdentityX509RevocationCreate: e,
		}
	case *WorkloadIdentityX509RevocationDelete:
		out.Event = &OneOf_WorkloadIdentityX509RevocationDelete{
			WorkloadIdentityX509RevocationDelete: e,
		}
	case *WorkloadIdentityX509RevocationUpdate:
		out.Event = &OneOf_WorkloadIdentityX509RevocationUpdate{
			WorkloadIdentityX509RevocationUpdate: e,
		}
	case *AWSICResourceSync:
		out.Event = &OneOf_AWSICResourceSync{
			AWSICResourceSync: e,
		}
	case *HealthCheckConfigCreate:
		out.Event = &OneOf_HealthCheckConfigCreate{
			HealthCheckConfigCreate: e,
		}
	case *HealthCheckConfigUpdate:
		out.Event = &OneOf_HealthCheckConfigUpdate{
			HealthCheckConfigUpdate: e,
		}
	case *HealthCheckConfigDelete:
		out.Event = &OneOf_HealthCheckConfigDelete{
			HealthCheckConfigDelete: e,
		}
	case *WorkloadIdentityX509IssuerOverrideCreate:
		out.Event = &OneOf_WorkloadIdentityX509IssuerOverrideCreate{
			WorkloadIdentityX509IssuerOverrideCreate: e,
		}
	case *WorkloadIdentityX509IssuerOverrideDelete:
		out.Event = &OneOf_WorkloadIdentityX509IssuerOverrideDelete{
			WorkloadIdentityX509IssuerOverrideDelete: e,
		}
	case *SigstorePolicyCreate:
		out.Event = &OneOf_SigstorePolicyCreate{
			SigstorePolicyCreate: e,
		}
	case *SigstorePolicyUpdate:
		out.Event = &OneOf_SigstorePolicyUpdate{
			SigstorePolicyUpdate: e,
		}
	case *SigstorePolicyDelete:
		out.Event = &OneOf_SigstorePolicyDelete{
			SigstorePolicyDelete: e,
		}
	case *MCPSessionStart:
		out.Event = &OneOf_MCPSessionStart{
			MCPSessionStart: e,
		}
	case *MCPSessionEnd:
		out.Event = &OneOf_MCPSessionEnd{
			MCPSessionEnd: e,
		}
	case *MCPSessionRequest:
		out.Event = &OneOf_MCPSessionRequest{
			MCPSessionRequest: e,
		}
	case *MCPSessionNotification:
		out.Event = &OneOf_MCPSessionNotification{
			MCPSessionNotification: e,
		}
	default:
		slog.ErrorContext(context.Background(), "Attempted to convert dynamic event of unknown type into protobuf event.", "event_type", in.GetType())
		unknown := &Unknown{}
		unknown.Type = UnknownEvent
		unknown.Code = UnknownCode
		unknown.Time = in.GetTime()
		unknown.ClusterName = in.GetClusterName()
		unknown.UnknownType = in.GetType()
		unknown.UnknownCode = in.GetCode()
		data, err := json.Marshal(in)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		unknown.Data = string(data)
		out.Event = &OneOf_Unknown{
			Unknown: unknown,
		}
	}
	return &out, nil
}

// FromOneOf converts audit event from one of wrapper to interface
func FromOneOf(in OneOf) (AuditEvent, error) {
	e := in.GetEvent()
	if e == nil {
		return nil, trace.BadParameter("failed to parse event, session record is corrupted")
	}

	// We go from e (isOneOf_Event) -> reflect.Value (*OneOf_SomeStruct) -> reflect.Value(OneOf_SomeStruct).
	elem := reflect.ValueOf(in.GetEvent()).Elem()

	// OneOfs only have one inner field, verify and then read it.
	if elem.NumField() != 1 {
		// This should never happen for proto one-ofs.
		return nil, trace.BadParameter("unexpected number in value %v: %v != 1", elem.Kind(), elem.NumField())
	}

	auditEvent, ok := elem.Field(0).Interface().(AuditEvent)
	if !ok || reflect.ValueOf(auditEvent).IsNil() {
		return nil, trace.BadParameter("received unsupported event %T", in.Event)
	}
	return auditEvent, nil
}
