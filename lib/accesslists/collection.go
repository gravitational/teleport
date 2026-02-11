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

package accesslists

import (
	"context"
	"slices"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accesslist"
)

// Collection represents a complete batch of access from one source (e.g. entraID or okta)
// That implies that all access lists and members in the batch are expected to be created/updated together
// because snapshot have a final complete state of the access lists and their members hierarchy.
// For instance imagine an access list import from EntraID directory where the access list
// can't be modified outside EntraID and teleport only fetches the complete state of access lists and members from EntraID
// This is useful to validate nested hierarchy on the fly and preset the reference updates (Status.MemberOf and Status.OwnerOf)
// before starting the actual creation/update process in the backend.
// Where the relation between access list outside the Collection is not possible because collection source are
// aware only of the access lists and members they contain.
type Collection struct {
	// MembersByAccessList maps access list names to their members
	MembersByAccessList map[string][]*accesslist.AccessListMember
	// AccessListsByName is an internal map for fast lookup by name
	AccessListsByName map[string]*accesslist.AccessList
}

// Validate validates all access lists and members in the batch.
func (b *Collection) Validate(ctx context.Context) error {
	for _, accessList := range b.AccessListsByName {
		if err := accessList.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, members := range b.MembersByAccessList {
		for _, member := range members {
			if err := member.CheckAndSetDefaults(); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	for _, accessList := range b.AccessListsByName {
		members := b.MembersByAccessList[accessList.GetName()]
		if err := ValidateAccessListWithMembers(ctx, nil, accessList, members, b); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := b.RefUpdates(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AddAccessList adds an access list and its members to the batch.
func (b *Collection) AddAccessList(accessList *accesslist.AccessList, members []*accesslist.AccessListMember) error {
	if accessList == nil {
		return trace.BadParameter("access list is nil")
	}
	if b.MembersByAccessList == nil {
		b.MembersByAccessList = make(map[string][]*accesslist.AccessListMember)
	}
	if b.AccessListsByName == nil {
		b.AccessListsByName = make(map[string]*accesslist.AccessList)
	}
	b.MembersByAccessList[accessList.GetName()] = members
	b.AccessListsByName[accessList.GetName()] = accessList

	if accessList.Status.MemberOf == nil {
		accessList.Status.MemberOf = []string{}
	}
	if accessList.Status.OwnerOf == nil {
		accessList.Status.OwnerOf = []string{}
	}
	return nil
}

// GetAccessList retrieves an access list from the batch by name.
// Implements accesslists.AccessListAndMembersGetter interface.
func (b *Collection) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	al, exists := b.AccessListsByName[name]
	if !exists {
		return nil, trace.NotFound("access list %q not found in batch", name)
	}
	return al, nil
}

// ListAccessListMembers retrieves members for an access list from the batch.
// Implements accesslists.AccessListAndMembersGetter interface.
func (b *Collection) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	members, exists := b.MembersByAccessList[accessListName]
	if !exists {
		return nil, "", trace.NotFound("access list %q not found in batch", accessListName)
	}

	// Handle pagination
	startIdx := 0
	if pageToken != "" {
		idx, err := strconv.Atoi(pageToken)
		if err != nil {
			return nil, "", trace.BadParameter("invalid page token: %v", err)
		}
		startIdx = idx
	}

	// If startIdx is beyond the members slice, return empty result
	if startIdx >= len(members) {
		return []*accesslist.AccessListMember{}, "", nil
	}

	// Calculate end index based on pageSize
	endIdx := len(members)
	if pageSize > 0 {
		endIdx = startIdx + pageSize
		if endIdx > len(members) {
			endIdx = len(members)
		}
	}

	// Slice members for this page
	pageMembers := members[startIdx:endIdx]

	// Generate next page token if there are more members
	nextToken := ""
	if endIdx < len(members) {
		nextToken = strconv.Itoa(endIdx)
	}

	return pageMembers, nextToken, nil
}

// GetAccessListMember retrieves a specific member from an access list in the batch.
// Implements accesslists.AccessListAndMembersGetter interface.
func (b *Collection) GetAccessListMember(ctx context.Context, accessListName, memberName string) (*accesslist.AccessListMember, error) {
	members, exists := b.MembersByAccessList[accessListName]
	if !exists {
		return nil, trace.NotFound("access list %q not found in batch", accessListName)
	}
	for _, member := range members {
		if member.GetName() == memberName {
			return member, nil
		}
	}
	return nil, trace.NotFound("member %q not found in access list %q", memberName, accessListName)
}

// RefUpdates calculates and applies reference updates (Status.MemberOf and Status.OwnerOf)
// based on the nested access list relationships in the collection.
func (b *Collection) RefUpdates() error {
	for name, accessList := range b.AccessListsByName {
		for _, owner := range accessList.Spec.Owners {
			if owner.MembershipKind != accesslist.MembershipKindList {
				continue
			}
			if targetList, exists := b.AccessListsByName[owner.Name]; exists {
				if slices.Contains(targetList.Status.OwnerOf, name) {
					continue
				}
				targetList.Status.OwnerOf = append(targetList.Status.OwnerOf, name)
			}
		}
	}

	for accessListName, members := range b.MembersByAccessList {
		for _, member := range members {
			if member.Spec.MembershipKind != accesslist.MembershipKindList {
				continue
			}
			if targetList, exists := b.AccessListsByName[member.Spec.Name]; exists {
				if slices.Contains(targetList.Status.MemberOf, accessListName) {
					continue
				}
				targetList.Status.MemberOf = append(targetList.Status.MemberOf, accessListName)
			}
		}
	}
	return nil
}
