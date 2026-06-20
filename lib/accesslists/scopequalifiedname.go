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
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/trace"
)

// ToScopesQualifiedName converts an [accesslist.ScopeQualifiedName] to a [scopes.QualifiedName].
func ToScopesQualifiedName(sqn accesslist.ScopeQualifiedName) scopes.QualifiedName {
	return scopes.QualifiedName{
		Scope: sqn.Scope,
		Name:  sqn.Name,
	}
}

// FromScopesQualifiedName converts a [scopes.QualifiedName] to an [accesslist.ScopeQualifiedName].
func FromScopesQualifiedName(sqn scopes.QualifiedName) accesslist.ScopeQualifiedName {
	return accesslist.ScopeQualifiedName{
		Scope: sqn.Scope,
		Name:  sqn.Name,
	}
}

// ParseScopeQualifiedName parses a scope-qualified name into an [accesslist.ParseScopeQualifiedName].
func ParseScopeQualifiedName(name string) (accesslist.ScopeQualifiedName, error) {
	sqn, err := scopes.ParseQualifiedName(name)
	if err != nil {
		return accesslist.ScopeQualifiedName{}, trace.Wrap(err)
	}
	return FromScopesQualifiedName(sqn), nil
}

// ScopeQualifiedNameToString returns the string representation of the [accesslist.ScopeQualifiedName].
// If the Scope is empty, the Name is returned verbatim.
func ScopeQualifiedNameToString(sqn accesslist.ScopeQualifiedName) string {
	return ToScopesQualifiedName(sqn).String()
}

// OwnerScopeQualifiedName returns the scope-qualified name of an access list owner.
func OwnerScopeQualifiedName(owner accesslist.Owner) (accesslist.ScopeQualifiedName, error) {
	switch owner.MembershipKind {
	case accesslist.MembershipKindUser, accesslist.MembershipKindList, accesslist.MembershipKindUnspecified, "":
		return accesslist.ScopeQualifiedName{
			Name: owner.Name,
		}, nil
	case accesslist.MembershipKindScopedList:
		return ParseScopeQualifiedName(owner.Name)
	default:
		return accesslist.ScopeQualifiedName{}, trace.BadParameter("unhandled membership kind %s", owner.MembershipKind)
	}
}

// MemberScopeQualifiedName returns the scope-qualified name of an access list member.
func MemberScopeQualifiedName(member *accesslist.AccessListMember) (accesslist.ScopeQualifiedName, error) {
	switch member.Spec.MembershipKind {
	case accesslist.MembershipKindUser, accesslist.MembershipKindList, accesslist.MembershipKindUnspecified, "":
		return accesslist.ScopeQualifiedName{
			Name: member.GetName(),
		}, nil
	case accesslist.MembershipKindScopedList:
		return ParseScopeQualifiedName(member.GetName())
	default:
		return accesslist.ScopeQualifiedName{}, trace.BadParameter("unhandled membership kind %s", member.Spec.MembershipKind)
	}
}

// AllOwnedLists returns the scope-qualified names of all access lists owned by
// the given list, as recorded in Status.OwnerOf and Status.ScopedOwnerOf.
func AllOwnedLists(list *accesslist.AccessList) ([]accesslist.ScopeQualifiedName, error) {
	ownedLists := make([]accesslist.ScopeQualifiedName, 0, len(list.Status.OwnerOf)+len(list.Status.ScopedOwnerOf))
	for _, ownerOf := range list.Status.OwnerOf {
		ownedLists = append(ownedLists, accesslist.ScopeQualifiedName{Name: ownerOf})
	}
	for _, scopedOwnerOf := range list.Status.ScopedOwnerOf {
		ownedSQN, err := ParseScopeQualifiedName(scopedOwnerOf)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ownedLists = append(ownedLists, ownedSQN)
	}
	return ownedLists, nil
}

// AllParentLists returns the scope-qualified names of all direct parent access
// lists of the given list, as recorded in Status.MemberOf and Status.ScopedMemberOf.
func AllParentLists(list *accesslist.AccessList) ([]accesslist.ScopeQualifiedName, error) {
	parentLists := make([]accesslist.ScopeQualifiedName, 0, len(list.Status.MemberOf)+len(list.Status.ScopedMemberOf))
	for _, memberOf := range list.Status.MemberOf {
		parentLists = append(parentLists, accesslist.ScopeQualifiedName{Name: memberOf})
	}
	for _, scopedMemberOf := range list.Status.ScopedMemberOf {
		parentSQN, err := ParseScopeQualifiedName(scopedMemberOf)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		parentLists = append(parentLists, parentSQN)
	}
	return parentLists, nil
}
