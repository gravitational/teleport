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

package accesslists

import (
	"context"
	"slices"
	"strconv"

	"github.com/gravitational/trace"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/scopes"
)

// ScopedCollection represents a complete batch of access list and members from
// one source (e.g. entraID or okta). It is a clone of [Collection] that
// supports inclusion of scoped access lists and nested scoped access list
// members.
//
// All access lists and members in the batch are expected to be created/updated
// together as a snapshot of a final complete state of the access lists and
// their member hierarchy.
//
// For instance, imagine an access list import from EntraID directory where the
// access list can't be modified outside EntraID and teleport only fetches the
// complete state of access lists and members from EntraID.
// This is useful to validate nested hierarchy on the fly and preset the
// reference updates (Status.MemberOf and Status.OwnerOf) before starting the
// actual creation/update process in the backend.
// Relationships with access lists outside the ScopedCollection is not possible
// because the collection source is aware only of the access lists and members
// they contain.
type ScopedCollection struct {
	// MembersByAccessList maps access list names to their members
	MembersByAccessList map[NormalizedSQN][]*accesslist.AccessListMember
	// AccessListsByName is an internal map for fast lookup by name
	AccessListsByName map[NormalizedSQN]*accesslist.AccessList
}

// Validate validates all access lists and members in the batch.
func (b *ScopedCollection) Validate(ctx context.Context) error {
	for aclName, accessList := range b.AccessListsByName {
		if err := accessList.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		if ScopeQualifiedName(accessList) != aclName {
			return trace.BadParameter("AccessListsByName key %s does not match actual access list name %s",
				aclName.String(), ScopeQualifiedName(accessList))
		}
	}
	for aclName, members := range b.MembersByAccessList {
		if _, ok := b.AccessListsByName[aclName]; !ok {
			return trace.BadParameter("MembersByAccessList key %s has no corresponding list in AccessListsByName", aclName.String())
		}
		for _, member := range members {
			if err := member.CheckAndSetDefaults(); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	for _, accessList := range b.AccessListsByName {
		members := b.MembersByAccessList[ScopeQualifiedName(accessList)]
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
func (b *ScopedCollection) AddAccessList(accessList *accesslist.AccessList, members []*accesslist.AccessListMember) error {
	if accessList == nil {
		return trace.BadParameter("access list is nil")
	}
	if b.MembersByAccessList == nil {
		b.MembersByAccessList = make(map[NormalizedSQN][]*accesslist.AccessListMember)
	}
	if b.AccessListsByName == nil {
		b.AccessListsByName = make(map[NormalizedSQN]*accesslist.AccessList)
	}
	b.MembersByAccessList[ScopeQualifiedName(accessList)] = members
	b.AccessListsByName[ScopeQualifiedName(accessList)] = accessList

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
func (b *ScopedCollection) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	al, exists := b.AccessListsByName[NormalizedSQN{Name: name}]
	if !exists {
		return nil, trace.NotFound("access list %q not found in batch", name)
	}
	return al, nil
}

// GetAccessListV2 retrieves an access list from the batch by scoped name.
// Implements accesslists.AccessListAndMembersGetter interface.
func (b *ScopedCollection) GetAccessListV2(ctx context.Context, req *accesslistv1.GetAccessListRequest) (*accesslist.AccessList, error) {
	listName := NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
	al, exists := b.AccessListsByName[listName]
	if !exists {
		return nil, trace.NotFound("access list %q not found in batch", listName.String())
	}
	return al, nil
}

// ListAccessListMembers retrieves members for an access list from the batch.
// Implements accesslists.AccessListAndMembersGetter interface.
func (b *ScopedCollection) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	return b.ListAccessListMembersV2(ctx, accesslistv1.ListAccessListMembersRequest_builder{
		AccessList: accessListName,
		PageSize:   int32(pageSize),
		PageToken:  pageToken,
	}.Build())
}

// ListAccessListMembersV2 retrieves members for an access list from the batch.
func (b *ScopedCollection) ListAccessListMembersV2(ctx context.Context, req *accesslistv1.ListAccessListMembersRequest) ([]*accesslist.AccessListMember, string, error) {
	listName := NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	members, exists := b.MembersByAccessList[listName]
	if !exists {
		return nil, "", trace.NotFound("access list %q not found in batch", listName.String())
	}

	// Handle pagination
	startIdx := 0
	if req.GetPageToken() != "" {
		idx, err := strconv.Atoi(req.GetPageToken())
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
	if req.GetPageSize() > 0 {
		endIdx = min(startIdx+int(req.GetPageSize()), len(members))
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
func (b *ScopedCollection) GetAccessListMember(ctx context.Context, accessListName, memberName string) (*accesslist.AccessListMember, error) {
	return b.GetAccessListMemberV2(ctx, accesslistv1.GetAccessListMemberRequest_builder{
		AccessList: accessListName,
		MemberName: memberName,
	}.Build())
}

// GetAccessListMemberV2 retrieves a specific member from an access list in the batch.
func (b *ScopedCollection) GetAccessListMemberV2(ctx context.Context, req *accesslistv1.GetAccessListMemberRequest) (*accesslist.AccessListMember, error) {
	listName := NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetAccessListScope(),
		Name:  req.GetAccessList(),
	})
	memberName := NormalizeSQN(scopes.QualifiedName{
		Scope: req.GetMemberScope(),
		Name:  req.GetMemberName(),
	})
	members, exists := b.MembersByAccessList[listName]
	if !exists {
		return nil, trace.NotFound("access list %q not found in batch", listName.String())
	}
	for _, member := range members {
		thisMemberName, err := MemberScopeQualifiedName(member)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if thisMemberName == memberName {
			return member, nil
		}
	}
	return nil, trace.NotFound("member %q not found in access list %q", memberName.String(), listName.String())
}

// RefUpdates calculates and applies reference updates (Status.MemberOf and Status.OwnerOf)
// based on the nested access list relationships in the collection.
func (b *ScopedCollection) RefUpdates() error {
	for name, accessList := range b.AccessListsByName {
		for _, owner := range accessList.Spec.Owners {
			if !owner.IsMembershipKindList() {
				continue
			}
			ownerName, err := OwnerScopeQualifiedName(owner)
			if err != nil {
				return trace.Wrap(err)
			}
			if targetList, exists := b.AccessListsByName[ownerName]; exists {
				target := &targetList.Status.OwnerOf
				if name.Scope != "" {
					target = &targetList.Status.ScopedOwnerOf
				}
				if slices.Contains(*target, name.String()) {
					continue
				}
				*target = append(*target, name.String())
			}
		}
	}

	for accessListName, members := range b.MembersByAccessList {
		for _, member := range members {
			if !member.IsList() {
				continue
			}
			memberName, err := MemberScopeQualifiedName(member)
			if err != nil {
				return trace.Wrap(err)
			}
			if targetList, exists := b.AccessListsByName[memberName]; exists {
				target := &targetList.Status.MemberOf
				if accessListName.Scope != "" {
					target = &targetList.Status.ScopedMemberOf
				}
				if slices.Contains(*target, accessListName.String()) {
					continue
				}
				*target = append(*target, accessListName.String())
			}
		}
	}
	return nil
}
