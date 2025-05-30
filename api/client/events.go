// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	accesslistv1conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	discoveryconfigv1conv "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
	"github.com/gravitational/teleport/api/types/secreports"
	secreprotsv1conv "github.com/gravitational/teleport/api/types/secreports/convert/v1"
	"github.com/gravitational/teleport/api/types/userloginstate"
	userloginstatev1conv "github.com/gravitational/teleport/api/types/userloginstate/convert/v1"
)

// EventToGRPC converts types.Event to proto.Event.
func EventToGRPC(in types.Event) (*proto.Event, error) {
	eventType, err := EventTypeToGRPC(in.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := proto.Event{
		Type: eventType,
	}
	if in.Type == types.OpInit {
		watchStatus, ok := in.Resource.(*types.WatchStatusV1)
		if !ok {
			return nil, trace.BadParameter("unexpected resource type %T for Init event", in.Resource)
		}
		out.Resource = &proto.Event_WatchStatus{
			WatchStatus: watchStatus,
		}
		return &out, nil
	}
	switch r := in.Resource.(type) {
	case types.Resource153UnwrapperT[*kubewaitingcontainerpb.KubernetesWaitingContainer]:
		out.Resource = &proto.Event_KubernetesWaitingContainer{
			KubernetesWaitingContainer: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*notificationsv1.Notification]:
		out.Resource = &proto.Event_UserNotification{
			UserNotification: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*notificationsv1.GlobalNotification]:
		out.Resource = &proto.Event_GlobalNotification{
			GlobalNotification: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*accessmonitoringrulesv1.AccessMonitoringRule]:
		out.Resource = &proto.Event_AccessMonitoringRule{
			AccessMonitoringRule: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*crownjewelv1.CrownJewel]:
		out.Resource = &proto.Event_CrownJewel{
			CrownJewel: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*dbobjectv1.DatabaseObject]:
		out.Resource = &proto.Event_DatabaseObject{
			DatabaseObject: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*machineidv1.BotInstance]:
		out.Resource = &proto.Event_BotInstance{
			BotInstance: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*clusterconfigpb.AccessGraphSettings]:
		out.Resource = &proto.Event_AccessGraphSettings{
			AccessGraphSettings: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*machineidv1.SPIFFEFederation]:
		out.Resource = &proto.Event_SPIFFEFederation{
			SPIFFEFederation: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*userprovisioningpb.StaticHostUser]:
		out.Resource = &proto.Event_StaticHostUserV2{
			StaticHostUserV2: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*autoupdate.AutoUpdateConfig]:
		out.Resource = &proto.Event_AutoUpdateConfig{
			AutoUpdateConfig: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*autoupdate.AutoUpdateVersion]:
		out.Resource = &proto.Event_AutoUpdateVersion{
			AutoUpdateVersion: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*usertasksv1.UserTask]:
		out.Resource = &proto.Event_UserTask{
			UserTask: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*provisioningv1.PrincipalState]:
		out.Resource = &proto.Event_ProvisioningPrincipalState{
			ProvisioningPrincipalState: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*autoupdate.AutoUpdateAgentRollout]:
		out.Resource = &proto.Event_AutoUpdateAgentRollout{
			AutoUpdateAgentRollout: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*autoupdate.AutoUpdateAgentReport]:
		out.Resource = &proto.Event_AutoUpdateAgentReport{
			AutoUpdateAgentReport: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*accessv1.ScopedRole]:
		out.Resource = &proto.Event_ScopedRole{
			ScopedRole: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*accessv1.ScopedRoleAssignment]:
		out.Resource = &proto.Event_ScopedRoleAssignment{
			ScopedRoleAssignment: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*identitycenterv1.Account]:
		out.Resource = &proto.Event_IdentityCenterAccount{
			IdentityCenterAccount: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*identitycenterv1.PrincipalAssignment]:
		out.Resource = &proto.Event_IdentityCenterPrincipalAssignment{
			IdentityCenterPrincipalAssignment: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*identitycenterv1.AccountAssignment]:
		out.Resource = &proto.Event_IdentityCenterAccountAssignment{
			IdentityCenterAccountAssignment: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*workloadidentityv1pb.WorkloadIdentity]:
		out.Resource = &proto.Event_WorkloadIdentity{
			WorkloadIdentity: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*workloadidentityv1pb.WorkloadIdentityX509Revocation]:
		out.Resource = &proto.Event_WorkloadIdentityX509Revocation{
			WorkloadIdentityX509Revocation: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*recordingencryptionv1.RecordingEncryption]:
		out.Resource = &proto.Event_RecordingEncryption{
			RecordingEncryption: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*healthcheckconfigv1.HealthCheckConfig]:
		out.Resource = &proto.Event_HealthCheckConfig{
			HealthCheckConfig: r.UnwrapT(),
		}
	case types.Resource153UnwrapperT[*presencev1.RelayServer]:
		out.Resource = &proto.Event_RelayServer{
			RelayServer: r.UnwrapT(),
		}
	case *types.ResourceHeader:
		out.Resource = &proto.Event_ResourceHeader{
			ResourceHeader: r,
		}
	case *types.CertAuthorityV2:
		out.Resource = &proto.Event_CertAuthority{
			CertAuthority: r,
		}
	case *types.StaticTokensV2:
		out.Resource = &proto.Event_StaticTokens{
			StaticTokens: r,
		}
	case *types.ProvisionTokenV2:
		out.Resource = &proto.Event_ProvisionToken{
			ProvisionToken: r,
		}
	case *types.ClusterNameV2:
		out.Resource = &proto.Event_ClusterName{
			ClusterName: r,
		}
	case *types.UserV2:
		out.Resource = &proto.Event_User{
			User: r,
		}
	case *types.RoleV6:
		out.Resource = &proto.Event_Role{
			Role: r,
		}
	case *types.Namespace:
		out.Resource = &proto.Event_Namespace{
			Namespace: r,
		}
	case *types.ServerV2:
		out.Resource = &proto.Event_Server{
			Server: r,
		}
	case *types.ReverseTunnelV2:
		out.Resource = &proto.Event_ReverseTunnel{
			ReverseTunnel: r,
		}
	case *types.TunnelConnectionV2:
		out.Resource = &proto.Event_TunnelConnection{
			TunnelConnection: r,
		}
	case *types.AccessRequestV3:
		out.Resource = &proto.Event_AccessRequest{
			AccessRequest: r,
		}
	case *types.WebSessionV2:
		switch r.GetSubKind() {
		case types.KindAppSession:
			out.Resource = &proto.Event_AppSession{
				AppSession: r,
			}
		case types.KindWebSession:
			out.Resource = &proto.Event_WebSession{
				WebSession: r,
			}
		case types.KindSnowflakeSession:
			out.Resource = &proto.Event_SnowflakeSession{
				SnowflakeSession: r,
			}
		default:
			return nil, trace.BadParameter("only %q supported", types.WebSessionSubKinds)
		}
	case *types.WebTokenV3:
		out.Resource = &proto.Event_WebToken{
			WebToken: r,
		}
	case *types.RemoteClusterV3:
		out.Resource = &proto.Event_RemoteCluster{
			RemoteCluster: r,
		}
	case *types.KubernetesServerV3:
		out.Resource = &proto.Event_KubernetesServer{
			KubernetesServer: r,
		}
	case *types.KubernetesClusterV3:
		out.Resource = &proto.Event_KubernetesCluster{
			KubernetesCluster: r,
		}
	case *types.AppServerV3:
		out.Resource = &proto.Event_AppServer{
			AppServer: r,
		}
	case *types.DatabaseServerV3:
		out.Resource = &proto.Event_DatabaseServer{
			DatabaseServer: r,
		}
	case *types.DatabaseV3:
		out.Resource = &proto.Event_Database{
			Database: r,
		}
	case *types.AppV3:
		out.Resource = &proto.Event_App{
			App: r,
		}
	case *types.ClusterAuditConfigV2:
		out.Resource = &proto.Event_ClusterAuditConfig{
			ClusterAuditConfig: r,
		}
	case *types.ClusterNetworkingConfigV2:
		out.Resource = &proto.Event_ClusterNetworkingConfig{
			ClusterNetworkingConfig: r,
		}
	case *types.SessionRecordingConfigV2:
		out.Resource = &proto.Event_SessionRecordingConfig{
			SessionRecordingConfig: r,
		}
	case *types.AuthPreferenceV2:
		out.Resource = &proto.Event_AuthPreference{
			AuthPreference: r,
		}
	case *types.LockV2:
		out.Resource = &proto.Event_Lock{
			Lock: r,
		}
	case *types.NetworkRestrictionsV4:
		out.Resource = &proto.Event_NetworkRestrictions{
			NetworkRestrictions: r,
		}
	case *types.WindowsDesktopServiceV3:
		out.Resource = &proto.Event_WindowsDesktopService{
			WindowsDesktopService: r,
		}
	case *types.WindowsDesktopV3:
		out.Resource = &proto.Event_WindowsDesktop{
			WindowsDesktop: r,
		}
	case *types.DynamicWindowsDesktopV1:
		out.Resource = &proto.Event_DynamicWindowsDesktop{
			DynamicWindowsDesktop: r,
		}
	case *types.InstallerV1:
		out.Resource = &proto.Event_Installer{
			Installer: r,
		}
	case *types.UIConfigV1:
		out.Resource = &proto.Event_UIConfig{
			UIConfig: r,
		}
	case *types.DatabaseServiceV1:
		out.Resource = &proto.Event_DatabaseService{
			DatabaseService: r,
		}
	case *types.SAMLIdPServiceProviderV1:
		out.Resource = &proto.Event_SAMLIdPServiceProvider{
			SAMLIdPServiceProvider: r,
		}
	case *types.UserGroupV1:
		out.Resource = &proto.Event_UserGroup{
			UserGroup: r,
		}
	case *types.OktaImportRuleV1:
		out.Resource = &proto.Event_OktaImportRule{
			OktaImportRule: r,
		}
	case *types.OktaAssignmentV1:
		out.Resource = &proto.Event_OktaAssignment{
			OktaAssignment: r,
		}
	case *types.IntegrationV1:
		out.Resource = &proto.Event_Integration{
			Integration: r,
		}
	case *types.HeadlessAuthentication:
		out.Resource = &proto.Event_HeadlessAuthentication{
			HeadlessAuthentication: r,
		}
	case *accesslist.AccessList:
		out.Resource = &proto.Event_AccessList{
			AccessList: accesslistv1conv.ToProto(r),
		}
	case *userloginstate.UserLoginState:
		out.Resource = &proto.Event_UserLoginState{
			UserLoginState: userloginstatev1conv.ToProto(r),
		}
	case *accesslist.AccessListMember:
		out.Resource = &proto.Event_AccessListMember{
			AccessListMember: accesslistv1conv.ToMemberProto(r),
		}
	case *discoveryconfig.DiscoveryConfig:
		out.Resource = &proto.Event_DiscoveryConfig{
			DiscoveryConfig: discoveryconfigv1conv.ToProto(r),
		}
	case *secreports.AuditQuery:
		out.Resource = &proto.Event_AuditQuery{
			AuditQuery: secreprotsv1conv.ToProtoAuditQuery(r),
		}
	case *secreports.Report:
		out.Resource = &proto.Event_Report{
			Report: secreprotsv1conv.ToProtoReport(r),
		}
	case *secreports.ReportState:
		out.Resource = &proto.Event_ReportState{
			ReportState: secreprotsv1conv.ToProtoReportState(r),
		}
	case *accesslist.Review:
		out.Resource = &proto.Event_AccessListReview{
			AccessListReview: accesslistv1conv.ToReviewProto(r),
		}
	case *types.PluginStaticCredentialsV1:
		out.Resource = &proto.Event_PluginStaticCredentials{
			PluginStaticCredentials: r,
		}
	default:
		return nil, trace.BadParameter("resource type %T is not supported", in.Resource)
	}
	return &out, nil
}

// EventTypeToGRPC converts types.OpType to proto.Operation
func EventTypeToGRPC(in types.OpType) (proto.Operation, error) {
	switch in {
	case types.OpInit:
		return proto.Operation_INIT, nil
	case types.OpPut:
		return proto.Operation_PUT, nil
	case types.OpDelete:
		return proto.Operation_DELETE, nil
	default:
		return -1, trace.BadParameter("event type %v is not supported", in)
	}
}

// EventFromGRPC converts proto.Event to types.Event
func EventFromGRPC(in *proto.Event) (*types.Event, error) {
	eventType, err := EventTypeFromGRPC(in.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := types.Event{
		Type: eventType,
	}
	if eventType == types.OpInit {
		if r := in.GetWatchStatus(); r != nil {
			out.Resource = r
		}
		return &out, nil
	}
	if r := in.GetResourceHeader(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetCertAuthority(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetStaticTokens(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetProvisionToken(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetClusterName(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetUser(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetRole(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetNamespace(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetServer(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetReverseTunnel(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetTunnelConnection(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetAccessRequest(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetSnowflakeSession(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetAppSession(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetWebSession(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetWebToken(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetRemoteCluster(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetAppServer(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetDatabaseServer(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetApp(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetDatabase(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetClusterAuditConfig(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetClusterNetworkingConfig(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetSessionRecordingConfig(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetAuthPreference(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetLock(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetNetworkRestrictions(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetWindowsDesktopService(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetWindowsDesktop(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetDynamicWindowsDesktop(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetKubernetesServer(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetKubernetesCluster(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetInstaller(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetUIConfig(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetDatabaseService(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetSAMLIdPServiceProvider(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetUserGroup(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetOktaImportRule(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetOktaAssignment(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetIntegration(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetHeadlessAuthentication(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetAccessList(); r != nil {
		out.Resource, err = accesslistv1conv.FromProto(
			r,
			accesslistv1conv.WithOwnersIneligibleStatusField(r.GetSpec().GetOwners()),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &out, nil
	} else if r := in.GetUserLoginState(); r != nil {
		out.Resource, err = userloginstatev1conv.FromProto(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &out, nil
	} else if r := in.GetAccessListMember(); r != nil {
		out.Resource, err = accesslistv1conv.FromMemberProto(
			r,
			accesslistv1conv.WithMemberIneligibleStatusField(r),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &out, nil
	} else if r := in.GetDiscoveryConfig(); r != nil {
		out.Resource, err = discoveryconfigv1conv.FromProto(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &out, nil
	} else if r := in.GetAuditQuery(); r != nil {
		out.Resource, err = secreprotsv1conv.FromProtoAuditQuery(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &out, nil
	} else if r := in.GetReport(); r != nil {
		out.Resource, err = secreprotsv1conv.FromProtoReport(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &out, nil
	} else if r := in.GetReportState(); r != nil {
		out.Resource, err = secreprotsv1conv.FromProtoReportState(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &out, nil
	} else if r := in.GetAccessListReview(); r != nil {
		out.Resource, err = accesslistv1conv.FromReviewProto(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &out, nil
	} else if r := in.GetKubernetesWaitingContainer(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetUserNotification(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetGlobalNotification(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetAccessMonitoringRule(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetCrownJewel(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetDatabaseObject(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetBotInstance(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetAccessGraphSettings(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetSPIFFEFederation(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetStaticHostUserV2(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetAutoUpdateConfig(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetAutoUpdateVersion(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetAutoUpdateAgentRollout(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetAutoUpdateAgentReport(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetScopedRole(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetScopedRoleAssignment(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetUserTask(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetProvisioningPrincipalState(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetIdentityCenterAccount(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetIdentityCenterPrincipalAssignment(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetIdentityCenterAccountAssignment(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetPluginStaticCredentials(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetWorkloadIdentity(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetWorkloadIdentityX509Revocation(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetHealthCheckConfig(); r != nil {
		out.Resource = types.Resource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetRelayServer(); r != nil {
		out.Resource = types.ProtoResource153ToLegacy(r)
		return &out, nil
	} else if r := in.GetRecordingEncryption(); r != nil {
		out.Resource = types.ProtoResource153ToLegacy(r)
		return &out, nil
	} else {
		return nil, trace.BadParameter("received unsupported resource %T", in.Resource)
	}
}

// EventTypeFromGRPC converts proto.Operation to types.OpType
func EventTypeFromGRPC(in proto.Operation) (types.OpType, error) {
	switch in {
	case proto.Operation_INIT:
		return types.OpInit, nil
	case proto.Operation_PUT:
		return types.OpPut, nil
	case proto.Operation_DELETE:
		return types.OpDelete, nil
	default:
		return types.OpInvalid, trace.BadParameter("unsupported operation type: %v", in)
	}
}
