// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/subca"
)

func (a *Server) loadCAOverrideResolverForCA(ctx context.Context, ca types.CertAuthority) (*subca.CAOverrideResolver, error) {
	r, err := subca.LoadCAOverrideResolver(
		ctx,
		a.Cache,
		modules.GetModules().IsEnterpriseBuild(),
		types.CertAuthorityOverrideID{
			ClusterName: ca.GetClusterName(),
			CAType:      string(ca.GetType()),
		})
	return r, trace.Wrap(err, "load CA override resolver")
}
