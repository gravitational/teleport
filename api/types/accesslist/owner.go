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

package accesslist

import (
	"slices"
	"strings"

	"github.com/gravitational/trace"
)

// Owner is an owner of an access list.
type Owner struct {
	// Name is the username of the owner.
	Name string `json:"name" yaml:"name"`

	// Description is the plaintext description of the owner and why they are an owner.
	Description string `json:"description" yaml:"description"`

	// IneligibleStatus describes the reason why this owner is not eligible.
	IneligibleStatus string `json:"ineligible_status" yaml:"ineligible_status"`

	// MembershipKind describes the kind of ownership,
	// either "MEMBERSHIP_KIND_USER" or "MEMBERSHIP_KIND_LIST".
	MembershipKind string `json:"membership_kind" yaml:"membership_kind"`
}

// SetDefaults sets the default values for the resource.
func (o *Owner) SetDefaults() error {
	if o.MembershipKind == "" {
		o.MembershipKind = MembershipKindUser
	}
	return nil
}

// Validate performs client-side resource validation which should be performed before sending
// create/update request.
func (o *Owner) Validate() error {
	if o.Name == "" {
		return trace.BadParameter("name is missing")
	}
	return nil
}

// deduplicateOwners returns a new slice with owners with the same name removed. The order is not
// guaranteed. deduplicateOwners zeroes the elements between the new length and the original
// length.
func deduplicateOwners(owners []Owner) []Owner {
	slices.SortFunc(owners, func(x, y Owner) int { return strings.Compare(x.Name, y.Name) })
	return slices.CompactFunc(owners, func(x, y Owner) bool { return x.Name == y.Name })
}
