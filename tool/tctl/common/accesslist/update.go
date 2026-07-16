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
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/lib/accesslists/preset"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
)

// Update handles `tctl acl update`.
func (c *Command) Update(ctx context.Context, client *authclient.Client) error {
	if !c.anyUpdateFlagSet() {
		return trace.BadParameter("no update flags are set")
	}

	// To avoid non-atomic partial writes, member updates are to be run
	// separately from updates to access list meta/spec. Combining
	// both means there will be partial failures where grants may be
	// unintentionally given b/c member update operation failed.
	if c.anyMemberUpdateFlagSet() && c.anyNonMemberUpdateFlagSet() {
		return trace.BadParameter("--members/--member-access-lists replaces membership only; run a separate `tctl acl update` to change access list settings")
	}

	if c.titleSet && c.title == "" {
		return trace.BadParameter("cannot unset title")
	}

	al, err := client.AccessListClient().GetAccessList(ctx, c.accessListName)
	if err != nil {
		return trace.Wrap(err)
	}

	if !al.IsPreset() {
		if c.anyAccessFlagsSet() {
			return trace.BadParameter("resource access flags (--node-labels, --logins, --aws-ic-assignments, etc.) cannot be applied; access list %q was not created with an access type", c.accessListName)
		}
		if c.removeAccess {
			return trace.BadParameter("--remove-access cannot be applied; access list %q was not created with an access type, so it has no resource access to remove", c.accessListName)
		}
	} else { // Is preset.
		if c.anyGrantsSet() {
			return trace.BadParameter("grant flags (--member-grant-*, --owner-grant-*) cannot be applied; access list %q was created with an access type where grants are automatically assigned by Teleport", c.accessListName)
		}
		if c.removeAccess && c.anyAccessFlagsSet() {
			return trace.BadParameter("--remove-access removes all resource access and cannot be combined with resource access flags (--node-labels, --logins, etc.)")
		}
	}

	var updatedAccessList *accesslist.AccessList
	var updatedRoles []*types.RoleV6
	var rolesToDelete []string
	var removedOwners []string
	var removedMembers []string

	switch {
	case c.anyMemberUpdateFlagSet():
		// Member-only update: replace the membership list without touching
		// the access list spec.
		var updatedMembers []*accesslist.AccessListMember
		updatedMembers, removedMembers, err = c.buildMembersForUpdate(ctx, client, al.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		updatedAccessList, _, err = client.AccessListClient().UpsertAccessListWithMembers(ctx, al, updatedMembers)
		if err != nil {
			return trace.Wrap(err)
		}

	// Access list spec/meta only update:
	default:
		if err := c.applySpecFlags(al); err != nil {
			return trace.Wrap(err)
		}
		if c.ownersSet || c.ownerAccessListsSet {
			var updatedOwners []accesslist.Owner
			updatedOwners, removedOwners = c.buildOwnersForUpdate(al)
			if len(updatedOwners) == 0 {
				return trace.BadParameter("an access list must have at least one owner")
			}
			al.Spec.Owners = updatedOwners
		}

		if al.IsPreset() && (c.anyAccessFlagsSet() || c.removeAccess) {
			updatedAccessList, updatedRoles, rolesToDelete, err = c.updateAccessListWithPreset(ctx, client, al)
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			updatedAccessList, err = client.AccessListClient().UpdateAccessList(ctx, al)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	resp := UpdateJSONResponse{
		AccessList:            updatedAccessList,
		AccessType:            accessType(updatedAccessList.GetAllLabels()[accesslist.AccessListPresetLabel]),
		UpdatedOrCreatedRoles: updatedRoles,
		RolesToDelete:         rolesToDelete,
		OwnersRemoved:         removedOwners,
		MembersRemoved:        removedMembers,
	}
	if c.format == teleport.JSON {
		return trace.Wrap(utils.WriteJSON(c.Stdout, resp), "failed to marshal access list update response")
	}
	c.printUpdateText(resp)
	return nil
}

func (c *Command) applySpecFlags(al *accesslist.AccessList) error {
	// Metadata
	if c.titleSet {
		al.Spec.Title = c.title
	}
	if c.descriptionSet {
		al.Spec.Description = c.description
	}
	if c.auditFrequencySet {
		freq, err := getReviewFrequency(c.auditFrequency)
		if err != nil {
			return trace.Wrap(err)
		}
		al.Spec.Audit.Recurrence.Frequency = freq
	}
	if c.auditDaySet {
		day, err := getReviewDayOfMonth(c.auditDay)
		if err != nil {
			return trace.Wrap(err)
		}
		al.Spec.Audit.Recurrence.DayOfMonth = day
	}
	// Zero out NextAuditDate so the backend can re-compute this field
	// automatically.
	if c.auditFrequencySet || c.auditDaySet {
		al.Spec.Audit.NextAuditDate = time.Time{}
	}

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

type applyAccessFlagsToRole func(allow *types.RoleConditions) error

// applyAWSICFlagsToRole modifies the AWS IC role's allow block.
// Empty values clear the whole allow spec since this role is specific to awsic,
// unset flags leave fields alone.
func (c *Command) applyAWSICFlagsToRole(allow *types.RoleConditions) error {
	if c.awsicAssignments == "" {
		*allow = types.RoleConditions{}
	} else {
		allow.AppLabels = awsIcAppLabel
		aa, err := buildAWSICAccountAssignments(c.awsicAssignments)
		if err != nil {
			return trace.Wrap(err)
		}
		allow.AccountAssignments = aa
	}
	return nil
}

// applyStandardAccessFlagsToRole modifies the standard role's allow block.
// Empty values clear the field, unset flags leave field alone.
func (c *Command) applyStandardAccessFlagsToRole(allow *types.RoleConditions) error {
	// Nodes
	if c.nodeLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.nodeLabels)
		if err != nil {
			return trace.Wrap(err, "--node-labels")
		}
		allow.NodeLabels = types.ToLabels(labels)
	}
	if c.loginsSet {
		allow.Logins = utils.SplitIdentifiers(c.logins)
	}

	// Dbs
	if c.dbLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.dbLabels)
		if err != nil {
			return trace.Wrap(err, "--db-labels")
		}
		allow.DatabaseLabels = types.ToLabels(labels)
	}
	if c.dbUsersSet {
		allow.DatabaseUsers = utils.SplitIdentifiers(c.dbUsers)
	}
	if c.dbNamesSet {
		allow.DatabaseNames = utils.SplitIdentifiers(c.dbNames)
	}

	// Kubes
	if c.kubeLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.kubeLabels)
		if err != nil {
			return trace.Wrap(err, "--kubernetes-labels")
		}
		allow.KubernetesLabels = types.ToLabels(labels)
	}
	if c.kubeUsersSet {
		allow.KubeUsers = utils.SplitIdentifiers(c.kubeUsers)
	}
	if c.kubeGroupsSet {
		allow.KubeGroups = utils.SplitIdentifiers(c.kubeGroups)
	}

	// Apps
	if c.appLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.appLabels)
		if err != nil {
			return trace.Wrap(err, "--app-labels")
		}
		allow.AppLabels = types.ToLabels(labels)
	}
	if c.awsRoleARNsSet {
		allow.AWSRoleARNs = utils.SplitIdentifiers(c.awsRoleARNs)
	}
	if c.azureIdentitiesSet {
		allow.AzureIdentities = utils.SplitIdentifiers(c.azureIdentities)
	}
	if c.gcpServiceAccountsSet {
		allow.GCPServiceAccounts = utils.SplitIdentifiers(c.gcpServiceAccounts)
	}
	if c.mcpToolsSet {
		tools := utils.SplitIdentifiers(c.mcpTools)
		if len(tools) == 0 {
			allow.MCP = nil
		} else {
			allow.MCP = &types.MCPPermissions{Tools: tools}
		}
	}

	// Windows
	if c.windowsLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.windowsLabels)
		if err != nil {
			return trace.Wrap(err, "--windows-labels")
		}
		allow.WindowsDesktopLabels = types.ToLabels(labels)
	}
	if c.windowsLoginsSet {
		allow.WindowsDesktopLogins = utils.SplitIdentifiers(c.windowsLogins)
	}

	// GitHub
	if c.gitHubOrgsSet {
		orgs := utils.SplitIdentifiers(c.gitHubOrgs)
		if len(orgs) == 0 {
			allow.GitHubPermissions = nil
		} else {
			allow.GitHubPermissions = []types.GitHubPermission{{Organizations: orgs}}
		}
	}
	return nil
}

func reconcileOwners(wantUpdate bool, ownersStr string, memberKind string, currOwners map[string]accesslist.Owner) (newOwners []accesslist.Owner, removedOwners []string) {
	if !wantUpdate {
		// return owners as is.
		for _, o := range currOwners {
			newOwners = append(newOwners, o)
		}
		return newOwners, nil
	}

	newOwnerLookup := make(map[string]struct{}, len(currOwners))
	for _, owner := range utils.SplitIdentifiers(ownersStr) {
		newOwnerLookup[owner] = struct{}{}
		newOwners = append(newOwners, accesslist.Owner{Name: owner, MembershipKind: memberKind})
	}

	for currOwner := range currOwners {
		if _, found := newOwnerLookup[currOwner]; !found {
			removedOwners = append(removedOwners, currOwner)
		}
	}

	return newOwners, removedOwners
}

func reconcileMembers(listName string, wantUpdate bool, membersStr string, memberKind string, currMembers map[string]*accesslist.AccessListMember) (newMembers []*accesslist.AccessListMember, removedMembers []string, err error) {
	if !wantUpdate {
		// return members as is.
		for _, m := range currMembers {
			newMembers = append(newMembers, m)
		}
		return newMembers, nil, nil
	}

	newMemberLookup := make(map[string]struct{}, len(currMembers))
	for _, name := range utils.SplitIdentifiers(membersStr) {
		newMemberLookup[name] = struct{}{}
		m, err := newMember(listName, name, memberKind)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		newMembers = append(newMembers, m)
	}

	for currMember := range currMembers {
		if _, found := newMemberLookup[currMember]; !found {
			removedMembers = append(removedMembers, currMember)
		}
	}

	return newMembers, removedMembers, nil
}

// buildMembersForUpdate returns new member list and all members removed from previous list.
// It separates users and access list from members so that flags "--members" and "--member-access-lists"
// updates only the specified member kinds.
// E.g.: if only "--member-access-lists" was specified, only member kind "access list" is updated
// and member kind "user" is still preserved.
func (c *Command) buildMembersForUpdate(ctx context.Context, client *authclient.Client, listName string) ([]*accesslist.AccessListMember, []string, error) {
	currentMembers, err := c.collectAllMembers(ctx, client, listName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	currentUsers := make(map[string]*accesslist.AccessListMember)
	currentLists := make(map[string]*accesslist.AccessListMember)
	for _, m := range currentMembers {
		if m.Spec.MembershipKind == accesslist.MembershipKindList {
			currentLists[m.Spec.Name] = m
		} else {
			currentUsers[m.Spec.Name] = m
		}
	}

	users, userRemoved, err := reconcileMembers(listName, c.membersSet, c.members, accesslist.MembershipKindUser, currentUsers)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	lists, listRemoved, err := reconcileMembers(listName, c.memberAccessListsSet, c.memberAccessLists, accesslist.MembershipKindList, currentLists)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return append(users, lists...), append(userRemoved, listRemoved...), nil
}

// buildOwnersForUpdate returns new owner list and all owners removed from previous list.
// It separates users and access list from owner so that flags "--owners" and "--owner-access-lists"
// updates only the specified owner kinds.
// E.g.: if only "--owner-access-lists" was specified, only owner kind "access list" is updated
// and owner kind "user" is still preserved.
func (c *Command) buildOwnersForUpdate(al *accesslist.AccessList) ([]accesslist.Owner, []string) {
	currentUserOwners := make(map[string]accesslist.Owner)
	currentListOwners := make(map[string]accesslist.Owner)
	for _, o := range al.Spec.Owners {
		if o.MembershipKind == accesslist.MembershipKindList {
			currentListOwners[o.Name] = o
		} else {
			currentUserOwners[o.Name] = o
		}
	}

	userOwners, userRemoved := reconcileOwners(c.ownersSet, c.owners, accesslist.MembershipKindUser, currentUserOwners)
	listOwners, listRemoved := reconcileOwners(c.ownerAccessListsSet, c.ownerAccessLists, accesslist.MembershipKindList, currentListOwners)
	newOwners := append(userOwners, listOwners...)
	removedOwners := append(userRemoved, listRemoved...)

	return newOwners, removedOwners
}

// updateAccessListWithPreset updates an access list spec/meta and its related access roles.
func (c *Command) updateAccessListWithPreset(ctx context.Context, client *authclient.Client, al *accesslist.AccessList) (*accesslist.AccessList, []*types.RoleV6, []string, error) {
	var updatedAccessRoles []*types.RoleV6
	if !c.removeAccess {
		var standardRoleUpdateFn applyAccessFlagsToRole
		if c.anyStandardAccessFlagsSet() {
			standardRoleUpdateFn = c.applyStandardAccessFlagsToRole
		}
		standardRole, err := resolveAccessRole(ctx, client, al, standardRolePrefixName, standardRoleUpdateFn)
		if err != nil {
			return nil, nil, nil, trace.Wrap(err)
		}

		var awsicRoleUpdateFn applyAccessFlagsToRole
		if c.awsicAssignmentsSet {
			awsicRoleUpdateFn = c.applyAWSICFlagsToRole
		}
		awsicRole, err := resolveAccessRole(ctx, client, al, awsicRolePrefixName, awsicRoleUpdateFn)
		if err != nil {
			return nil, nil, nil, trace.Wrap(err)
		}

		if standardRole != nil {
			updatedAccessRoles = append(updatedAccessRoles, standardRole)
		}
		if awsicRole != nil {
			updatedAccessRoles = append(updatedAccessRoles, awsicRole)
		}
	} // else, no roles appended means "remove these access roles from al grants"

	grpcClient := accesslistv1.NewAccessListServiceClient(client.GetConnection())
	resp, err := grpcClient.UpdateAccessListWithPreset(ctx, accesslistv1.UpdateAccessListWithPresetRequest_builder{
		AccessList: conv.ToProto(al),
		Roles:      updatedAccessRoles,
	}.Build())
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	updatedAcl, err := conv.FromProto(resp.GetAccessList())
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	return updatedAcl, resp.GetRoles(), resp.GetRolesToBeDeleted(), nil
}

// printUpdateText renders the human-readable summary of an update.
func (c *Command) printUpdateText(r UpdateJSONResponse) {
	fmt.Fprintf(c.Stdout, "Updated access list %q (%s)\n", r.AccessList.Spec.Title, r.AccessList.GetName())

	if len(r.OwnersRemoved) > 0 {
		fmt.Fprintf(c.Stdout, "Owners removed: %s\n", strings.Join(r.OwnersRemoved, ", "))
	}

	if len(r.MembersRemoved) > 0 {
		fmt.Fprintf(c.Stdout, "Members removed: %s\n", strings.Join(r.MembersRemoved, ", "))
	}

	if len(r.UpdatedOrCreatedRoles) > 0 {
		names := make([]string, 0, len(r.UpdatedOrCreatedRoles))
		for _, role := range r.UpdatedOrCreatedRoles {
			names = append(names, role.GetMetadata().Name)
		}
		fmt.Fprintf(c.Stdout, "Roles updated or created: %s\n", strings.Join(names, ", "))
	}

	c.printRolesToBeDeleted(r.RolesToDelete)
}

// resolveAccessRole returns the access role to send for upsert. A role is
// "attached" when it exists and is listed in al.PresetRoleNames(); "not
// attached" covers both missing roles and roles not currently referenced
// by the access list.
// - attached: return existing role (with updates applied if requested)
// - not attached + updates requested: return a fresh role with updates applied
// - not attached + no updates: return nil
func resolveAccessRole(ctx context.Context, client *authclient.Client, al *accesslist.AccessList, prefix string, applyUpdates applyAccessFlagsToRole) (*types.RoleV6, error) {
	roleName := preset.RoleName(prefix, al.GetName())
	role, err := getAccessRoleByName(ctx, client, roleName)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	roleExists := err == nil

	// If an update is required and this existing role is attached to the list already,
	// update the existing role, else assume a fresh role.
	isAttachedToList := roleExists && slices.Contains(al.PresetRoleNames(), roleName)

	if applyUpdates == nil {
		if isAttachedToList {
			return role, nil
		}
		return nil, nil
	}

	if !isAttachedToList {
		role, err = buildRole(prefix, types.RoleConditions{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := applyUpdates(&role.Spec.Allow); err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

// getAccessRoleByName fetches a role by given name.
func getAccessRoleByName(ctx context.Context, client *authclient.Client, roleName string) (*types.RoleV6, error) {
	role, err := client.GetRole(ctx, roleName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleV6, ok := role.(*types.RoleV6)
	if !ok {
		return nil, trace.BadParameter("role %q has unexpected type %T", roleName, role)
	}
	return roleV6, nil
}

// UpdateJSONResponse is the structured response for `tctl acl update
// --format=json`.
type UpdateJSONResponse struct {
	AccessList            *accesslist.AccessList `json:"access_list"`
	AccessType            string                 `json:"access_type,omitempty"`
	UpdatedOrCreatedRoles []*types.RoleV6        `json:"updated_or_created_roles,omitempty"`
	RolesToDelete         []string               `json:"roles_to_delete,omitempty"`
	OwnersRemoved         []string               `json:"owners_removed,omitempty"`
	MembersRemoved        []string               `json:"members_removed,omitempty"`
}
