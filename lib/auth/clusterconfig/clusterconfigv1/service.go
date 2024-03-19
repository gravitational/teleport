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
	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/lib/authz"
)

// ServiceConfig contain dependencies required to create a [Service].
type ServiceConfig struct {
	Authorizer authz.Authorizer
}

// Service implements the teleport.clusterconfig.v1.ClusterConfigService RPC service.
type Service struct {
	clusterconfigpb.UnimplementedClusterConfigServiceServer

	authorizer authz.Authorizer
}

// NewService validates the provided configuration and returns a [Service].
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}

	return &Service{authorizer: cfg.Authorizer}, nil
}
