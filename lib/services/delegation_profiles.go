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

package services

import (
	"context"
	"net/url"

	"github.com/gravitational/trace"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
)

// DelegationProfiles is an interface over the DelegationProfile service. This
// interface may also be implemented by a client to allow remote and local
// consumers to access the resource in a similar way.
type DelegationProfiles interface {
	// GetDelegationProfile gets a DelegationProfile by name.
	GetDelegationProfile(
		ctx context.Context, name string,
	) (*delegationv1.DelegationProfile, error)

	// ListDelegationProfiles lists all DelegationProfile resources using Google
	// style pagination.
	ListDelegationProfiles(
		ctx context.Context, pageSize int, lastToken string,
	) ([]*delegationv1.DelegationProfile, string, error)

	// CreateDelegationProfile creates a new DelegationProfile.
	CreateDelegationProfile(
		ctx context.Context,
		delegationProfile *delegationv1.DelegationProfile,
	) (*delegationv1.DelegationProfile, error)

	// DeleteDelegationProfile deletes a DelegationProfile by name.
	DeleteDelegationProfile(ctx context.Context, name string) error

	// UpdateDelegationProfile updates a specific DelegationProfile.
	//
	// The resource must already exist, and, conditional update semantics are
	// used - e.g the submitted resource must have a revision matching the
	// revision of the resource in the backend.
	UpdateDelegationProfile(
		ctx context.Context,
		delegationProfile *delegationv1.DelegationProfile,
	) (*delegationv1.DelegationProfile, error)

	// UpsertDelegationProfile creates or updates a DelegationProfile.
	UpsertDelegationProfile(
		ctx context.Context,
		delegationProfile *delegationv1.DelegationProfile,
	) (*delegationv1.DelegationProfile, error)
}

// MarshalDelegationProfile marshals the DelegationProfile object into a JSON
// byte slice.
func MarshalDelegationProfile(
	object *delegationv1.DelegationProfile,
	opts ...MarshalOption,
) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalDelegationProfile unmarshals the DelegationProfile object from a
// JSON byte slice.
func UnmarshalDelegationProfile(
	data []byte,
	opts ...MarshalOption,
) (*delegationv1.DelegationProfile, error) {
	return UnmarshalProtoResource[*delegationv1.DelegationProfile](data, opts...)
}

// ValidateDelegationProfile validates a DelegationProfile object.
func ValidateDelegationProfile(p *delegationv1.DelegationProfile) error {
	switch {
	case p.GetKind() != types.KindDelegationProfile:
		return trace.BadParameter("kind: must be %s", types.KindDelegationProfile)
	case p.GetVersion() != types.V1:
		return trace.BadParameter("version: must be %s", types.V1)
	case p.GetMetadata().GetName() == "":
		return trace.BadParameter("metadata.name: is required")
	}

	if len(p.GetSpec().GetRequiredResources()) == 0 {
		return trace.BadParameter("spec.required_resources: at least one resource is required")
	}

	for idx, spec := range p.GetSpec().GetRequiredResources() {
		if err := ValidateDelegationResourceSpec(spec); err != nil {
			return trace.BadParameter("spec.required_resources[%d]: invalid resource spec: %v", idx, err)
		}
	}

	if len(p.GetSpec().GetAuthorizedUsers()) == 0 {
		return trace.BadParameter("spec.authorized_users: at least one user is required")
	}

	for idx, user := range p.GetSpec().GetAuthorizedUsers() {
		if user.GetType() != types.DelegationUserTypeBot {
			return trace.BadParameter("spec.authorized_users[%d].type: must be %s", idx, types.DelegationUserTypeBot)
		}
		if user.GetBotName() == "" {
			return trace.BadParameter("spec.authorized_users[%d].bot_name: is required", idx)
		}
	}

	if p.GetSpec().GetDefaultSessionLength().AsDuration() < 0 {
		return trace.BadParameter("spec.default_session_length: must be non-negative")
	}

	for idx, urlStr := range p.GetSpec().GetConsent().GetAllowedRedirectUrls() {
		if u, err := url.Parse(urlStr); err != nil || u.Scheme == "" || u.Host == "" {
			return trace.BadParameter("spec.consent.allowed_redirect_urls[%d]: invalid URL", idx)
		}
	}

	return nil
}

// ValidateDelegationResourceSpec validates a DelegationResourceSpec object.
func ValidateDelegationResourceSpec(s *delegationv1.DelegationResourceSpec) error {
	if s.GetName() == "" {
		return trace.BadParameter("name is required")
	}

	switch s.GetKind() {
	case types.KindApp:
		switch s.GetConstraints().(type) {
		case nil:
		case *delegationv1.DelegationResourceSpec_Mcp:
			// TODO(boxofrad): Remove this (and actually validate tool names)
			// once MCP constraints are supported by the RBAC system.
			return trace.BadParameter("mcp constraints are not yet supported")
		default:
			return trace.BadParameter("app resource may only have mcp constraints")
		}
	case types.KindDatabase:
		switch s.GetConstraints().(type) {
		case nil:
		case *delegationv1.DelegationResourceSpec_Db:
			// TODO(boxofrad): Remove this (and actually validate database/users)
			// once database constraints are supported by the RBAC system.
			return trace.BadParameter("db constraints are not yet supported")
		default:
			return trace.BadParameter("db resource may only have db constraints")
		}
	case types.KindNode:
		switch s.GetConstraints().(type) {
		case nil:
		case *delegationv1.DelegationResourceSpec_Ssh:
			// TODO(boxofrad): Remove this (and actually validate SSH users)
			// once node constraints are supported by the RBAC system.
			return trace.BadParameter("ssh constraints are not yet supported")
		default:
			return trace.BadParameter("node resource may only have ssh constraints")
		}
	case types.KindKubernetesCluster:
		switch c := s.GetConstraints().(type) {
		case nil:
		case *delegationv1.DelegationResourceSpec_Kubernetes:
			for idx, resource := range c.Kubernetes.GetResources() {
				if resource.GetKind() == "" {
					return trace.BadParameter("kubernetes.resources[%d].kind is required", idx)
				}
				if resource.GetName() == "" {
					return trace.BadParameter("kubernetes.resources[%d].name is required", idx)
				}
			}
		default:
			return trace.BadParameter("kube_cluster resource may only have kubernetes constraints")
		}
	case "":
		return trace.BadParameter("kind is required")
	default:
		return trace.BadParameter("invalid kind: %q", s.GetKind())
	}

	return nil
}
