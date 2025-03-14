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

package decision

import (
	"context"

	"github.com/gravitational/trace"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
)

// GenerateDryRunSSHIdentity generates an ssh identity for use in dry-run decision requests. Identities of this kind
// are for introspection purposes only and decisions produced based on them may not be reflective of actual access
// access control decisions.
func (s *Service) GenerateDryRunSSHIdentity(ctx context.Context, req *decisionpb.DryRunIdentity) (*decisionpb.SSHIdentity, error) {
	// XXX: modeled heavily off of auth/methods.go login logic. this is for use with dry-run requests only
	// and is *not* suitable for making any binding access decisions fo any kind.

	// TODO(fspmarshall): refactor identity setup related logic in lib/auth so that this method and our
	// various equivalent auth logic can have a single source of truth.

	// get the core state user configuration
	user, err := s.cfg.AccessPoint.GetUser(ctx, req.Username, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// generate a user login state value for the user (this will apply dynamic configuration such as access lists)
	userState, err := s.cfg.ULSGenerator.GeneratePureULS(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessInfo := services.AccessInfoFromUserState(userState)

	localClusterName, err := s.getLocalClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	checker, err := services.NewAccessChecker(accessInfo, localClusterName, s.cfg.AccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedLogins, err := checker.CheckLoginDuration(0 /* all logins regardless of ttl */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return SSHIdentityFromSSHCA(&sshca.Identity{
		Username:              user.GetName(),
		Principals:            allowedLogins,
		Roles:                 checker.RoleNames(),
		PermitPortForwarding:  checker.CanPortForward(),
		PermitAgentForwarding: checker.CanForwardAgents(),
		PermitX11Forwarding:   checker.PermitX11Forwarding(),
		Traits:                userState.GetTraits(),
		CertificateExtensions: checker.CertificateExtensions(),
	}), nil
}
