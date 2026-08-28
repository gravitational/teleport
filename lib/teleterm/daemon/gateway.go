// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package daemon

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

func (s *Service) checkIfGatewayAlreadyExists(targetURI uri.ResourceURI, params CreateGatewayParams) error {
	var resourceParamsCheckFunc func(g gateway.Gateway) error

	switch {
	case targetURI.IsDB():
		resourceParamsCheckFunc = func(g gateway.Gateway) error {
			if g.TargetUser() == params.TargetUser {
				return trace.AlreadyExists("gateway for database %s and user %s already exists", targetURI.GetDbName(), params.TargetUser)
			}
			return nil
		}
	case targetURI.IsKube():
		// Return early for kubes as kube gateways depend on s.shouldReuseGateway.
		return nil
	case targetURI.IsApp():
		resourceParamsCheckFunc = func(g gateway.Gateway) error {
			if g.TargetSubresourceName() == params.TargetSubresourceName {
				if params.TargetSubresourceName != "" {
					return trace.AlreadyExists("gateway for app %s and target port %s already exists", targetURI.GetAppName(), params.TargetSubresourceName)
				} else {
					return trace.AlreadyExists("gateway for app %s already exists", targetURI.GetAppName())
				}
			}
			return nil
		}
	default:
		return trace.NotImplemented("gateway not supported for %s", targetURI.String())
	}

	for _, g := range s.gateways {
		if g.TargetURI() != targetURI {
			continue
		}

		if err := resourceParamsCheckFunc(g); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
