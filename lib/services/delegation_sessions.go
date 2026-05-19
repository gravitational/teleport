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

package services

import (
	"context"

	"github.com/gravitational/trace"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
)

// DelegationSessions is an interface over the DelegationSessions service. This
// interface may also be implemented by a client to allow remote and local
// consumers to access the resource in a similar way.
type DelegationSessions interface {
	// CreateDelegationSession creates a new delegation session.
	CreateDelegationSession(ctx context.Context, session *delegationv1.DelegationSession) (*delegationv1.DelegationSession, error)

	// GetDelegationSession reads a delegation session using its ID.
	GetDelegationSession(ctx context.Context, id string) (*delegationv1.DelegationSession, error)

	// DeleteDelegationSession deletes a delegation session using its ID.
	DeleteDelegationSession(ctx context.Context, id string) error
}

// ValidateDelegationSession validates a DelegationSession object.
func ValidateDelegationSession(p *delegationv1.DelegationSession) error {
	switch {
	case p.GetKind() != types.KindDelegationSession:
		return trace.BadParameter("kind: must be %s", types.KindDelegationSession)
	case p.GetVersion() != types.V1:
		return trace.BadParameter("version: must be %s", types.V1)
	case p.GetMetadata().GetName() == "":
		return trace.BadParameter("metadata.name: is required")
	case p.GetMetadata().GetExpires() == nil:
		return trace.BadParameter("metadata.expires: is required")
	case p.GetSpec().GetUser() == "":
		return trace.BadParameter("spec.user: is required")
	}

	if len(p.GetSpec().GetResources()) == 0 {
		return trace.BadParameter("spec.resources: at least one resource is required")
	}

	var hasWildcard, hasExplicit bool
	for idx, spec := range p.GetSpec().GetResources() {
		if err := ValidateDelegationResourceSpec(spec); err != nil {
			return trace.BadParameter("spec.resources[%d]: invalid resource spec: %v", idx, err)
		}
		if spec.GetKind() == types.Wildcard {
			hasWildcard = true
		} else {
			hasExplicit = true
		}
		if hasWildcard && hasExplicit {
			return trace.BadParameter("spec.resources: wildcard is mutually exclusive with explicit resources")
		}
	}

	if len(p.GetSpec().GetAuthorizedUsers()) == 0 {
		return trace.BadParameter("spec.authorized_users: at least one user is required")
	}

	for idx, user := range p.GetSpec().GetAuthorizedUsers() {
		if user.GetKind() != types.KindBot {
			return trace.BadParameter("spec.authorized_users[%d].kind: must be %s", idx, types.KindBot)
		}
		if user.GetBotName() == "" {
			return trace.BadParameter("spec.authorized_users[%d].bot_name: is required", idx)
		}
	}

	return nil
}

// ValidateDelegationResourceSpec validates a DelegationResourceSpec object.
func ValidateDelegationResourceSpec(s *delegationv1.DelegationResourceSpec) error {
	if s.GetName() == "" {
		return trace.BadParameter("name is required")
	}

	// TODO(boxofrad): implement support for constraints.
	if s.GetConstraints() != nil {
		return trace.BadParameter("constraints are not yet supported")
	}

	switch s.GetKind() {
	case types.KindApp, types.KindDatabase, types.KindNode, types.KindKubernetesCluster, types.KindWindowsDesktop, types.KindGitServer, types.Wildcard:
	case "":
		return trace.BadParameter("kind is required")
	default:
		return trace.BadParameter("invalid kind: %q", s.GetKind())
	}

	switch {
	case s.GetKind() == types.Wildcard && s.GetName() != types.Wildcard:
		return trace.BadParameter("name must also be '*'")
	case s.GetKind() != types.Wildcard && s.GetName() == types.Wildcard:
		return trace.BadParameter("kind must also be '*'")
	}

	return nil
}
