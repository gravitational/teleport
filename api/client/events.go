/*
Copyright 2020 Gravitational, Inc.

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

package client

import (
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
)

// EventToGRPC converts a types.Event to an proto.Event
func EventToGRPC(in types.Event) (*proto.Event, error) {
	eventType, err := eventTypeToGRPC(in.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := proto.Event{
		Type: eventType,
	}
	if in.Type == types.OpInit {
		return &out, nil
	}
	switch r := in.Resource.(type) {
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
	case *types.ClusterConfigV3:
		out.Resource = &proto.Event_ClusterConfig{
			ClusterConfig: r,
		}
	case *types.UserV2:
		out.Resource = &proto.Event_User{
			User: r,
		}
	case *types.RoleV3:
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
		default:
			return nil, trace.BadParameter("only %q supported", types.WebSessionSubKinds)
		}
		// TODO(dmitri): handle WebTokenV1
	case *types.RemoteClusterV3:
		out.Resource = &proto.Event_RemoteCluster{
			RemoteCluster: r,
		}
	case *types.DatabaseServerV3:
		out.Resource = &proto.Event_DatabaseServer{
			DatabaseServer: r,
		}
	default:
		return nil, trace.BadParameter("resource type %T is not supported", in.Resource)
	}
	return &out, nil
}

func eventTypeToGRPC(in types.OpType) (proto.Operation, error) {
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

// EventFromGRPC converts an proto.Event to a types.Event
func EventFromGRPC(in proto.Event) (*types.Event, error) {
	eventType, err := eventTypeFromGRPC(in.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := types.Event{
		Type: eventType,
	}
	if eventType == types.OpInit {
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
	} else if r := in.GetClusterConfig(); r != nil {
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
	} else if r := in.GetAppSession(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetWebSession(); r != nil {
		out.Resource = r
		return &out, nil
		// TODO(dmitri): handle WebTokenV1
	} else if r := in.GetRemoteCluster(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetDatabaseServer(); r != nil {
		out.Resource = r
		return &out, nil
	} else {
		return nil, trace.BadParameter("received unsupported resource %T", in.Resource)
	}
}

func eventTypeFromGRPC(in proto.Operation) (types.OpType, error) {
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
