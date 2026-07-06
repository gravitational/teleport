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

package accesslist

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
)

func newMember(listName, name, kind string) (*accesslist.AccessListMember, error) {
	return accesslist.NewAccessListMember(
		header.Metadata{Name: name},
		accesslist.AccessListMemberSpec{
			AccessList:     listName,
			Name:           name,
			MembershipKind: kind,
		},
	)
}

func getReviewFrequency(months int) (accesslist.ReviewFrequency, error) {
	f := accesslist.ReviewFrequency(months)
	switch f {
	case accesslist.OneMonth, accesslist.ThreeMonths, accesslist.SixMonths, accesslist.OneYear:
		return f, nil
	}
	return 0, trace.BadParameter("--audit-frequency must be one of 1, 3, 6, 12 (got %d)", months)
}

func getReviewDayOfMonth(day int) (accesslist.ReviewDayOfMonth, error) {
	d := accesslist.ReviewDayOfMonth(day)
	switch d {
	case accesslist.FirstDayOfMonth, accesslist.FifteenthDayOfMonth, accesslist.LastDayOfMonth:
		return d, nil
	}
	return 0, trace.BadParameter("--audit-day must be one of 1, 15, 31 (got %d)", day)
}

// applyGrantsAndRequirements applies the owner/member grant and requirement flags to
// the access list spec.
func (c *Command) applyGrantsAndRequirements(al *accesslist.AccessList) error {
	// Owner grants and requirements
	if c.ownerGrantRolesSet {
		al.Spec.OwnerGrants.Roles = utils.SplitIdentifiers(c.ownerGrantRoles)
	}
	if c.ownerGrantTraitsSet {
		traits, err := parse.MultiValueLabelSelectorSpec(c.ownerGrantTraits)
		if err != nil {
			return trace.Wrap(err)
		}
		al.Spec.OwnerGrants.Traits = traits
	}
	if c.ownerRequiredRolesSet {
		al.Spec.OwnershipRequires.Roles = utils.SplitIdentifiers(c.ownerRequiredRoles)
	}
	if c.ownerRequiredTraitsSet {
		traits, err := parse.MultiValueLabelSelectorSpec(c.ownerRequiredTraits)
		if err != nil {
			return trace.Wrap(err)
		}
		al.Spec.OwnershipRequires.Traits = traits
	}

	// Member grants and requirements
	if c.memberGrantRolesSet {
		al.Spec.Grants.Roles = utils.SplitIdentifiers(c.memberGrantRoles)
	}
	if c.memberGrantTraitsSet {
		traits, err := parse.MultiValueLabelSelectorSpec(c.memberGrantTraits)
		if err != nil {
			return trace.Wrap(err)
		}
		al.Spec.Grants.Traits = traits
	}
	if c.memberRequiredRolesSet {
		al.Spec.MembershipRequires.Roles = utils.SplitIdentifiers(c.memberRequiredRoles)
	}
	if c.memberRequiredTraitsSet {
		traits, err := parse.MultiValueLabelSelectorSpec(c.memberRequiredTraits)
		if err != nil {
			return trace.Wrap(err)
		}
		al.Spec.MembershipRequires.Traits = traits
	}

	return nil
}
