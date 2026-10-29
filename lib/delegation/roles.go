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

package delegation

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/utils/set"
)

// AllDelegatableRoles filters the given role set down to only roles that may be
// delegated, by applying `allow.delegation.roles` and `deny.delegation.roles`.
//
// If any roles are unsafe to "drop" (i.e. they contain `deny` conditions), but
// the user is not permitted to delegate them, this method will return an error
// as there is no safe way to proceed.
//
// You should call this method with the user's *standing* roles, not temporary
// roles assigned by access requests or access lists, to prevent them from using
// delegation to extend their access beyond the allowed time.
func AllDelegatableRoles(roleSet []types.Role) ([]types.Role, error) {
	var (
		allowStatements      = set.New[string]()
		denyStatements       = set.New[string]()
		undroppableRoleNames = set.New[string]()
		rolesByName          = make(map[string]types.Role)
	)
	for _, role := range roleSet {
		rolesByName[role.GetName()] = role

		allowStatements.Add(role.GetDelegationRoles(types.Allow)...)
		denyStatements.Add(role.GetDelegationRoles(types.Deny)...)

		if roleIsUndroppable(role) {
			undroppableRoleNames.Add(role.GetName())
		}
	}

	// Start with all of the roles the user holds.
	allowedRoleNames := roleNames(roleSet)

	// Filter it down to any explicitly allowed roles, unless there's a wildcard.
	if !allowStatements.Contains(types.Wildcard) {
		allowedRoleNames.Intersection(allowStatements)
	}

	// If there's a deny wildcard, empty the list entirely. Otherwise, filter
	// out explicitly denied roles.
	if denyStatements.Contains(types.Wildcard) {
		allowedRoleNames = set.New[string]()
	} else {
		allowedRoleNames.Subtract(denyStatements)
	}

	// Check if there are any undroppable roles that the user is not allowed to
	// delegate. This should be a fairly unusual edge-case, the user themselves
	// cannot fix it and must speak to their cluster administrator to fix their
	// permissions.
	unsafeDrops := undroppableRoleNames.Subtract(allowedRoleNames).Elements()
	if len(unsafeDrops) > 0 {
		sort.Strings(unsafeDrops)
		return nil, trace.AccessDenied(
			"You are assigned the following roles that must be delegated, but you do not have permission to do so: [%s]",
			strings.Join(unsafeDrops, ", "),
		)
	}

	// Collect them back up into a RoleSet.
	finalRoleSet := make([]types.Role, len(allowedRoleNames))
	for idx, roleName := range allowedRoleNames.Elements() {
		finalRoleSet[idx] = rolesByName[roleName]
	}
	return finalRoleSet, nil
}

// CheckDelegatedRoles checks whether the requested roles may be delegated by
// applying `allow.delegation.roles` and `deny.delegation.roles` - and ensuring
// any "undroppable" roles (i.e. those containing `deny` conditions) are included.
//
// You should call this method with the user's *standing* roles, not temporary
// roles assigned by access requests or access lists, to prevent them from using
// delegation to extend their access beyond the allowed time.
func CheckDelegatedRoles(roleSet []types.Role, delegatedRoleNames []string) error {
	roles, err := AllDelegatableRoles(roleSet)
	if err != nil {
		return trace.Wrap(err)
	}

	requested := set.New(delegatedRoleNames...)
	allowed := roleNames(roles)

	// Check if there are any undroppable roles that the user isn't delegating.
	var unsafeDrops []string
	for _, role := range roles {
		if requested.Contains(role.GetName()) {
			continue
		}
		if roleIsUndroppable(role) {
			unsafeDrops = append(unsafeDrops, role.GetName())
		}
	}
	if len(unsafeDrops) > 0 {
		// We have already checked the user is actually allowed to delegate these
		// roles in AllDelegatableRoles.
		sort.Strings(unsafeDrops)
		return trace.AccessDenied(
			"You must delegate the following roles: [%s]",
			strings.Join(unsafeDrops, ", "),
		)
	}

	missing := requested.Subtract(allowed).Elements()
	if len(missing) > 0 {
		sort.Strings(missing)
		return trace.AccessDenied(
			"You do not have permission to delegate the following roles: [%s]",
			strings.Join(missing, ", "),
		)
	}

	return nil
}

// roleIsUndroppable determines whether the role is unsafe to drop because it
// contains any `deny` conditions.
func roleIsUndroppable(role types.Role) bool {
	return !role.GetRoleConditions(types.Deny).IsEmpty()
}

func roleNames(roles []types.Role) set.Set[string] {
	names := set.New[string]()
	for _, role := range roles {
		names.Add(role.GetName())
	}
	return names
}

// BuildRolesForAccessRequests generates a set of roles that would allow the
// delegate to search for, and request access to, resources the delegator can
// access with their standing roles (or request access to via search_as_roles).
//
// Roles with equivalent request.options (e.g. max_duration, kubernetes_resources)
// are merged into a single role with with search_as_roles being the union of
// their respective sets. Roles with different request.options are kept separate
// to ensure we do not lose their conflicting options, thereby creating a vector
// for privilege escalation.
//
// delegatedRoleNames contains the names of roles that are being directly assigned
// to the delegate. These roles do not need to be represented in the generated set.
//
// Generated roles will also not include any deny.request conditions, as it is
// not possible to "drop" a role with deny conditions through delegation.
func BuildRolesForAccessRequests(
	nameInfix string,
	standingRoles []types.Role,
	delegatedRoleNames []string,
) ([]types.Role, error) {
	delegatedRoleNameSet := set.New(delegatedRoleNames...)

	// Bucket roles by their matching options.
	//
	// Create a default bucket where the user's undelegated standing roles are
	// shared. We also merge into it any "unqualified" search_as_roles (i.e.
	// those without options such as max_duration or kubernetes_resources).
	defaultBucket := newAccessRequestConditionBucket(nil /* role */)
	buckets := []*accessRequestConditionBucket{defaultBucket}

	for _, role := range standingRoles {
		// This role is delegated, no need to include in derived roles.
		if delegatedRoleNameSet.Contains(role.GetName()) {
			continue
		}
		defaultBucket.combinedSearchAsRoles.Add(role.GetName())
		defaultBucket.sourceRoles.Add(role.GetName())

		searchAsRoles := role.GetSearchAsRoles(types.Allow)

		// This role has no search_as_roles, no need to include in derived roles.
		if len(searchAsRoles) == 0 {
			continue
		}

		// Find a bucket that matches this role's conditions.
		var matchingBucket *accessRequestConditionBucket
		for _, bucket := range buckets {
			if bucket.matches(role) {
				matchingBucket = bucket
				break
			}
		}
		if matchingBucket == nil {
			matchingBucket = newAccessRequestConditionBucket(role)
			buckets = append(buckets, matchingBucket)
		}
		matchingBucket.combinedSearchAsRoles.Add(searchAsRoles...)
		matchingBucket.sourceRoles.Add(role.GetName())
	}

	// Create derived roles from buckets.
	var roles []types.Role
	for idx, bucket := range buckets {
		if bucket.combinedSearchAsRoles.Len() == 0 {
			continue
		}

		role, err := types.NewRole(
			fmt.Sprintf("delegation-%s-%d", nameInfix, idx),
			types.RoleSpecV6{},
		)
		if err != nil {
			return nil, trace.Wrap(err, "creating generated role")
		}

		searchAsRoles := bucket.combinedSearchAsRoles.Elements()
		sort.Strings(searchAsRoles)

		role.SetAccessRequestConditions(types.Allow, types.AccessRequestConditions{
			SearchAsRoles:       searchAsRoles,
			Annotations:         bucket.annotations,
			Thresholds:          bucket.thresholds,
			MaxDuration:         bucket.maxDuration,
			KubernetesResources: bucket.kubernetesResources,
			Reason:              bucket.reason,
		})

		sourceRoles := bucket.sourceRoles.Elements()
		sort.Strings(sourceRoles)

		meta := role.GetMetadata()
		meta.Description = fmt.Sprintf("Automatically generated role to enable delegated access requests. Derived from roles: [%s].", strings.Join(sourceRoles, ", "))
		role.SetMetadata(meta)
		roles = append(roles, role)
	}
	return roles, nil
}

func newAccessRequestConditionBucket(role types.Role) *accessRequestConditionBucket {
	bucket := &accessRequestConditionBucket{
		sourceRoles:           set.New[string](),
		combinedSearchAsRoles: set.New[string](),
	}

	if role != nil {
		conds := role.GetAccessRequestConditions(types.Allow)
		bucket.annotations = conds.Annotations
		bucket.thresholds = conds.Thresholds
		bucket.maxDuration = conds.MaxDuration
		bucket.kubernetesResources = conds.KubernetesResources
		bucket.reason = conds.Reason
	}
	return bucket
}

type accessRequestConditionBucket struct {
	annotations         wrappers.Traits
	thresholds          []types.AccessReviewThreshold
	maxDuration         types.Duration
	kubernetesResources []types.RequestKubernetesResource
	reason              *types.AccessRequestConditionsReason

	sourceRoles           set.Set[string]
	combinedSearchAsRoles set.Set[string]
}

func (bucket *accessRequestConditionBucket) matches(role types.Role) bool {
	conds := role.GetAccessRequestConditions(types.Allow)

	annotationsMatch := maps.EqualFunc(
		conds.Annotations,
		bucket.annotations,
		slices.Equal,
	)

	thresholdsMatch := slices.EqualFunc(
		conds.Thresholds,
		bucket.thresholds,
		func(a, b types.AccessReviewThreshold) bool {
			return a.IsEqual(&b)
		},
	)

	kubernetesResourcesMatch := slices.EqualFunc(
		conds.KubernetesResources,
		bucket.kubernetesResources,
		func(a, b types.RequestKubernetesResource) bool {
			return a.IsEqual(&b)
		},
	)

	return annotationsMatch &&
		thresholdsMatch &&
		kubernetesResourcesMatch &&
		conds.MaxDuration == bucket.maxDuration &&
		conds.Reason.IsEqual(bucket.reason)
}
