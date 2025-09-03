/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package accesslists

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/trait"
)

// UserMeetsRequirements is a helper which will return whether the User meets the AccessList Ownership/MembershipRequires.
func UserMeetsRequirements(identity types.User, requires accesslist.Requires) bool {
	return newRequirementsEvaluator(identity).meets(requires)
}

// GetInheritedMembershipRequires returns the combined Requires for an Access List's members,
// inherited from any ancestor lists, and the Access List's own MembershipRequires.
func GetInheritedMembershipRequires(ctx context.Context, accessList *accesslist.AccessList, g AccessListAndMembersGetter) (*accesslist.Requires, error) {
	ownRequires := accessList.GetMembershipRequires()
	ancestors, err := GetAncestorsFor(ctx, accessList, RelationshipKindMember, g)
	if err != nil {
		return &ownRequires, trace.Wrap(err)
	}

	roles := ownRequires.Roles
	traits := ownRequires.Traits

	for _, ancestor := range ancestors {
		requires := ancestor.GetMembershipRequires()
		roles = append(roles, requires.Roles...)
		for traitKey, traitValues := range requires.Traits {
			if _, exists := traits[traitKey]; !exists {
				traits[traitKey] = []string{}
			}
			traits[traitKey] = append(traits[traitKey], traitValues...)
		}
	}

	slices.Sort(roles)
	roles = slices.Compact(roles)

	for k, v := range traits {
		slices.Sort(v)
		traits[k] = slices.Compact(v)
	}

	return &accesslist.Requires{
		Roles:  roles,
		Traits: traits,
	}, nil
}

// GetInheritedGrants returns the combined Grants for an Access List's members, inherited from any ancestor lists.
func GetInheritedGrants(ctx context.Context, accessList *accesslist.AccessList, g AccessListAndMembersGetter) (*accesslist.Grants, error) {
	grants := accesslist.Grants{
		Traits: trait.Traits{},
	}

	collectedRoles := make(map[string]struct{})
	collectedTraits := make(map[string]map[string]struct{})

	addGrants := func(grantRoles []string, grantTraits trait.Traits) {
		for _, role := range grantRoles {
			if _, exists := collectedRoles[role]; !exists {
				grants.Roles = append(grants.Roles, role)
				collectedRoles[role] = struct{}{}
			}
		}
		for traitKey, traitValues := range grantTraits {
			if _, exists := collectedTraits[traitKey]; !exists {
				collectedTraits[traitKey] = make(map[string]struct{})
			}
			for _, traitValue := range traitValues {
				if _, exists := collectedTraits[traitKey][traitValue]; !exists {
					grants.Traits[traitKey] = append(grants.Traits[traitKey], traitValue)
					collectedTraits[traitKey][traitValue] = struct{}{}
				}
			}
		}
	}

	// Get ancestors via member relationship
	ancestorLists, err := GetAncestorsFor(ctx, accessList, RelationshipKindMember, g)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ancestor := range ancestorLists {
		memberGrants := ancestor.GetGrants()
		addGrants(memberGrants.Roles, memberGrants.Traits)
	}

	// Get ancestors via owner relationship
	ancestorOwnerLists, err := GetAncestorsFor(ctx, accessList, RelationshipKindOwner, g)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, ancestorOwner := range ancestorOwnerLists {
		ownerGrants := ancestorOwner.GetOwnerGrants()
		addGrants(ownerGrants.Roles, ownerGrants.Traits)
	}

	slices.Sort(grants.Roles)
	grants.Roles = slices.Compact(grants.Roles)

	for k, v := range grants.Traits {
		slices.Sort(v)
		grants.Traits[k] = slices.Compact(v)
	}

	return &grants, nil
}

// requirementsEvaluator caches role/trait lookups for a single user to avoid rebuilding sets.
type requirementsEvaluator struct {
	rolesSet map[string]struct{}
	traits   map[string]map[string]struct{}
	user     types.User
}

func newRequirementsEvaluator(u types.User) *requirementsEvaluator {
	return &requirementsEvaluator{
		user: u,
	}
}

func (e *requirementsEvaluator) createLazyRoleLookupMap() {
	if e.rolesSet != nil {
		return
	}
	// Assemble the user's roles for easy look up.
	userRolesMap := map[string]struct{}{}
	for _, role := range e.user.GetRoles() {
		userRolesMap[role] = struct{}{}
	}
	e.rolesSet = userRolesMap
}

func (e *requirementsEvaluator) createLazyTraitsLookupMap() {
	if e.traits != nil {
		return
	}
	// Assemble traits for easy lookup.
	userTraitsMap := map[string]map[string]struct{}{}
	for k, values := range e.user.GetTraits() {
		if _, ok := userTraitsMap[k]; !ok {
			userTraitsMap[k] = map[string]struct{}{}
		}

		for _, v := range values {
			userTraitsMap[k][v] = struct{}{}
		}
	}
	e.traits = userTraitsMap
}

func (e *requirementsEvaluator) meets(requires accesslist.Requires) bool {
	if requires.IsEmpty() {
		// No requirements to meet return early to avoid unnecessary work.
		return true
	}
	if len(requires.Roles) > 0 {
		e.createLazyRoleLookupMap()
	}
	// Check that the user meets the role requirements.
	for _, role := range requires.Roles {
		if _, ok := e.rolesSet[role]; !ok {
			return false
		}
	}
	if len(requires.Traits) > 0 {
		e.createLazyTraitsLookupMap()
	}
	// Check that user meets trait requirements.
	for k, values := range requires.Traits {
		if _, ok := e.traits[k]; !ok {
			return false
		}

		for _, v := range values {
			if _, ok := e.traits[k][v]; !ok {
				return false
			}
		}
	}
	// The user meets all requirements.
	return true
}
