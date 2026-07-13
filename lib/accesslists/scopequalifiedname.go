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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/scopes"
)

// NormalizedSQN is a scope-qualified name that has been normalized for
// comparisons and usage as a map key.
type NormalizedSQN struct {
	// Scope is the resource's normalized scope path, e.g. "/staging/west".
	Scope string
	// Name is the resource's name within its scope, e.g. "mylist".
	Name string
}

// NormalizeSQN returns a scope-qualified name that has been normalized for
// comparisons and usage as a map key.
func NormalizeSQN(sqn scopes.QualifiedName) NormalizedSQN {
	return NormalizedSQN{
		Scope: scopes.NormalizeForEquality(sqn.Scope),
		Name:  sqn.Name,
	}
}

// ToScopesQualifiedName converts the NormalizedSQN to a [scopes.QualifiedName].
func (n NormalizedSQN) ToScopesQualifiedName() scopes.QualifiedName {
	return scopes.QualifiedName{
		Scope: n.Scope,
		Name:  n.Name,
	}
}

// String formates the NormalizedSQN as a string.
func (n NormalizedSQN) String() string {
	return n.ToScopesQualifiedName().String()
}

// ParseScopeQualifiedName parses the given scope-qualified name string,
// applies weak validation, and normalizes it.
func ParseScopeQualifiedName(str string) (NormalizedSQN, error) {
	sqn, err := scopes.ParseQualifiedName(str)
	if err != nil {
		return NormalizedSQN{}, trace.Wrap(err)
	}
	if err := sqn.WeakValidate(); err != nil {
		return NormalizedSQN{}, trace.Wrap(err)
	}
	return NormalizeSQN(sqn), nil
}

// ScopeQualifiedName returns the normalized scope-qualified name of the given access list.
func ScopeQualifiedName(list *accesslist.AccessList) NormalizedSQN {
	return NormalizeSQN(scopes.QualifiedName{
		Scope: list.Scope,
		Name:  list.Metadata.Name,
	})
}

// OwnerScopeQualifiedName returns the scope-qualified name of an access list owner.
func OwnerScopeQualifiedName(owner accesslist.Owner) (NormalizedSQN, error) {
	switch owner.MembershipKind {
	case accesslist.MembershipKindUser, accesslist.MembershipKindList, accesslist.MembershipKindUnspecified, "":
		return NormalizedSQN{
			Name: owner.Name,
		}, nil
	case accesslist.MembershipKindScopedList:
		return ParseScopeQualifiedName(owner.Name)
	default:
		return NormalizedSQN{}, trace.BadParameter("unhandled membership kind %s", owner.MembershipKind)
	}
}

// MemberScopeQualifiedName returns the scope-qualified name of an access list member.
func MemberScopeQualifiedName(member *accesslist.AccessListMember) (NormalizedSQN, error) {
	switch member.Spec.MembershipKind {
	case accesslist.MembershipKindUser, accesslist.MembershipKindList, accesslist.MembershipKindUnspecified, "":
		return NormalizedSQN{
			Name: member.GetName(),
		}, nil
	case accesslist.MembershipKindScopedList:
		return ParseScopeQualifiedName(member.GetName())
	default:
		return NormalizedSQN{}, trace.BadParameter("unhandled membership kind %s", member.Spec.MembershipKind)
	}
}

// AllOwnedLists returns the scope-qualified names of all access lists owned by
// the given list, as recorded in Status.OwnerOf and Status.ScopedOwnerOf.
func AllOwnedLists(list *accesslist.AccessList) ([]NormalizedSQN, error) {
	ownedLists := make([]NormalizedSQN, 0, len(list.Status.OwnerOf)+len(list.Status.ScopedOwnerOf))
	for _, ownerOf := range list.Status.OwnerOf {
		ownedLists = append(ownedLists, NormalizedSQN{Name: ownerOf})
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
func AllParentLists(list *accesslist.AccessList) ([]NormalizedSQN, error) {
	parentLists := make([]NormalizedSQN, 0, len(list.Status.MemberOf)+len(list.Status.ScopedMemberOf))
	for _, memberOf := range list.Status.MemberOf {
		parentLists = append(parentLists, NormalizedSQN{Name: memberOf})
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
