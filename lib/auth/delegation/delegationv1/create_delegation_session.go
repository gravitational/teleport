/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package delegationv1

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/accessrequest"
	"github.com/gravitational/teleport/api/client/proto"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
	"github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateDelegationSession creates a delegation session.
//
// TODO(boxofrad):
//   - Validate Redirect URL
//   - Decide whether we should treat NotFound errors when reading the profile
//     as AccessDenied
func (s *SessionService) CreateDelegationSession(
	ctx context.Context,
	req *delegationv1.CreateDelegationSessionRequest,
) (*delegationv1.DelegationSession, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This is a security-sensitive action, so require MFA.
	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	var ttl time.Duration
	if t := req.GetTtl(); t != nil {
		ttl = t.AsDuration()
	}

	var (
		resources       []*delegationv1.DelegationResourceSpec
		authorizedUsers []*delegationv1.DelegationUserSpec
	)
	switch from := req.From.(type) {
	case *delegationv1.CreateDelegationSessionRequest_Parameters:
		resources = from.Parameters.GetResources()
		authorizedUsers = from.Parameters.GetAuthorizedUsers()
	case *delegationv1.CreateDelegationSessionRequest_Profile:
		profile, err := s.profileReader.GetDelegationProfile(ctx, from.Profile.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if profile.GetMetadata().GetRevision() != from.Profile.GetRevision() {
			return nil, trace.CompareFailed("profile.revision: does not match the profile's current revision")
		}
		if err := authCtx.Checker.CheckAccess(
			types.Resource153ToResourceWithLabels(profile),
			services.AccessState{MFAVerified: true},
		); err != nil {
			return nil, trace.Wrap(err)
		}

		resources = profile.GetSpec().GetRequiredResources()
		authorizedUsers = profile.GetSpec().GetAuthorizedUsers()

		// If the caller did not provide a TTL, take the default session length.
		if ttl == 0 {
			if sl := profile.GetSpec().GetDefaultSessionLength(); sl != nil {
				ttl = sl.AsDuration()
			}
		}
	default:
		return nil, trace.NotImplemented("from: unsupported type %T`", from)
	}

	switch {
	case ttl == 0:
		return nil, trace.BadParameter("ttl is required")
	case len(resources) == 0:
		return nil, trace.BadParameter("at least one resource is required")
	case len(authorizedUsers) == 0:
		return nil, trace.BadParameter("at least one authorized user is required")
	}

	// Read user from the backend to get the current roles and traits.
	user, err := s.userGetter.GetUser(ctx, authCtx.User.GetName(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.bestEffortCheckResourceAccess(ctx, user, resources); err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := s.sessionWriter.CreateDelegationSession(ctx, &delegationv1.DelegationSession{
		Kind:    types.KindDelegationSession,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    uuid.NewString(),
			Expires: timestamppb.New(time.Now().Add(ttl).UTC()),
		},
		Spec: &delegationv1.DelegationSessionSpec{
			User:            authCtx.User.GetName(),
			Resources:       resources,
			AuthorizedUsers: authorizedUsers,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}

// bestEffortCheckResourceAccess makes a best-effort attempt to check a user's
// access to the given resources. It is not strictly required as the RBAC engine
// will check resource access at time-of-use, but enables us to surface permission
// errors as early as possible (while the user is still "in the loop").
func (s *SessionService) bestEffortCheckResourceAccess(
	ctx context.Context,
	user types.User,
	resources []*delegationv1.DelegationResourceSpec,
) error {
	checker, err := services.NewAccessChecker(
		&services.AccessInfo{
			Roles:  user.GetRoles(),
			Traits: user.GetTraits(),
		},
		"",
		s.roleGetter,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	resourceNamesByKind := make(map[string]set.Set[string])
	for _, res := range resources {
		byKind, ok := resourceNamesByKind[res.GetKind()]
		if !ok {
			byKind = set.New[string]()
			resourceNamesByKind[res.GetKind()] = byKind
		}
		byKind.Add(res.GetName())
	}

	resourcesByKindName := make(map[string]map[string]types.ResourceWithLabels)
	for kind, resourceNames := range resourceNamesByKind {
		req := proto.ListResourcesRequest{
			PredicateExpression: strings.Join(
				slices.Map(
					resourceNames.Elements(),
					func(name string) string {
						return fmt.Sprintf(`resource.metadata.name == %q`, name)
					},
				),
				" || ",
			),
			Limit: int32(len(resourceNames)),
		}

		rsp, err := accessrequest.GetResourcesByKind(ctx, s.resourceLister, req, kind)
		if err != nil {
			return trace.Wrap(err)
		}

		byName := make(map[string]types.ResourceWithLabels)
		for _, res := range rsp {
			byName[res.GetName()] = res
		}
		resourcesByKindName[kind] = byName
	}

	unauthorizedResources := set.New[string]()
	for _, spec := range resources {
		id := fmt.Sprintf("%s/%s", spec.GetKind(), spec.GetName())

		byName, ok := resourcesByKindName[spec.GetKind()]
		if !ok {
			unauthorizedResources.Add(id)
			continue
		}

		res, ok := byName[spec.GetName()]
		if !ok {
			unauthorizedResources.Add(id)
			continue
		}

		if err := checker.CheckAccess(res, services.AccessState{MFAVerified: true}); err != nil {
			unauthorizedResources.Add(id)
			continue
		}
	}

	if unauthorizedResources.Len() != 0 {
		idStrings := unauthorizedResources.Elements()
		sort.Strings(idStrings)
		return trace.AccessDenied("You do not have permission to delegate access to all of the required resources, missing resources: [%s]", strings.Join(idStrings, ", "))
	}

	return nil
}
