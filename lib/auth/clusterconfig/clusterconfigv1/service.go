// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package clusterconfigv1

import (
	"context"

	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
)

// ServiceConfig contain dependencies required to create a [Service].
type ServiceConfig struct {
	Authorizer  authz.Authorizer
	AccessGraph AccessGraphConfig
}

// AccessGraphConfig contains the configuration about the access graph service
// and whether it is enabled or not.
type AccessGraphConfig struct {
	// Enabled is a flag that indicates whether the access graph service is enabled.
	Enabled bool
	// Address is the address of the access graph service. The address is in the
	// form of "host:port".
	Address string
	// CA is the PEM-encoded CA certificate of the access graph service.
	CA []byte
	// Insecure is a flag that indicates whether the access graph service should
	// skip verifying the server's certificate chain and host name.
	Insecure bool
}

// Service implements the teleport.clusterconfig.v1.ClusterConfigService RPC service.
type Service struct {
	clusterconfigpb.UnimplementedClusterConfigServiceServer

	authorizer  authz.Authorizer
	accessGraph AccessGraphConfig
}

// NewService validates the provided configuration and returns a [Service].
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}

	return &Service{authorizer: cfg.Authorizer, accessGraph: cfg.AccessGraph}, nil
}

func (s *Service) GetClusterAccessGraphConfig(ctx context.Context, _ *clusterconfigpb.GetClusterAccessGraphConfigRequest) (*clusterconfigpb.GetClusterAccessGraphConfigResponse, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleProxy)) && !authz.HasBuiltinRole(*authzCtx, string(types.RoleDiscovery)) {
		return nil, trace.AccessDenied("this request can be only executed by proxy or discovery services")
	}

	// If the policy feature is disabled in the license, return a disabled response.
	if !modules.GetModules().Features().Policy.Enabled && !modules.GetModules().Features().AccessGraph {
		return &clusterconfigpb.GetClusterAccessGraphConfigResponse{
			AccessGraph: &clusterconfigpb.AccessGraphConfig{
				Enabled: false,
			},
		}, nil
	}

	return &clusterconfigpb.GetClusterAccessGraphConfigResponse{
		AccessGraph: &clusterconfigpb.AccessGraphConfig{
			Enabled:  s.accessGraph.Enabled,
			Address:  s.accessGraph.Address,
			Ca:       s.accessGraph.CA,
			Insecure: s.accessGraph.Insecure,
		},
	}, nil
}
