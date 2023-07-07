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

	eventv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/event/v1"
	"github.com/gravitational/teleport/api/types"
)

// EventToGRPC converts types.Event to *eventv1.Event.
func EventToGRPC(in types.Event) (*eventv1.Event, error) {
	eventType, err := EventTypeToGRPC(in.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := &eventv1.Event{
		Type: eventType,
	}
	if in.Type == types.OpInit {
		watchStatus, ok := in.Resource.(*types.WatchStatusV1)
		if !ok {
			return nil, trace.BadParameter("unexpected resource type %T for Init event", in.Resource)
		}
		out.Resource = &eventv1.Event_WatchStatus{
			WatchStatus: watchStatus,
		}
		return out, nil
	}
	switch r := in.Resource.(type) {
	case *types.ResourceHeader:
		out.Resource = &eventv1.Event_ResourceHeader{
			ResourceHeader: r,
		}
	case *types.CertAuthorityV2:
		out.Resource = &eventv1.Event_CertAuthority{
			CertAuthority: r,
		}
	case *types.StaticTokensV2:
		out.Resource = &eventv1.Event_StaticTokens{
			StaticTokens: r,
		}
	case *types.ProvisionTokenV2:
		out.Resource = &eventv1.Event_ProvisionToken{
			ProvisionToken: r,
		}
	case *types.ClusterNameV2:
		out.Resource = &eventv1.Event_ClusterName{
			ClusterName: r,
		}
	case *types.UserV2:
		out.Resource = &eventv1.Event_User{
			User: r,
		}
	case *types.RoleV6:
		out.Resource = &eventv1.Event_Role{
			Role: r,
		}
	case *types.Namespace:
		out.Resource = &eventv1.Event_Namespace{
			Namespace: r,
		}
	case *types.ServerV2:
		out.Resource = &eventv1.Event_Server{
			Server: r,
		}
	case *types.ReverseTunnelV2:
		out.Resource = &eventv1.Event_ReverseTunnel{
			ReverseTunnel: r,
		}
	case *types.TunnelConnectionV2:
		out.Resource = &eventv1.Event_TunnelConnection{
			TunnelConnection: r,
		}
	case *types.AccessRequestV3:
		out.Resource = &eventv1.Event_AccessRequest{
			AccessRequest: r,
		}
	case *types.WebSessionV2:
		switch r.GetSubKind() {
		case types.KindAppSession:
			out.Resource = &eventv1.Event_AppSession{
				AppSession: r,
			}
		case types.KindWebSession:
			out.Resource = &eventv1.Event_WebSession{
				WebSession: r,
			}
		case types.KindSnowflakeSession:
			out.Resource = &eventv1.Event_SnowflakeSession{
				SnowflakeSession: r,
			}
		case types.KindSAMLIdPSession:
			out.Resource = &eventv1.Event_SamlIdpSession{
				SamlIdpSession: r,
			}
		default:
			return nil, trace.BadParameter("only %q supported", types.WebSessionSubKinds)
		}
	case *types.WebTokenV3:
		out.Resource = &eventv1.Event_WebToken{
			WebToken: r,
		}
	case *types.RemoteClusterV3:
		out.Resource = &eventv1.Event_RemoteCluster{
			RemoteCluster: r,
		}
	case *types.KubernetesServerV3:
		out.Resource = &eventv1.Event_KubernetesServer{
			KubernetesServer: r,
		}
	case *types.KubernetesClusterV3:
		out.Resource = &eventv1.Event_KubernetesCluster{
			KubernetesCluster: r,
		}
	case *types.AppServerV3:
		out.Resource = &eventv1.Event_AppServer{
			AppServer: r,
		}
	case *types.DatabaseServerV3:
		out.Resource = &eventv1.Event_DatabaseServer{
			DatabaseServer: r,
		}
	case *types.DatabaseV3:
		out.Resource = &eventv1.Event_Database{
			Database: r,
		}
	case *types.AppV3:
		out.Resource = &eventv1.Event_App{
			App: r,
		}
	case *types.ClusterAuditConfigV2:
		out.Resource = &eventv1.Event_ClusterAuditConfig{
			ClusterAuditConfig: r,
		}
	case *types.ClusterNetworkingConfigV2:
		out.Resource = &eventv1.Event_ClusterNetworkingConfig{
			ClusterNetworkingConfig: r,
		}
	case *types.SessionRecordingConfigV2:
		out.Resource = &eventv1.Event_SessionRecordingConfig{
			SessionRecordingConfig: r,
		}
	case *types.AuthPreferenceV2:
		out.Resource = &eventv1.Event_AuthPreference{
			AuthPreference: r,
		}
	case *types.LockV2:
		out.Resource = &eventv1.Event_Lock{
			Lock: r,
		}
	case *types.NetworkRestrictionsV4:
		out.Resource = &eventv1.Event_NetworkRestrictions{
			NetworkRestrictions: r,
		}
	case *types.WindowsDesktopServiceV3:
		out.Resource = &eventv1.Event_WindowsDesktopService{
			WindowsDesktopService: r,
		}
	case *types.WindowsDesktopV3:
		out.Resource = &eventv1.Event_WindowsDesktop{
			WindowsDesktop: r,
		}
	case *types.InstallerV1:
		out.Resource = &eventv1.Event_Installer{
			Installer: r,
		}
	case *types.UIConfigV1:
		out.Resource = &eventv1.Event_UiConfig{
			UiConfig: r,
		}
	case *types.DatabaseServiceV1:
		out.Resource = &eventv1.Event_DatabaseService{
			DatabaseService: r,
		}
	case *types.SAMLIdPServiceProviderV1:
		out.Resource = &eventv1.Event_SamlIdpServiceProvider{
			SamlIdpServiceProvider: r,
		}
	case *types.UserGroupV1:
		out.Resource = &eventv1.Event_UserGroup{
			UserGroup: r,
		}
	case *types.OktaImportRuleV1:
		out.Resource = &eventv1.Event_OktaImportRule{
			OktaImportRule: r,
		}
	case *types.OktaAssignmentV1:
		out.Resource = &eventv1.Event_OktaAssignment{
			OktaAssignment: r,
		}
	case *types.IntegrationV1:
		out.Resource = &eventv1.Event_Integration{
			Integration: r,
		}
	case *types.HeadlessAuthentication:
		out.Resource = &eventv1.Event_HeadlessAuthentication{
			HeadlessAuthentication: r,
		}
	default:
		return nil, trace.BadParameter("resource type %T is not supported", in.Resource)
	}
	return out, nil
}

// EventTypeToGRPC converts types.OpType to eventv1.Operation
func EventTypeToGRPC(in types.OpType) (eventv1.Operation, error) {
	switch in {
	case types.OpInit:
		return eventv1.Operation_OPERATION_INIT, nil
	case types.OpPut:
		return eventv1.Operation_OPERATION_PUT, nil
	case types.OpDelete:
		return eventv1.Operation_OPERATION_DELETE, nil
	default:
		return -1, trace.BadParameter("event type %v is not supported", in)
	}
}

// EventFromGRPC converts *eventv1.Event to *types.Event
func EventFromGRPC(in *eventv1.Event) (*types.Event, error) {
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
	} else if r := in.GetKubernetesServer(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetKubernetesCluster(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetInstaller(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetUiConfig(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetDatabaseService(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetSamlIdpServiceProvider(); r != nil {
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
	} else {
		return nil, trace.BadParameter("received unsupported resource %T", in.Resource)
	}
}

// EventTypeFromGRPC converts eventv1.Operation to types.OpType
func EventTypeFromGRPC(in eventv1.Operation) (types.OpType, error) {
	switch in {
	case eventv1.Operation_OPERATION_INIT:
		return types.OpInit, nil
	case eventv1.Operation_OPERATION_PUT:
		return types.OpPut, nil
	case eventv1.Operation_OPERATION_DELETE:
		return types.OpDelete, nil
	default:
		return types.OpInvalid, trace.BadParameter("unsupported operation type: %v", in)
	}
}
