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
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils"
)

// eventsMap maps event names to event types for testing. Be sure to update
// this map if you add a new event type.
var eventsMap = map[string]apievents.AuditEvent{
	SessionPrintEvent:                              &apievents.SessionPrint{},
	SessionStartEvent:                              &apievents.SessionStart{},
	SessionEndEvent:                                &apievents.SessionEnd{},
	SessionUploadEvent:                             &apievents.SessionUpload{},
	SessionJoinEvent:                               &apievents.SessionJoin{},
	SessionLeaveEvent:                              &apievents.SessionLeave{},
	SessionDataEvent:                               &apievents.SessionData{},
	ClientDisconnectEvent:                          &apievents.ClientDisconnect{},
	UserLoginEvent:                                 &apievents.UserLogin{},
	UserDeleteEvent:                                &apievents.UserDelete{},
	UserCreateEvent:                                &apievents.UserCreate{},
	UserUpdatedEvent:                               &apievents.UserUpdate{},
	UserPasswordChangeEvent:                        &apievents.UserPasswordChange{},
	AccessRequestCreateEvent:                       &apievents.AccessRequestCreate{},
	AccessRequestReviewEvent:                       &apievents.AccessRequestCreate{},
	AccessRequestUpdateEvent:                       &apievents.AccessRequestCreate{},
	AccessRequestResourceSearch:                    &apievents.AccessRequestResourceSearch{},
	BillingCardCreateEvent:                         &apievents.BillingCardCreate{},
	BillingCardUpdateEvent:                         &apievents.BillingCardCreate{},
	BillingCardDeleteEvent:                         &apievents.BillingCardDelete{},
	BillingInformationUpdateEvent:                  &apievents.BillingInformationUpdate{},
	ResetPasswordTokenCreateEvent:                  &apievents.UserTokenCreate{},
	ExecEvent:                                      &apievents.Exec{},
	SubsystemEvent:                                 &apievents.Subsystem{},
	X11ForwardEvent:                                &apievents.X11Forward{},
	PortForwardEvent:                               &apievents.PortForward{},
	AuthAttemptEvent:                               &apievents.AuthAttempt{},
	SCPEvent:                                       &apievents.SCP{},
	ResizeEvent:                                    &apievents.Resize{},
	SessionCommandEvent:                            &apievents.SessionCommand{},
	SessionDiskEvent:                               &apievents.SessionDisk{},
	SessionNetworkEvent:                            &apievents.SessionNetwork{},
	RoleCreatedEvent:                               &apievents.RoleCreate{},
	RoleUpdatedEvent:                               &apievents.RoleUpdate{},
	RoleDeletedEvent:                               &apievents.RoleDelete{},
	TrustedClusterCreateEvent:                      &apievents.TrustedClusterCreate{},
	TrustedClusterDeleteEvent:                      &apievents.TrustedClusterDelete{},
	TrustedClusterTokenCreateEvent:                 &apievents.TrustedClusterTokenCreate{}, //nolint:staticcheck // SA1019. We want to test every event type, even if they're deprecated.
	ProvisionTokenCreateEvent:                      &apievents.ProvisionTokenCreate{},
	GithubConnectorCreatedEvent:                    &apievents.GithubConnectorCreate{},
	GithubConnectorUpdatedEvent:                    &apievents.GithubConnectorUpdate{},
	GithubConnectorDeletedEvent:                    &apievents.GithubConnectorDelete{},
	OIDCConnectorCreatedEvent:                      &apievents.OIDCConnectorCreate{},
	OIDCConnectorUpdatedEvent:                      &apievents.OIDCConnectorUpdate{},
	OIDCConnectorDeletedEvent:                      &apievents.OIDCConnectorDelete{},
	SAMLConnectorCreatedEvent:                      &apievents.SAMLConnectorCreate{},
	SAMLConnectorUpdatedEvent:                      &apievents.SAMLConnectorUpdate{},
	SAMLConnectorDeletedEvent:                      &apievents.SAMLConnectorDelete{},
	SessionRejectedEvent:                           &apievents.SessionReject{},
	AppSessionStartEvent:                           &apievents.AppSessionStart{},
	AppSessionEndEvent:                             &apievents.AppSessionEnd{},
	AppSessionChunkEvent:                           &apievents.AppSessionChunk{},
	AppSessionRequestEvent:                         &apievents.AppSessionRequest{},
	AppSessionDynamoDBRequestEvent:                 &apievents.AppSessionDynamoDBRequest{},
	AppCreateEvent:                                 &apievents.AppCreate{},
	AppUpdateEvent:                                 &apievents.AppUpdate{},
	AppDeleteEvent:                                 &apievents.AppDelete{},
	DatabaseCreateEvent:                            &apievents.DatabaseCreate{},
	DatabaseUpdateEvent:                            &apievents.DatabaseUpdate{},
	DatabaseDeleteEvent:                            &apievents.DatabaseDelete{},
	DatabaseSessionStartEvent:                      &apievents.DatabaseSessionStart{},
	DatabaseSessionEndEvent:                        &apievents.DatabaseSessionEnd{},
	DatabaseSessionQueryEvent:                      &apievents.DatabaseSessionQuery{},
	DatabaseSessionQueryFailedEvent:                &apievents.DatabaseSessionQuery{},
	DatabaseSessionCommandResultEvent:              &apievents.DatabaseSessionCommandResult{},
	DatabaseSessionMalformedPacketEvent:            &apievents.DatabaseSessionMalformedPacket{},
	DatabaseSessionPermissionsUpdateEvent:          &apievents.DatabasePermissionUpdate{},
	DatabaseSessionUserCreateEvent:                 &apievents.DatabaseUserCreate{},
	DatabaseSessionUserDeactivateEvent:             &apievents.DatabaseUserDeactivate{},
	DatabaseSessionPostgresParseEvent:              &apievents.PostgresParse{},
	DatabaseSessionPostgresBindEvent:               &apievents.PostgresBind{},
	DatabaseSessionPostgresExecuteEvent:            &apievents.PostgresExecute{},
	DatabaseSessionPostgresCloseEvent:              &apievents.PostgresClose{},
	DatabaseSessionPostgresFunctionEvent:           &apievents.PostgresFunctionCall{},
	DatabaseSessionMySQLStatementPrepareEvent:      &apievents.MySQLStatementPrepare{},
	DatabaseSessionMySQLStatementExecuteEvent:      &apievents.MySQLStatementExecute{},
	DatabaseSessionMySQLStatementSendLongDataEvent: &apievents.MySQLStatementSendLongData{},
	DatabaseSessionMySQLStatementCloseEvent:        &apievents.MySQLStatementClose{},
	DatabaseSessionMySQLStatementResetEvent:        &apievents.MySQLStatementReset{},
	DatabaseSessionMySQLStatementFetchEvent:        &apievents.MySQLStatementFetch{},
	DatabaseSessionMySQLStatementBulkExecuteEvent:  &apievents.MySQLStatementBulkExecute{},
	DatabaseSessionMySQLInitDBEvent:                &apievents.MySQLInitDB{},
	DatabaseSessionMySQLCreateDBEvent:              &apievents.MySQLCreateDB{},
	DatabaseSessionMySQLDropDBEvent:                &apievents.MySQLDropDB{},
	DatabaseSessionMySQLShutDownEvent:              &apievents.MySQLShutDown{},
	DatabaseSessionMySQLProcessKillEvent:           &apievents.MySQLProcessKill{},
	DatabaseSessionMySQLDebugEvent:                 &apievents.MySQLDebug{},
	DatabaseSessionMySQLRefreshEvent:               &apievents.MySQLRefresh{},
	DatabaseSessionSQLServerRPCRequestEvent:        &apievents.SQLServerRPCRequest{},
	DatabaseSessionElasticsearchRequestEvent:       &apievents.ElasticsearchRequest{},
	DatabaseSessionOpenSearchRequestEvent:          &apievents.OpenSearchRequest{},
	DatabaseSessionDynamoDBRequestEvent:            &apievents.DynamoDBRequest{},
	KubeRequestEvent:                               &apievents.KubeRequest{},
	MFADeviceAddEvent:                              &apievents.MFADeviceAdd{},
	MFADeviceDeleteEvent:                           &apievents.MFADeviceDelete{},
	DeviceEvent:                                    &apievents.DeviceEvent{},
	DeviceCreateEvent:                              &apievents.DeviceEvent2{},
	DeviceDeleteEvent:                              &apievents.DeviceEvent2{},
	DeviceUpdateEvent:                              &apievents.DeviceEvent2{},
	DeviceEnrollEvent:                              &apievents.DeviceEvent2{},
	DeviceAuthenticateEvent:                        &apievents.DeviceEvent2{},
	DeviceEnrollTokenCreateEvent:                   &apievents.DeviceEvent2{},
	DeviceWebTokenCreateEvent:                      &apievents.DeviceEvent2{},
	DeviceAuthenticateConfirmEvent:                 &apievents.DeviceEvent2{},
	LockCreatedEvent:                               &apievents.LockCreate{},
	LockDeletedEvent:                               &apievents.LockDelete{},
	RecoveryCodeGeneratedEvent:                     &apievents.RecoveryCodeGenerate{},
	RecoveryCodeUsedEvent:                          &apievents.RecoveryCodeUsed{},
	RecoveryTokenCreateEvent:                       &apievents.UserTokenCreate{},
	PrivilegeTokenCreateEvent:                      &apievents.UserTokenCreate{},
	WindowsDesktopSessionStartEvent:                &apievents.WindowsDesktopSessionStart{},
	WindowsDesktopSessionEndEvent:                  &apievents.WindowsDesktopSessionEnd{},
	DesktopRecordingEvent:                          &apievents.DesktopRecording{},
	DesktopClipboardSendEvent:                      &apievents.DesktopClipboardSend{},
	DesktopClipboardReceiveEvent:                   &apievents.DesktopClipboardReceive{},
	SessionConnectEvent:                            &apievents.SessionConnect{},
	AccessRequestDeleteEvent:                       &apievents.AccessRequestDelete{},
	CertificateCreateEvent:                         &apievents.CertificateCreate{},
	RenewableCertificateGenerationMismatchEvent:    &apievents.RenewableCertificateGenerationMismatch{},
	SFTPEvent:                                   &apievents.SFTP{},
	UpgradeWindowStartUpdateEvent:               &apievents.UpgradeWindowStartUpdate{},
	SessionRecordingAccessEvent:                 &apievents.SessionRecordingAccess{},
	SSMRunEvent:                                 &apievents.SSMRun{},
	KubernetesClusterCreateEvent:                &apievents.KubernetesClusterCreate{},
	KubernetesClusterUpdateEvent:                &apievents.KubernetesClusterUpdate{},
	KubernetesClusterDeleteEvent:                &apievents.KubernetesClusterDelete{},
	DesktopSharedDirectoryStartEvent:            &apievents.DesktopSharedDirectoryStart{},
	DesktopSharedDirectoryReadEvent:             &apievents.DesktopSharedDirectoryRead{},
	DesktopSharedDirectoryWriteEvent:            &apievents.DesktopSharedDirectoryWrite{},
	BotJoinEvent:                                &apievents.BotJoin{},
	InstanceJoinEvent:                           &apievents.InstanceJoin{},
	BotCreateEvent:                              &apievents.BotCreate{},
	BotUpdateEvent:                              &apievents.BotUpdate{},
	BotDeleteEvent:                              &apievents.BotDelete{},
	LoginRuleCreateEvent:                        &apievents.LoginRuleCreate{},
	LoginRuleDeleteEvent:                        &apievents.LoginRuleDelete{},
	SAMLIdPAuthAttemptEvent:                     &apievents.SAMLIdPAuthAttempt{},
	SAMLIdPServiceProviderCreateEvent:           &apievents.SAMLIdPServiceProviderCreate{},
	SAMLIdPServiceProviderUpdateEvent:           &apievents.SAMLIdPServiceProviderUpdate{},
	SAMLIdPServiceProviderDeleteEvent:           &apievents.SAMLIdPServiceProviderDelete{},
	SAMLIdPServiceProviderDeleteAllEvent:        &apievents.SAMLIdPServiceProviderDeleteAll{},
	OktaGroupsUpdateEvent:                       &apievents.OktaResourcesUpdate{},
	OktaApplicationsUpdateEvent:                 &apievents.OktaResourcesUpdate{},
	OktaSyncFailureEvent:                        &apievents.OktaSyncFailure{},
	OktaAssignmentProcessEvent:                  &apievents.OktaAssignmentResult{},
	OktaAssignmentCleanupEvent:                  &apievents.OktaAssignmentResult{},
	OktaUserSyncEvent:                           &apievents.OktaUserSync{},
	OktaAccessListSyncEvent:                     &apievents.OktaAccessListSync{},
	AccessGraphAccessPathChangedEvent:           &apievents.AccessPathChanged{},
	AccessListCreateEvent:                       &apievents.AccessListCreate{},
	AccessListUpdateEvent:                       &apievents.AccessListUpdate{},
	AccessListDeleteEvent:                       &apievents.AccessListDelete{},
	AccessListReviewEvent:                       &apievents.AccessListReview{},
	AccessListMemberCreateEvent:                 &apievents.AccessListMemberCreate{},
	AccessListMemberUpdateEvent:                 &apievents.AccessListMemberUpdate{},
	AccessListMemberDeleteEvent:                 &apievents.AccessListMemberDelete{},
	AccessListMemberDeleteAllForAccessListEvent: &apievents.AccessListMemberDeleteAllForAccessList{},
	UserLoginAccessListInvalidEvent:             &apievents.UserLoginAccessListInvalid{},
	SecReportsAuditQueryRunEvent:                &apievents.AuditQueryRun{},
	SecReportsReportRunEvent:                    &apievents.SecurityReportRun{},
	ExternalAuditStorageEnableEvent:             &apievents.ExternalAuditStorageEnable{},
	ExternalAuditStorageDisableEvent:            &apievents.ExternalAuditStorageDisable{},
	CreateMFAAuthChallengeEvent:                 &apievents.CreateMFAAuthChallenge{},
	ValidateMFAAuthResponseEvent:                &apievents.ValidateMFAAuthResponse{},
	SPIFFESVIDIssuedEvent:                       &apievents.SPIFFESVIDIssued{},
	AuthPreferenceUpdateEvent:                   &apievents.AuthPreferenceUpdate{},
	ClusterNetworkingConfigUpdateEvent:          &apievents.ClusterNetworkingConfigUpdate{},
	SessionRecordingConfigUpdateEvent:           &apievents.SessionRecordingConfigUpdate{},
	AccessGraphSettingsUpdateEvent:              &apievents.AccessGraphSettingsUpdate{},
	DatabaseSessionSpannerRPCEvent:              &apievents.SpannerRPC{},
	UnknownEvent:                                &apievents.Unknown{},
	DatabaseSessionCassandraBatchEvent:          &apievents.CassandraBatch{},
	DatabaseSessionCassandraRegisterEvent:       &apievents.CassandraRegister{},
	DatabaseSessionCassandraPrepareEvent:        &apievents.CassandraPrepare{},
	DatabaseSessionCassandraExecuteEvent:        &apievents.CassandraExecute{},
	DiscoveryConfigCreateEvent:                  &apievents.DiscoveryConfigCreate{},
	DiscoveryConfigUpdateEvent:                  &apievents.DiscoveryConfigUpdate{},
	DiscoveryConfigDeleteEvent:                  &apievents.DiscoveryConfigDelete{},
	DiscoveryConfigDeleteAllEvent:               &apievents.DiscoveryConfigDeleteAll{},
	IntegrationCreateEvent:                      &apievents.IntegrationCreate{},
	IntegrationUpdateEvent:                      &apievents.IntegrationUpdate{},
	IntegrationDeleteEvent:                      &apievents.IntegrationDelete{},
	SPIFFEFederationCreateEvent:                 &apievents.SPIFFEFederationCreate{},
	SPIFFEFederationDeleteEvent:                 &apievents.SPIFFEFederationDelete{},
	PluginCreateEvent:                           &apievents.PluginCreate{},
	PluginUpdateEvent:                           &apievents.PluginUpdate{},
	PluginDeleteEvent:                           &apievents.PluginDelete{},
	StaticHostUserCreateEvent:                   &apievents.StaticHostUserCreate{},
	StaticHostUserUpdateEvent:                   &apievents.StaticHostUserUpdate{},
	StaticHostUserDeleteEvent:                   &apievents.StaticHostUserDelete{},
	CrownJewelCreateEvent:                       &apievents.CrownJewelCreate{},
	CrownJewelUpdateEvent:                       &apievents.CrownJewelUpdate{},
	CrownJewelDeleteEvent:                       &apievents.CrownJewelDelete{},
	UserTaskCreateEvent:                         &apievents.UserTaskCreate{},
	UserTaskUpdateEvent:                         &apievents.UserTaskUpdate{},
	UserTaskDeleteEvent:                         &apievents.UserTaskDelete{},
	SFTPSummaryEvent:                            &apievents.SFTPSummary{},
	AutoUpdateConfigCreateEvent:                 &apievents.AutoUpdateConfigCreate{},
	AutoUpdateConfigUpdateEvent:                 &apievents.AutoUpdateConfigUpdate{},
	AutoUpdateConfigDeleteEvent:                 &apievents.AutoUpdateConfigDelete{},
	AutoUpdateVersionCreateEvent:                &apievents.AutoUpdateVersionCreate{},
	AutoUpdateVersionUpdateEvent:                &apievents.AutoUpdateVersionUpdate{},
	AutoUpdateVersionDeleteEvent:                &apievents.AutoUpdateVersionDelete{},
	ContactCreateEvent:                          &apievents.ContactCreate{},
	ContactDeleteEvent:                          &apievents.ContactDelete{},
	WorkloadIdentityCreateEvent:                 &apievents.WorkloadIdentityCreate{},
	WorkloadIdentityUpdateEvent:                 &apievents.WorkloadIdentityUpdate{},
	WorkloadIdentityDeleteEvent:                 &apievents.WorkloadIdentityDelete{},
	AccessRequestExpireEvent:                    &apievents.AccessRequestExpire{},
	StableUNIXUserCreateEvent:                   &apievents.StableUNIXUserCreate{},
}

// TestJSON tests JSON marshal events
func TestJSON(t *testing.T) {
	type testCase struct {
		name  string
		json  string
		event interface{}
	}
	testCases := []testCase{
		{
			name: "session start event",
			json: `{"ei":0,"event":"session.start","uid":"36cee9e9-9a80-4c32-9163-3d9241cdac7a","code":"T2000I","time":"2020-03-30T15:58:54.561Z","namespace":"default","sid":"5b3555dc-729f-11ea-b66a-507b9dd95841","cluster_name":"testcluster","login":"bob","user":"bob@example.com","server_id":"a7c54b0c-469c-431e-af4d-418cd3ae9694","server_hostname":"planet","server_labels":{"group":"gravitational/devc","kernel":"5.3.0-42-generic","date":"Mon Mar 30 08:58:54 PDT 2020"},"addr.local":"127.0.0.1:3022","addr.remote":"[::1]:37718","size":"80:25"}`,
			event: apievents.SessionStart{
				Metadata: apievents.Metadata{
					Index:       0,
					Type:        SessionStartEvent,
					ID:          "36cee9e9-9a80-4c32-9163-3d9241cdac7a",
					Code:        SessionStartCode,
					Time:        time.Date(2020, 03, 30, 15, 58, 54, 561*int(time.Millisecond), time.UTC),
					ClusterName: "testcluster",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID: "a7c54b0c-469c-431e-af4d-418cd3ae9694",
					ServerLabels: map[string]string{
						"kernel": "5.3.0-42-generic",
						"date":   "Mon Mar 30 08:58:54 PDT 2020",
						"group":  "gravitational/devc",
					},
					ServerHostname:  "planet",
					ServerNamespace: "default",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "5b3555dc-729f-11ea-b66a-507b9dd95841",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "bob@example.com",
					Login: "bob",
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					LocalAddr:  "127.0.0.1:3022",
					RemoteAddr: "[::1]:37718",
				},
				TerminalSize: "80:25",
			},
		},
		{
			name: "resize event",
			json: `{"time":"2020-03-30T15:58:54.564Z","uid":"c34e512f-e6cb-44f1-ab94-4cea09002d29","event":"resize","login":"bob","sid":"5b3555dc-729f-11ea-b66a-507b9dd95841","cluster_name":"testcluster","size":"194:59","ei":1,"code":"T2002I","namespace":"default","server_id":"a7c54b0c-469c-431e-af4d-418cd3ae9694","user":"bob@example.com"}`,
			event: apievents.Resize{
				Metadata: apievents.Metadata{
					Index:       1,
					Type:        ResizeEvent,
					ID:          "c34e512f-e6cb-44f1-ab94-4cea09002d29",
					Code:        TerminalResizeCode,
					Time:        time.Date(2020, 03, 30, 15, 58, 54, 564*int(time.Millisecond), time.UTC),
					ClusterName: "testcluster",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        "a7c54b0c-469c-431e-af4d-418cd3ae9694",
					ServerNamespace: "default",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "5b3555dc-729f-11ea-b66a-507b9dd95841",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "bob@example.com",
					Login: "bob",
				},
				TerminalSize: "194:59",
			},
		},
		{
			name: "session end event",
			json: `{"code":"T2004I","ei":20,"enhanced_recording":true,"event":"session.end","interactive":true,"namespace":"default","participants":["alice@example.com"],"server_id":"a7c54b0c-469c-431e-af4d-418cd3ae9694","sid":"5b3555dc-729f-11ea-b66a-507b9dd95841","cluster_name":"test-cluster","time":"2020-03-30T15:58:58.999Z","uid":"da455e0f-c27d-459f-a218-4e83b3db9426","user":"alice@example.com", "session_start":"2020-03-30T15:58:54.561Z", "session_stop": "2020-03-30T15:58:58.999Z"}`,
			event: apievents.SessionEnd{
				Metadata: apievents.Metadata{
					Index:       20,
					Type:        SessionEndEvent,
					ID:          "da455e0f-c27d-459f-a218-4e83b3db9426",
					Code:        SessionEndCode,
					Time:        time.Date(2020, 03, 30, 15, 58, 58, 999*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        "a7c54b0c-469c-431e-af4d-418cd3ae9694",
					ServerNamespace: "default",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "5b3555dc-729f-11ea-b66a-507b9dd95841",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				EnhancedRecording: true,
				Interactive:       true,
				Participants:      []string{"alice@example.com"},
				StartTime:         time.Date(2020, 03, 30, 15, 58, 54, 561*int(time.Millisecond), time.UTC),
				EndTime:           time.Date(2020, 03, 30, 15, 58, 58, 999*int(time.Millisecond), time.UTC),
			},
		},
		{
			name: "session print event",
			json: `{"time":"2020-03-30T15:58:56.959Z","event":"print","bytes":1551,"ms":2284,"offset":1957,"ei":11,"ci":9,"cluster_name":"test"}`,
			event: apievents.SessionPrint{
				Metadata: apievents.Metadata{
					Index:       11,
					Type:        SessionPrintEvent,
					Time:        time.Date(2020, 03, 30, 15, 58, 56, 959*int(time.Millisecond), time.UTC),
					ClusterName: "test",
				},
				ChunkIndex:        9,
				Bytes:             1551,
				DelayMilliseconds: 2284,
				Offset:            1957,
			},
		},
		{
			name: "session command event",
			json: `{"argv":["/usr/bin/lesspipe"],"login":"alice","path":"/usr/bin/dirname","return_code":0,"time":"2020-03-30T15:58:54.65Z","user":"alice@example.com","code":"T4000I","event":"session.command","pid":31638,"server_id":"a7c54b0c-469c-431e-af4d-418cd3ae9694","server_hostname":"ip-172-31-11-148","uid":"4f725f11-e87a-452f-96ec-ef93e9e6a260","cgroup_id":4294971450,"ppid":31637,"program":"dirname","namespace":"default","sid":"5b3555dc-729f-11ea-b66a-507b9dd95841","cluster_name":"test","ei":4}`,
			event: apievents.SessionCommand{
				Metadata: apievents.Metadata{
					Index:       4,
					ID:          "4f725f11-e87a-452f-96ec-ef93e9e6a260",
					Type:        SessionCommandEvent,
					Time:        time.Date(2020, 03, 30, 15, 58, 54, 650*int(time.Millisecond), time.UTC),
					Code:        SessionCommandCode,
					ClusterName: "test",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        "a7c54b0c-469c-431e-af4d-418cd3ae9694",
					ServerHostname:  "ip-172-31-11-148",
					ServerNamespace: "default",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "5b3555dc-729f-11ea-b66a-507b9dd95841",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "alice@example.com",
					Login: "alice",
				},
				BPFMetadata: apievents.BPFMetadata{
					CgroupID: 4294971450,
					Program:  "dirname",
					PID:      31638,
				},
				PPID:       31637,
				ReturnCode: 0,
				Path:       "/usr/bin/dirname",
				Argv:       []string{"/usr/bin/lesspipe"},
			},
		},
		{
			name: "session network event",
			json: `{"dst_port":443,"cgroup_id":4294976805,"dst_addr":"2607:f8b0:400a:801::200e","program":"curl","sid":"e9a4bd34-78ff-11ea-b062-507b9dd95841","src_addr":"2601:602:8700:4470:a3:813c:1d8c:30b9","login":"alice","pid":17604,"uid":"729498e0-c28b-438f-baa7-663a74418449","user":"alice@example.com","event":"session.network","namespace":"default","time":"2020-04-07T18:45:16.602Z","version":6,"ei":0,"code":"T4002I","server_id":"00b54ef5-ae1e-425f-8565-c71b01d8f7b8","server_hostname":"ip-172-31-11-148","cluster_name":"example","operation":0,"action":1}`,
			event: apievents.SessionNetwork{
				Metadata: apievents.Metadata{
					Index:       0,
					ID:          "729498e0-c28b-438f-baa7-663a74418449",
					Type:        SessionNetworkEvent,
					Time:        time.Date(2020, 04, 07, 18, 45, 16, 602*int(time.Millisecond), time.UTC),
					Code:        SessionNetworkCode,
					ClusterName: "example",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        "00b54ef5-ae1e-425f-8565-c71b01d8f7b8",
					ServerHostname:  "ip-172-31-11-148",
					ServerNamespace: "default",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "e9a4bd34-78ff-11ea-b062-507b9dd95841",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "alice@example.com",
					Login: "alice",
				},
				BPFMetadata: apievents.BPFMetadata{
					CgroupID: 4294976805,
					Program:  "curl",
					PID:      17604,
				},
				DstPort:    443,
				DstAddr:    "2607:f8b0:400a:801::200e",
				SrcAddr:    "2601:602:8700:4470:a3:813c:1d8c:30b9",
				TCPVersion: 6,
				Operation:  apievents.SessionNetwork_CONNECT,
				Action:     apievents.EventAction_DENIED,
			},
		},
		{
			name: "session disk event",
			json: `{"time":"2020-04-07T19:56:38.545Z","login":"bob","pid":31521,"sid":"ddddce15-7909-11ea-b062-507b9dd95841","user":"bob@example.com","ei":175,"code":"T4001I","flags":142606336,"namespace":"default","uid":"ab8467af-6d85-46ce-bb5c-bdfba8acad3f","cgroup_id":4294976835,"program":"clear_console","server_id":"00b54ef5-ae1e-425f-8565-c71b01d8f7b8","server_hostname":"ip-172-31-11-148","event":"session.disk","path":"/etc/ld.so.cache","return_code":3,"cluster_name":"example2"}`,
			event: apievents.SessionDisk{
				Metadata: apievents.Metadata{
					Index:       175,
					ID:          "ab8467af-6d85-46ce-bb5c-bdfba8acad3f",
					Type:        SessionDiskEvent,
					Time:        time.Date(2020, 04, 07, 19, 56, 38, 545*int(time.Millisecond), time.UTC),
					Code:        SessionDiskCode,
					ClusterName: "example2",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        "00b54ef5-ae1e-425f-8565-c71b01d8f7b8",
					ServerHostname:  "ip-172-31-11-148",
					ServerNamespace: "default",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "ddddce15-7909-11ea-b062-507b9dd95841",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "bob@example.com",
					Login: "bob",
				},
				BPFMetadata: apievents.BPFMetadata{
					CgroupID: 4294976835,
					Program:  "clear_console",
					PID:      31521,
				},
				Flags:      142606336,
				Path:       "/etc/ld.so.cache",
				ReturnCode: 3,
			},
		},
		{
			name: "successful user.login event",
			json: `{"ei": 0, "attributes":{"followers_url": "https://api.github.com/users/bob/followers", "err": null, "public_repos": 20, "site_admin": false, "app_metadata":{"roles":["example/admins","example/devc"]}, "emails":[{"email":"bob@example.com","primary":true,"verified":true,"visibility":"public"},{"email":"bob@alternative.com","primary":false,"verified":true,"visibility":null}]},"code":"T1001I","event":"user.login","method":"oidc","success":true,"time":"2020-04-07T18:45:07Z","uid":"019432f1-3021-4860-af41-d9bd1668c3ea","user":"bob@example.com","cluster_name":"testcluster"}`,
			event: apievents.UserLogin{
				Metadata: apievents.Metadata{
					ID:          "019432f1-3021-4860-af41-d9bd1668c3ea",
					Type:        UserLoginEvent,
					Time:        time.Date(2020, 04, 07, 18, 45, 07, 0*int(time.Millisecond), time.UTC),
					Code:        UserSSOLoginCode,
					ClusterName: "testcluster",
				},
				Status: apievents.Status{
					Success: true,
				},
				UserMetadata: apievents.UserMetadata{
					User: "bob@example.com",
				},
				IdentityAttributes: apievents.MustEncodeMap(map[string]interface{}{
					"followers_url": "https://api.github.com/users/bob/followers",
					"err":           nil,
					"public_repos":  20,
					"site_admin":    false,
					"app_metadata":  map[string]interface{}{"roles": []interface{}{"example/admins", "example/devc"}},
					"emails": []interface{}{
						map[string]interface{}{
							"email":      "bob@example.com",
							"primary":    true,
							"verified":   true,
							"visibility": "public",
						},
						map[string]interface{}{
							"email":      "bob@alternative.com",
							"primary":    false,
							"verified":   true,
							"visibility": nil,
						},
					},
				}),
				Method: LoginMethodOIDC,
			},
		},
		{
			name: "session data event",
			json: `{"addr.local":"127.0.0.1:3022","addr.remote":"[::1]:44382","code":"T2006I","ei":2147483646,"event":"session.data","login":"alice","rx":9526,"server_id":"00b54ef5-ae1e-425f-8565-c71b01d8f7b8","sid":"ddddce15-7909-11ea-b062-507b9dd95841","time":"2020-04-07T19:56:39Z","tx":10279,"uid":"cb404873-cd7c-4036-854b-42e0f5fd5f2c","user":"alice@example.com","cluster_name":"test"}`,
			event: apievents.SessionData{
				Metadata: apievents.Metadata{
					Index:       2147483646,
					ID:          "cb404873-cd7c-4036-854b-42e0f5fd5f2c",
					Type:        SessionDataEvent,
					Time:        time.Date(2020, 04, 07, 19, 56, 39, 0*int(time.Millisecond), time.UTC),
					Code:        SessionDataCode,
					ClusterName: "test",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID: "00b54ef5-ae1e-425f-8565-c71b01d8f7b8",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "ddddce15-7909-11ea-b062-507b9dd95841",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "alice@example.com",
					Login: "alice",
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					LocalAddr:  "127.0.0.1:3022",
					RemoteAddr: "[::1]:44382",
				},
				BytesReceived:    9526,
				BytesTransmitted: 10279,
			},
		},
		{
			name: "session leave event",
			json: `{"code":"T2003I","ei":39,"event":"session.leave","namespace":"default","server_id":"00b54ef5-ae1e-425f-8565-c71b01d8f7b8","sid":"ddddce15-7909-11ea-b062-507b9dd95841","time":"2020-04-07T19:56:38.556Z","uid":"d7c7489f-6559-42ad-9963-8543e518a058","user":"alice@example.com","cluster_name":"example"}`,
			event: apievents.SessionLeave{
				Metadata: apievents.Metadata{
					Index:       39,
					ID:          "d7c7489f-6559-42ad-9963-8543e518a058",
					Type:        SessionLeaveEvent,
					Time:        time.Date(2020, 04, 07, 19, 56, 38, 556*int(time.Millisecond), time.UTC),
					Code:        SessionLeaveCode,
					ClusterName: "example",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        "00b54ef5-ae1e-425f-8565-c71b01d8f7b8",
					ServerNamespace: "default",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "ddddce15-7909-11ea-b062-507b9dd95841",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
			},
		},
		{
			name: "user update",
			json: `{"ei": 0, "code":"T1003I","connector":"auth0","event":"user.update","expires":"2020-04-08T02:45:06.524816756Z","roles":["clusteradmin"],"time":"2020-04-07T18:45:07Z","uid":"e7c8e36e-adb4-4c98-b818-226d73add7fc","user":"alice@example.com","cluster_name":"test-cluster"}`,
			event: apievents.UserCreate{
				Metadata: apievents.Metadata{
					ID:          "e7c8e36e-adb4-4c98-b818-226d73add7fc",
					Type:        UserUpdatedEvent,
					Time:        time.Date(2020, 4, 7, 18, 45, 7, 0*int(time.Millisecond), time.UTC),
					Code:        UserUpdateCode,
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				ResourceMetadata: apievents.ResourceMetadata{
					Expires: time.Date(2020, 4, 8, 2, 45, 6, 524816756*int(time.Nanosecond), time.UTC),
				},
				Connector: "auth0",
				Roles:     []string{"clusteradmin"},
			},
		},
		{
			name: "success port forward",
			json: `{"ei": 0, "addr":"localhost:3025","addr.local":"127.0.0.1:3022","addr.remote":"127.0.0.1:45976","code":"T3003I","event":"port","login":"alice","success":true,"time":"2020-04-15T18:06:56.397Z","uid":"7efc5025-a712-47de-8086-7d935c110188","user":"alice@example.com","cluster_name":"test"}`,
			event: apievents.PortForward{
				Metadata: apievents.Metadata{
					ID:          "7efc5025-a712-47de-8086-7d935c110188",
					Type:        PortForwardEvent,
					Time:        time.Date(2020, 4, 15, 18, 06, 56, 397*int(time.Millisecond), time.UTC),
					Code:        PortForwardCode,
					ClusterName: "test",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "alice@example.com",
					Login: "alice",
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					LocalAddr:  "127.0.0.1:3022",
					RemoteAddr: "127.0.0.1:45976",
				},
				Status: apievents.Status{
					Success: true,
				},
				Addr: "localhost:3025",
			},
		},
		{
			name: "rejected port forward",
			json: `{"ei": 0, "addr":"localhost:3025","addr.local":"127.0.0.1:3022","addr.remote":"127.0.0.1:46452","code":"T3003E","error":"port forwarding not allowed by role set: roles clusteradmin,default-implicit-role","event":"port","login":"bob","success":false,"time":"2020-04-15T18:20:21Z","uid":"097724d1-5ee3-4c8d-a911-ea6021e5b3fb","user":"bob@example.com","cluster_name":"test"}`,
			event: apievents.PortForward{
				Metadata: apievents.Metadata{
					ID:          "097724d1-5ee3-4c8d-a911-ea6021e5b3fb",
					Type:        PortForwardEvent,
					Time:        time.Date(2020, 4, 15, 18, 20, 21, 0*int(time.Millisecond), time.UTC),
					Code:        PortForwardFailureCode,
					ClusterName: "test",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "bob@example.com",
					Login: "bob",
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					LocalAddr:  "127.0.0.1:3022",
					RemoteAddr: "127.0.0.1:46452",
				},
				Status: apievents.Status{
					Error:   "port forwarding not allowed by role set: roles clusteradmin,default-implicit-role",
					Success: false,
				},
				Addr: "localhost:3025",
			},
		},
		{
			name: "rejected subsystem",
			json: `{"ei":0,"cluster_name":"test","addr.local":"127.0.0.1:57518","addr.remote":"127.0.0.1:3022","code":"T3001E","event":"subsystem","exitError":"some error","forwarded_by":"abc","login":"alice","name":"proxy","server_id":"123","time":"2020-04-15T20:28:18Z","uid":"3129a5ae-ee1e-4b39-8d7c-a0a3f218e7dc","user":"alice@example.com"}`,
			event: apievents.Subsystem{
				Metadata: apievents.Metadata{
					ID:          "3129a5ae-ee1e-4b39-8d7c-a0a3f218e7dc",
					Type:        SubsystemEvent,
					Time:        time.Date(2020, 4, 15, 20, 28, 18, 0*int(time.Millisecond), time.UTC),
					Code:        SubsystemFailureCode,
					ClusterName: "test",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "alice@example.com",
					Login: "alice",
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					LocalAddr:  "127.0.0.1:57518",
					RemoteAddr: "127.0.0.1:3022",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:    "123",
					ForwardedBy: "abc",
				},
				Name:  "proxy",
				Error: "some error",
			},
		},
		{
			name: "failed auth attempt",
			json: `{"ei": 0, "code":"T3007W","error":"ssh: principal \"bob\" not in the set of valid principals for given certificate: [\"root\" \"alice\"]","event":"auth","success":false,"time":"2020-04-22T20:53:50Z","uid":"ebac95ca-8673-44af-b2cf-65f517acf35a","user":"alice@example.com","cluster_name":"testcluster"}`,
			event: apievents.AuthAttempt{
				Metadata: apievents.Metadata{
					ID:          "ebac95ca-8673-44af-b2cf-65f517acf35a",
					Type:        AuthAttemptEvent,
					Time:        time.Date(2020, 4, 22, 20, 53, 50, 0*int(time.Millisecond), time.UTC),
					Code:        AuthAttemptFailureCode,
					ClusterName: "testcluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				Status: apievents.Status{
					Success: false,
					Error:   "ssh: principal \"bob\" not in the set of valid principals for given certificate: [\"root\" \"alice\"]",
				},
			},
		},
		{
			name: "session join",
			json: `{"uid":"cd03665f-3ce1-4c22-809d-4be9512c36e2","addr.local":"127.0.0.1:3022","addr.remote":"[::1]:34902","code":"T2001I","event":"session.join","login":"root","time":"2020-04-23T18:22:35.35Z","namespace":"default","server_id":"00b54ef5-ae1e-425f-8565-c71b01d8f7b8","sid":"b0252ad2-2fa5-4bb2-a7de-2cacd1169c96","user":"bob@example.com","ei":4,"cluster_name":"test-cluster"}`,
			event: apievents.SessionJoin{
				Metadata: apievents.Metadata{
					Index:       4,
					Type:        SessionJoinEvent,
					ID:          "cd03665f-3ce1-4c22-809d-4be9512c36e2",
					Code:        SessionJoinCode,
					Time:        time.Date(2020, 04, 23, 18, 22, 35, 350*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        "00b54ef5-ae1e-425f-8565-c71b01d8f7b8",
					ServerNamespace: "default",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "b0252ad2-2fa5-4bb2-a7de-2cacd1169c96",
				},
				UserMetadata: apievents.UserMetadata{
					User:  "bob@example.com",
					Login: "root",
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					LocalAddr:  "127.0.0.1:3022",
					RemoteAddr: "[::1]:34902",
				},
			},
		},
		{
			name: "desktop session start",
			json: `{"uid":"cd06365f-3cef-4b21-809a-4af9502c11a1","user":"foo","impersonator":"bar","login":"Administrator","success":true,"proto":"tdp","sid":"test-session","addr.local":"192.168.1.100:39887","addr.remote":"[::1]:34902","with_mfa":"mfa-device","code":"TDP00I","event":"windows.desktop.session.start","time":"2020-04-23T18:22:35.35Z","ei":4,"cluster_name":"test-cluster","windows_user":"Administrator","windows_domain":"test.example.com","desktop_name":"test-desktop","desktop_addr":"[::1]:34902","windows_desktop_service":"00baaef5-ff1e-4222-85a5-c7cb0cd8e7b8","allow_user_creation":false,"nla":true,"desktop_labels":{"env":"production"}}`,
			event: apievents.WindowsDesktopSessionStart{
				Metadata: apievents.Metadata{
					Index:       4,
					ID:          "cd06365f-3cef-4b21-809a-4af9502c11a1",
					Type:        WindowsDesktopSessionStartEvent,
					Code:        DesktopSessionStartCode,
					Time:        time.Date(2020, 04, 23, 18, 22, 35, 350*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User:         "foo",
					Impersonator: "bar",
					Login:        "Administrator",
				},
				SessionMetadata: apievents.SessionMetadata{
					WithMFA:   "mfa-device",
					SessionID: "test-session",
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					LocalAddr:  "192.168.1.100:39887",
					RemoteAddr: "[::1]:34902",
					Protocol:   EventProtocolTDP,
				},
				Status: apievents.Status{
					Success: true,
				},
				WindowsDesktopService: "00baaef5-ff1e-4222-85a5-c7cb0cd8e7b8",
				DesktopName:           "test-desktop",
				DesktopAddr:           "[::1]:34902",
				Domain:                "test.example.com",
				WindowsUser:           "Administrator",
				DesktopLabels:         map[string]string{"env": "production"},
				NLA:                   true,
			},
		},
		{
			name: "desktop session end",
			json: `{"uid":"cd06365f-3cef-4b21-809a-4af9502c11a1","user":"foo","impersonator":"bar","login":"Administrator","participants":["foo"],"recorded":false,"sid":"test-session","with_mfa":"mfa-device","code":"TDP01I","event":"windows.desktop.session.end","time":"2020-04-23T18:22:35.35Z","session_start":"2020-04-23T18:22:35.35Z","session_stop":"2020-04-23T18:26:35.35Z","ei":4,"cluster_name":"test-cluster","windows_user":"Administrator","windows_domain":"test.example.com","desktop_name":"desktop1","desktop_addr":"[::1]:34902","windows_desktop_service":"00baaef5-ff1e-4222-85a5-c7cb0cd8e7b8","desktop_labels":{"env":"production"}}`,
			event: apievents.WindowsDesktopSessionEnd{
				Metadata: apievents.Metadata{
					Index:       4,
					ID:          "cd06365f-3cef-4b21-809a-4af9502c11a1",
					Type:        WindowsDesktopSessionEndEvent,
					Code:        DesktopSessionEndCode,
					Time:        time.Date(2020, 04, 23, 18, 22, 35, 350*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					Login:        "Administrator",
					User:         "foo",
					Impersonator: "bar",
				},
				SessionMetadata: apievents.SessionMetadata{
					WithMFA:   "mfa-device",
					SessionID: "test-session",
				},
				WindowsDesktopService: "00baaef5-ff1e-4222-85a5-c7cb0cd8e7b8",
				DesktopName:           "desktop1",
				DesktopAddr:           "[::1]:34902",
				Domain:                "test.example.com",
				WindowsUser:           "Administrator",
				DesktopLabels:         map[string]string{"env": "production"},
				Participants:          []string{"foo"},
				StartTime:             time.Date(2020, 04, 23, 18, 22, 35, 350*int(time.Millisecond), time.UTC),
				EndTime:               time.Date(2020, 04, 23, 18, 26, 35, 350*int(time.Millisecond), time.UTC),
			},
		},
		{
			name: "MySQL statement prepare",
			json: `{"cluster_name":"test-cluster","code":"TMY00I","db_name":"test","db_protocol":"mysql","db_service":"test-mysql","db_uri":"localhost:3306","db_user":"alice","ei":22,"event":"db.session.mysql.statements.prepare","query":"select 1","sid":"test-session","time":"2022-02-22T22:22:22.222Z","uid":"test-id","user":"alice@example.com"}`,
			event: apievents.MySQLStatementPrepare{
				Metadata: apievents.Metadata{
					Index:       22,
					ID:          "test-id",
					Type:        DatabaseSessionMySQLStatementPrepareEvent,
					Code:        MySQLStatementPrepareCode,
					Time:        time.Date(2022, 02, 22, 22, 22, 22, 222*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "test-session",
				},
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService:  "test-mysql",
					DatabaseProtocol: "mysql",
					DatabaseURI:      "localhost:3306",
					DatabaseName:     "test",
					DatabaseUser:     "alice",
				},
				Query: "select 1",
			},
		},
		{
			name: "MySQL statement execute",
			json: `{"cluster_name":"test-cluster","code":"TMY01I","db_name":"test","db_protocol":"mysql","db_service":"test-mysql","db_uri":"localhost:3306","db_user":"alice","ei":22,"event":"db.session.mysql.statements.execute","parameters":null,"sid":"test-session","statement_id":222,"time":"2022-02-22T22:22:22.222Z","uid":"test-id","user":"alice@example.com"}`,
			event: apievents.MySQLStatementExecute{
				Metadata: apievents.Metadata{
					Index:       22,
					ID:          "test-id",
					Type:        DatabaseSessionMySQLStatementExecuteEvent,
					Code:        MySQLStatementExecuteCode,
					Time:        time.Date(2022, 02, 22, 22, 22, 22, 222*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "test-session",
				},
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService:  "test-mysql",
					DatabaseProtocol: "mysql",
					DatabaseURI:      "localhost:3306",
					DatabaseName:     "test",
					DatabaseUser:     "alice",
				},
				StatementID: 222,
			},
		},
		{
			name: "MySQL statement send long data",
			json: `{"cluster_name":"test-cluster","code":"TMY02I","db_name":"test","db_protocol":"mysql","db_service":"test-mysql","data_size":55,"db_uri":"localhost:3306","db_user":"alice","ei":22,"event":"db.session.mysql.statements.send_long_data","parameter_id":5,"sid":"test-session","statement_id":222,"time":"2022-02-22T22:22:22.222Z","uid":"test-id","user":"alice@example.com"}`,
			event: apievents.MySQLStatementSendLongData{
				Metadata: apievents.Metadata{
					Index:       22,
					ID:          "test-id",
					Type:        DatabaseSessionMySQLStatementSendLongDataEvent,
					Code:        MySQLStatementSendLongDataCode,
					Time:        time.Date(2022, 02, 22, 22, 22, 22, 222*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "test-session",
				},
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService:  "test-mysql",
					DatabaseProtocol: "mysql",
					DatabaseURI:      "localhost:3306",
					DatabaseName:     "test",
					DatabaseUser:     "alice",
				},
				ParameterID: 5,
				StatementID: 222,
				DataSize:    55,
			},
		},
		{
			name: "MySQL statement close",
			json: `{"cluster_name":"test-cluster","code":"TMY03I","db_name":"test","db_protocol":"mysql","db_service":"test-mysql","db_uri":"localhost:3306","db_user":"alice","ei":22,"event":"db.session.mysql.statements.close","sid":"test-session","statement_id":222,"time":"2022-02-22T22:22:22.222Z","uid":"test-id","user":"alice@example.com"}`,
			event: apievents.MySQLStatementClose{
				Metadata: apievents.Metadata{
					Index:       22,
					ID:          "test-id",
					Type:        DatabaseSessionMySQLStatementCloseEvent,
					Code:        MySQLStatementCloseCode,
					Time:        time.Date(2022, 02, 22, 22, 22, 22, 222*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "test-session",
				},
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService:  "test-mysql",
					DatabaseProtocol: "mysql",
					DatabaseURI:      "localhost:3306",
					DatabaseName:     "test",
					DatabaseUser:     "alice",
				},
				StatementID: 222,
			},
		},
		{
			name: "MySQL statement reset",
			json: `{"cluster_name":"test-cluster","code":"TMY04I","db_name":"test","db_protocol":"mysql","db_service":"test-mysql","db_uri":"localhost:3306","db_user":"alice","ei":22,"event":"db.session.mysql.statements.reset","sid":"test-session","statement_id":222,"time":"2022-02-22T22:22:22.222Z","uid":"test-id","user":"alice@example.com"}`,
			event: apievents.MySQLStatementReset{
				Metadata: apievents.Metadata{
					Index:       22,
					ID:          "test-id",
					Type:        DatabaseSessionMySQLStatementResetEvent,
					Code:        MySQLStatementResetCode,
					Time:        time.Date(2022, 02, 22, 22, 22, 22, 222*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "test-session",
				},
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService:  "test-mysql",
					DatabaseProtocol: "mysql",
					DatabaseURI:      "localhost:3306",
					DatabaseName:     "test",
					DatabaseUser:     "alice",
				},
				StatementID: 222,
			},
		},
		{
			name: "MySQL statement fetch",
			json: `{"cluster_name":"test-cluster","code":"TMY05I","db_name":"test","db_protocol":"mysql","db_service":"test-mysql","db_uri":"localhost:3306","db_user":"alice","ei":22,"event":"db.session.mysql.statements.fetch","rows_count": 5,"sid":"test-session","statement_id":222,"time":"2022-02-22T22:22:22.222Z","uid":"test-id","user":"alice@example.com"}`,
			event: apievents.MySQLStatementFetch{
				Metadata: apievents.Metadata{
					Index:       22,
					ID:          "test-id",
					Type:        DatabaseSessionMySQLStatementFetchEvent,
					Code:        MySQLStatementFetchCode,
					Time:        time.Date(2022, 02, 22, 22, 22, 22, 222*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "test-session",
				},
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService:  "test-mysql",
					DatabaseProtocol: "mysql",
					DatabaseURI:      "localhost:3306",
					DatabaseName:     "test",
					DatabaseUser:     "alice",
				},
				StatementID: 222,
				RowsCount:   5,
			},
		},
		{
			name: "MySQL statement bulk execute",
			json: `{"cluster_name":"test-cluster","code":"TMY06I","db_name":"test","db_protocol":"mysql","db_service":"test-mysql","db_uri":"localhost:3306","db_user":"alice","ei":22,"event":"db.session.mysql.statements.bulk_execute","parameters":null,"sid":"test-session","statement_id":222,"time":"2022-02-22T22:22:22.222Z","uid":"test-id","user":"alice@example.com"}`,
			event: apievents.MySQLStatementBulkExecute{
				Metadata: apievents.Metadata{
					Index:       22,
					ID:          "test-id",
					Type:        DatabaseSessionMySQLStatementBulkExecuteEvent,
					Code:        MySQLStatementBulkExecuteCode,
					Time:        time.Date(2022, 02, 22, 22, 22, 22, 222*int(time.Millisecond), time.UTC),
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "alice@example.com",
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: "test-session",
				},
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService:  "test-mysql",
					DatabaseProtocol: "mysql",
					DatabaseURI:      "localhost:3306",
					DatabaseName:     "test",
					DatabaseUser:     "alice",
				},
				StatementID: 222,
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			outJSON, err := utils.FastMarshal(tc.event)
			require.NoError(t, err)
			require.JSONEq(t, tc.json, string(outJSON))

			// unmarshal back into the type and compare the values
			outEvent := reflect.New(reflect.TypeOf(tc.event))
			err = json.Unmarshal(outJSON, outEvent.Interface())
			require.NoError(t, err)
			require.Equal(t, tc.event, outEvent.Elem().Interface())
		})
	}
}

// TestEvents tests that all events can be converted and processed correctly.
func TestEvents(t *testing.T) {
	t.Parallel()

	for eventName, eventType := range eventsMap {
		t.Run(fmt.Sprintf("%s OneOf", eventName), func(t *testing.T) {
			converted, err := apievents.ToOneOf(eventType)
			require.NoError(t, err, "failed to convert event type to OneOf, is the event type added to api/types/events/oneof.go?")
			auditEvent, err := apievents.FromOneOf(*converted)
			require.NoError(t, err, "failed to convert OneOf back to an Audit event")
			require.IsType(t, eventType, auditEvent, "FromOneOf did not convert the event type correctly")
		})

		t.Run(fmt.Sprintf("%s EventFields", eventName), func(t *testing.T) {
			auditEvent, err := FromEventFields(EventFields{EventType: eventName})
			require.NoError(t, err, "failed to convert EventFields to an Audit event, is the event type added to lib/events/dynamic.go?")
			require.IsType(t, eventType, auditEvent, "FromEventFields did not convert the event type correctly")
		})
	}
}

func TestTrimToMaxSize(t *testing.T) {
	t.Parallel()

	for eventName, eventMsg := range eventsMap {
		t.Run(eventName, func(t *testing.T) {
			// clone the message to avoid modifying the original in the global map
			event := proto.Clone(toV2Proto(t, eventMsg))
			setProtoFields(event)

			auditEvent := protoadapt.MessageV1Of(event).(apievents.AuditEvent)
			size := auditEvent.Size()
			maxSize := int(float32(size) * 0.8)

			trimmedAuditEvent := auditEvent.TrimToMaxSize(maxSize)
			if trimmedAuditEvent.Size() == auditEvent.Size() {
				t.Skipf("skipping %s, event does not have any fields to trim", eventName)
			}
			trimmedEvent := toV2Proto(t, trimmedAuditEvent)

			require.NotEqual(t, auditEvent, trimmedEvent)
			require.LessOrEqual(t, trimmedAuditEvent.Size(), maxSize)
			if trimmedAuditEvent.Size() != maxSize {
				t.Logf("original event: %s\ntrimmed event: %s", protojson.Format(event), protojson.Format(trimmedEvent))
			}

			// ensure Metadata hasn't been trimmed
			require.Equal(t, auditEvent.GetID(), trimmedAuditEvent.GetID())
			require.Equal(t, auditEvent.GetCode(), trimmedAuditEvent.GetCode())
			require.Equal(t, auditEvent.GetType(), trimmedAuditEvent.GetType())
			require.Equal(t, auditEvent.GetClusterName(), trimmedAuditEvent.GetClusterName())
		})
	}
}

type testingVal interface {
	Helper()
	require.TestingT
}

func setProtoFields(msg proto.Message) {
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()

	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if m.Has(fd) {
			continue
		}

		if fd.IsList() {
			// Handle repeated fields
			listValue := m.Mutable(fd).List()
			if fd.Kind() == protoreflect.MessageKind {
				listMsg := listValue.AppendMutable().Message()
				setProtoFields(listMsg.Interface())
			} else {
				listValue.Append(getDefaultValue(m, fd))
			}
			continue
		}

		switch fd.Kind() {
		case protoreflect.MessageKind:
			if fd.IsMap() {
				// Handle map values
				mapValue := m.Mutable(fd).Map()
				keyDesc := fd.MapKey()
				valueDesc := fd.MapValue()

				keyVal := getDefaultValue(m, keyDesc).MapKey()
				var valueVal protoreflect.Value

				if valueDesc.Kind() == protoreflect.MessageKind {
					valueMsg := mapValue.NewValue().Message()
					setProtoFields(valueMsg.Interface())
					valueVal = protoreflect.ValueOfMessage(valueMsg)
				} else {
					valueVal = getDefaultValue(m, valueDesc)
				}

				mapValue.Set(keyVal, valueVal)
			} else {
				// Handle singular message fields
				nestedMsg := m.Mutable(fd).Message()
				setProtoFields(nestedMsg.Interface())
			}
		default:
			m.Set(fd, getDefaultValue(m, fd))
		}
	}
}

const metadataString = "some metadata"

var (
	eventString = strings.Repeat("umai", 170)
)

func getDefaultValue(m protoreflect.Message, fd protoreflect.FieldDescriptor) protoreflect.Value {
	strVal := metadataString
	msgName := string(m.Descriptor().Name())
	// set shorter strings for metadata fields which won't be trimmed
	if msgName == "CommandMetadata" || !strings.Contains(msgName, "Metadata") {
		strVal = eventString
	}

	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(true)
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		return protoreflect.ValueOfInt64(6)
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		return protoreflect.ValueOfUint64(7)
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(3.14)
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(strVal)
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes([]byte(strVal))
	case protoreflect.EnumKind:
		enumValues := fd.Enum().Values()
		if enumValues.Len() > 0 {
			return protoreflect.ValueOfEnum(enumValues.Get(0).Number())
		}
	case protoreflect.MessageKind:
		// Handle singular message fields
		nestedMsg := m.NewField(fd).Message()
		setProtoFields(nestedMsg.Interface())
		return protoreflect.ValueOfMessage(nestedMsg)
	default:
		panic(fmt.Sprintf("unhandled field kind: %s", fd.Kind()))
	}
	return protoreflect.Value{} // This should never happen
}

func toV2Proto(t testingVal, e apievents.AuditEvent) protoreflect.ProtoMessage {
	t.Helper()

	pm, ok := e.(protoiface.MessageV1)
	require.True(t, ok)
	return protoadapt.MessageV2Of(pm)
}
