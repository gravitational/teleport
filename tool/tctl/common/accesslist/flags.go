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

// anyStandardResourceVisibilityFlagsSet returns true if any standard (non awsic)
// labels or github is set.
func (c *Command) anyStandardResourceVisibilityFlagsSet() bool {
	return c.nodeLabelsSet ||
		c.dbLabelsSet ||
		c.kubeLabelsSet ||
		c.appLabelsSet ||
		c.windowsLabelsSet ||
		c.gitHubOrgsSet
}

// anyStandardIdentityFlagsSet returns true if any access related to a user
// connecting "as" to a resource is set.
func (c *Command) anyStandardIdentityFlagsSet() bool {
	return c.loginsSet || c.awsRoleARNsSet || c.azureIdentitiesSet ||
		c.gcpServiceAccountsSet || c.mcpToolsSet || c.dbNamesSet || c.dbUsersSet ||
		c.kubeGroupsSet || c.kubeUsersSet || c.windowsLoginsSet
}

func (c *Command) anyStandardAccessFlagsSet() bool {
	return c.anyStandardResourceVisibilityFlagsSet() || c.anyStandardIdentityFlagsSet()
}

func (c *Command) anyAccessFlagsSet() bool {
	return c.anyStandardAccessFlagsSet() || c.awsicAssignmentsSet
}

func (c *Command) anyGrantsSet() bool {
	return c.ownerGrantRolesSet || c.ownerGrantTraitsSet ||
		c.memberGrantRolesSet || c.memberGrantTraitsSet
}

// anyUpdateFlagSet returns true if the user passed any update-able flag.
func (c *Command) anyUpdateFlagSet() bool {
	return c.anyMemberUpdateFlagSet() || c.anyNonMemberUpdateFlagSet()
}

func (c *Command) anyMemberUpdateFlagSet() bool {
	return c.membersSet || c.memberAccessListsSet
}

func (c *Command) anyNonMemberUpdateFlagSet() bool {
	return c.titleSet || c.descriptionSet || c.auditFrequencySet || c.auditDaySet ||
		c.ownersSet || c.ownerAccessListsSet || c.ownerRequiredRolesSet || c.ownerRequiredTraitsSet ||
		c.memberRequiredRolesSet || c.memberRequiredTraitsSet ||
		c.removeAccess ||
		c.anyGrantsSet() ||
		c.anyAccessFlagsSet()
}
