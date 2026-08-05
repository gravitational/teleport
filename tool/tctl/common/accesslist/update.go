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
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/accesslists/preset"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

// Safeguard against older role versions that may have unsupported fields.
const minimumRoleVersion = 8

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

	aclName, err := c.accessListScopeQualifiedName()
	if err != nil {
		return trace.Wrap(err)
	}
	al, err := client.AccessListClient().GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
		Scope: aclName.Scope,
		Name:  aclName.Name,
	}.Build())
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
	var updatedRoles []string
	var rolesToDelete []string
	var removedOwners []accesslists.NormalizedSQN
	var removedMembers []accesslists.NormalizedSQN

	switch {
	case c.anyMemberUpdateFlagSet():
		// Member-only update: replace the membership list without touching
		// the access list spec.
		var updatedMembers []*accesslist.AccessListMember
		updatedMembers, removedMembers, err = c.buildMembersForUpdate(ctx, client, aclName)
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
			updatedOwners, removedOwners, err = c.buildOwnersForUpdate(al)
			if err != nil {
				return trace.Wrap(err)
			}
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
		OwnersRemoved:         sliceutils.Map(removedOwners, accesslists.NormalizedSQN.String),
		MembersRemoved:        sliceutils.Map(removedMembers, accesslists.NormalizedSQN.String),
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

	if err := c.applyGrantsAndRequirements(al); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func reconcileOwners(wantUpdate bool, ownersStr string, memberKind string, currOwners map[accesslists.NormalizedSQN]accesslist.Owner) (newOwners []accesslist.Owner, removedOwners []accesslists.NormalizedSQN, err error) {
	if !wantUpdate {
		// return owners as is.
		for _, o := range currOwners {
			newOwners = append(newOwners, o)
		}
		return newOwners, nil, nil
	}

	newOwnerLookup := make(map[accesslists.NormalizedSQN]struct{}, len(currOwners))
	ownerNames, err := splitQualifiedNames(ownersStr)
	if err != nil {
		return nil, nil, trace.Wrap(err, "parsing owner name")
	}
	for _, ownerName := range ownerNames {
		kind := memberKind
		if ownerName.Scope != "" {
			if !accesslist.IsMembershipKindList(memberKind) {
				return nil, nil, trace.BadParameter("user owners cannot be scoped, got %q", ownerName.String())
			}
			kind = accesslist.MembershipKindScopedList
		}
		newOwnerLookup[accesslists.NormalizeSQN(ownerName)] = struct{}{}
		newOwners = append(newOwners, accesslist.Owner{Name: ownerName.String(), MembershipKind: kind})
	}

	for currOwner := range currOwners {
		if _, found := newOwnerLookup[currOwner]; !found {
			removedOwners = append(removedOwners, currOwner)
		}
	}

	return newOwners, removedOwners, nil
}

func reconcileMembers(listName scopes.QualifiedName, wantUpdate bool, membersStr string, memberKind string, currMembers map[accesslists.NormalizedSQN]*accesslist.AccessListMember) (newMembers []*accesslist.AccessListMember, removedMembers []accesslists.NormalizedSQN, err error) {
	if !wantUpdate {
		// return members as is.
		for _, m := range currMembers {
			newMembers = append(newMembers, m)
		}
		return newMembers, nil, nil
	}

	newMemberLookup := make(map[accesslists.NormalizedSQN]struct{}, len(currMembers))
	memberNames, err := splitQualifiedNames(membersStr)
	if err != nil {
		return nil, nil, trace.Wrap(err, "parsing member name")
	}
	for _, memberName := range memberNames {
		kind := memberKind
		if memberName.Scope != "" {
			if !accesslist.IsMembershipKindList(memberKind) {
				return nil, nil, trace.BadParameter("user members cannot be scoped, got %q", memberName.String())
			}
			kind = accesslist.MembershipKindScopedList
		}
		newMemberLookup[accesslists.NormalizeSQN(memberName)] = struct{}{}
		m, err := newMember(listName, memberName, kind)
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
func (c *Command) buildMembersForUpdate(ctx context.Context, client *authclient.Client, listName scopes.QualifiedName) ([]*accesslist.AccessListMember, []accesslists.NormalizedSQN, error) {
	currentMembers, err := c.collectAllMembers(ctx, client, listName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	currentUsers := make(map[accesslists.NormalizedSQN]*accesslist.AccessListMember)
	currentLists := make(map[accesslists.NormalizedSQN]*accesslist.AccessListMember)
	for _, m := range currentMembers {
		memberName, err := accesslists.MemberScopeQualifiedName(m)
		if err != nil {
			return nil, nil, trace.Wrap(err, "parsing member name")
		}
		if m.IsList() {
			currentLists[memberName] = m
		} else {
			currentUsers[memberName] = m
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
func (c *Command) buildOwnersForUpdate(al *accesslist.AccessList) ([]accesslist.Owner, []accesslists.NormalizedSQN, error) {
	currentUserOwners := make(map[accesslists.NormalizedSQN]accesslist.Owner)
	currentListOwners := make(map[accesslists.NormalizedSQN]accesslist.Owner)
	for _, o := range al.Spec.Owners {
		ownerName, err := accesslists.OwnerScopeQualifiedName(o)
		if err != nil {
			return nil, nil, trace.Wrap(err, "parsing owner name")
		}
		if o.IsMembershipKindList() {
			currentListOwners[ownerName] = o
		}
		if o.IsMembershipKindUser() {
			currentUserOwners[ownerName] = o
		}
	}

	userOwners, userRemoved, userErr := reconcileOwners(c.ownersSet, c.owners, accesslist.MembershipKindUser, currentUserOwners)
	listOwners, listRemoved, listErr := reconcileOwners(c.ownerAccessListsSet, c.ownerAccessLists, accesslist.MembershipKindList, currentListOwners)
	newOwners := append(userOwners, listOwners...)
	removedOwners := append(userRemoved, listRemoved...)

	return newOwners, removedOwners, trace.NewAggregate(userErr, listErr)
}

// updateAccessListWithPreset updates an access list spec/meta and its related access roles.
func (c *Command) updateAccessListWithPreset(ctx context.Context, client *authclient.Client, al *accesslist.AccessList) (*accesslist.AccessList, []string, []string, error) {
	if err := rejectUnknownGrants(al); err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

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
	roleNames := make([]string, 0, len(resp.GetRoles()))
	for _, role := range resp.GetRoles() {
		roleNames = append(roleNames, role.GetName())
	}
	return updatedAcl, roleNames, resp.GetRolesToBeDeleted(), nil
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
		fmt.Fprintf(c.Stdout, "Roles updated or created: %s\n", strings.Join(r.UpdatedOrCreatedRoles, ", "))
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
		// New role, nothing to validate.
		role, err = buildRole(prefix, types.RoleConditions{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// Existing role, validate that unsupported role fields were not set.
		// This will happen if a user manually edited the role outside of the editor/command
		// built for access lists using preset (access-type).
		// This helps in avoiding unintentional/silent overwrites or dropping of fields (role/grants).
		if err := validateQueriedRole(prefix, roleName, role.GetVersion(), role.Spec.Allow, role.Spec.Deny); err != nil {
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

func roleDenyMismatchError(roleName string) error {
	return trace.BadParameter("the role Teleport created for this list has deny fields not supported by this update.\n"+
		"Use `tctl edit role/%s` to clear `spec.deny`, then retry; or edit the role directly.",
		roleName)
}

func roleAllowMismatchError(roleName string, unsupportedFields []string) error {
	return trace.BadParameter("the role Teleport created for this list has allow fields not supported by this update: %s.\n"+
		"Use `tctl edit role/%s` to clear those fields or `spec`, then retry; or edit the role directly.",
		strings.Join(unsupportedFields, ", "), roleName)
}

func unknownRoleGrants(unknownRoles []string, accessListID string, field string) error {
	return trace.BadParameter(
		"resource access changes cannot be applied; this list grants roles this command doesn't update: %s.\n"+
			"Use `tctl edit access_list/%s` to remove those roles from %q field, then retry; or edit the access list directly.",
		strings.Join(unknownRoles, ", "), accessListID, field,
	)
}

func unsupportedGrantFields(unsupportedFields []string, accessListID string, field string) error {
	return trace.BadParameter(
		"resource access changes cannot be applied; this list has grant fields not supported by this update: %s.\n"+
			"Use `tctl edit access_list/%s` to clear those fields from %q, then retry; or edit the access list directly.",
		strings.Join(unsupportedFields, ", "), accessListID, field,
	)
}

// rejectUnknownGrants returns an error if an unknown role exists in the
// member/owner grant roles. Also rejects if any grant fields other than
// "roles" were defined. This helps in avoiding silent dropping of
// unsupported fields when updating.
func rejectUnknownGrants(al *accesslist.AccessList) error {
	reviewerRole := preset.RoleName(preset.RoleReviewerPrefix, al.GetName())
	requesterRole := preset.RoleName(preset.RoleRequesterPrefix, al.GetName())
	standardRole := preset.RoleName(standardRolePrefixName, al.GetName())
	awsicRole := preset.RoleName(awsicRolePrefixName, al.GetName())

	unknownRoles := func(validRoles []string, gotRoles []string) []string {
		unknownRoles := []string{}
		for _, roleName := range gotRoles {
			if !slices.Contains(validRoles, roleName) {
				unknownRoles = append(unknownRoles, roleName)
			}
		}
		return unknownRoles
	}

	var validMemberRoles []string
	var validOwnerRoles []string

	switch al.GetAllLabels()[accesslist.AccessListPresetLabel] {
	case string(preset.ShortTermPresetType):
		validMemberRoles = []string{requesterRole}
		validOwnerRoles = []string{reviewerRole}

	case string(preset.LongTermPresetType):
		validMemberRoles = []string{standardRole, awsicRole}
		validOwnerRoles = []string{reviewerRole}
	}

	// Fail if unknown roles exists in any grants.
	unknownMemberRoles := unknownRoles(validMemberRoles, al.Spec.Grants.Roles)
	if len(unknownMemberRoles) > 0 {
		return unknownRoleGrants(unknownMemberRoles, al.GetName(), "spec.grants.roles")
	}
	unknownOwnerRoles := unknownRoles(validOwnerRoles, al.Spec.OwnerGrants.Roles)
	if len(unknownOwnerRoles) > 0 {
		return unknownRoleGrants(unknownOwnerRoles, al.GetName(), "spec.owner_grants.roles")
	}

	// Fail if any other fields (other than roles) are defined.
	grantMismatches := nonEmptyGrantFields(al.Spec.Grants)
	if len(grantMismatches) > 0 {
		return unsupportedGrantFields(grantMismatches, al.GetName(), "spec.grants")
	}
	ownerGrantMismatches := nonEmptyGrantFields(al.Spec.OwnerGrants)
	if len(ownerGrantMismatches) > 0 {
		return unsupportedGrantFields(ownerGrantMismatches, al.GetName(), "spec.owner_grants")
	}
	return nil
}

// getEmptyRoleV6 returns an empty role with defaults set.
func getEmptyRoleV6(roleName, roleVersion string) (*types.RoleV6, error) {
	emptyRoleWithDefaults, err := types.NewRoleWithVersion(roleName, roleVersion, types.RoleSpecV6{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleV6WithDefaults, ok := emptyRoleWithDefaults.(*types.RoleV6)
	if !ok {
		return nil, trace.BadParameter("role %q has unexpected type %T", roleName, emptyRoleWithDefaults)
	}

	return roleV6WithDefaults, nil
}

// validateAWSICRoleSpec returns an error if any deny fields got set or if any other allow fields
// other than ones allowed are set.
func validateAWSICRoleSpec(roleName string, wantWithDefaults *types.RoleV6, gotAllow types.RoleConditions, gotDeny types.RoleConditions) error {
	denyMismatches := topLevelRoleConditionMismatches(wantWithDefaults.Spec.Deny, gotDeny)
	if len(denyMismatches) > 0 {
		return roleDenyMismatchError(roleName)
	}

	// Empty allow is valid.
	allowMismatches := topLevelRoleConditionMismatches(wantWithDefaults.Spec.Allow, gotAllow)
	if len(allowMismatches) == 0 {
		return nil
	}

	validAllow := wantWithDefaults.Spec.Allow
	// Set allowed fields:
	validAllow.AppLabels = awsIcAppLabel
	validAllow.AccountAssignments = gotAllow.AccountAssignments
	allowMismatches = topLevelRoleConditionMismatches(validAllow, gotAllow)
	if len(allowMismatches) > 0 {
		return roleAllowMismatchError(roleName, allowMismatches)
	}

	return nil
}

// validateStandardRoleSpec returns an error if any deny fields got set or if any other allow fields
// other than ones allowed are set.
func validateStandardRoleSpec(roleName string, wantWithDefaults *types.RoleV6, gotAllow types.RoleConditions, gotDeny types.RoleConditions) error {
	validAllow := wantWithDefaults.Spec.Allow
	// Set allowed fields:
	validAllow.NodeLabels = gotAllow.NodeLabels
	validAllow.Logins = gotAllow.Logins
	validAllow.DatabaseLabels = gotAllow.DatabaseLabels
	validAllow.DatabaseUsers = gotAllow.DatabaseUsers
	validAllow.DatabaseNames = gotAllow.DatabaseNames
	validAllow.KubernetesLabels = gotAllow.KubernetesLabels
	validAllow.KubeUsers = gotAllow.KubeUsers
	validAllow.KubeGroups = gotAllow.KubeGroups
	validAllow.KubernetesResources = gotAllow.KubernetesResources
	validAllow.AppLabels = gotAllow.AppLabels
	validAllow.AWSRoleARNs = gotAllow.AWSRoleARNs
	validAllow.AzureIdentities = gotAllow.AzureIdentities
	validAllow.GCPServiceAccounts = gotAllow.GCPServiceAccounts
	validAllow.MCP = gotAllow.MCP
	validAllow.WindowsDesktopLabels = gotAllow.WindowsDesktopLabels
	validAllow.WindowsDesktopLogins = gotAllow.WindowsDesktopLogins
	validAllow.GitHubPermissions = gotAllow.GitHubPermissions

	denyMismatches := topLevelRoleConditionMismatches(wantWithDefaults.Spec.Deny, gotDeny)
	if len(denyMismatches) > 0 {
		return roleDenyMismatchError(roleName)
	}

	allowMismatches := topLevelRoleConditionMismatches(validAllow, gotAllow)
	if len(allowMismatches) > 0 {
		return roleAllowMismatchError(roleName, allowMismatches)
	}

	return nil
}

func supportsRoleVersion(gotVersion string, roleName string) error {
	roleVersionNumber := func(version string) (int, error) {
		return strconv.Atoi(strings.TrimPrefix(version, "v"))
	}

	gotVersionNum, err := roleVersionNumber(gotVersion)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return trace.BadParameter(
			"resource access changes cannot be applied; role is using an unsupported version %q.\nUse `tctl edit role/%s` to upgrade its version, or edit the role directly.",
			gotVersion, roleName,
		)
	}
	if gotVersionNum < minimumRoleVersion {
		return trace.BadParameter(
			"resource access changes cannot be applied; role is using an unsupported version %q.\nUse `tctl edit role/%s` to upgrade its version, or edit the role directly.",
			gotVersion, roleName,
		)
	}

	defaultVersionNum, err := roleVersionNumber(types.DefaultRoleVersion)
	if err != nil || gotVersionNum > defaultVersionNum {
		return trace.BadParameter(
			"resource access changes cannot be applied; role %q is using an unsupported version %q.\nUpgrade your tctl binary.",
			roleName, gotVersion,
		)
	}

	return nil
}

func validateQueriedRole(prefix string, roleName, roleVersion string, allow, deny types.RoleConditions) error {
	// Rare: the role predates this feature's minimum version requirement, or
	// was manually edited to an older version.
	if err := supportsRoleVersion(roleVersion, roleName); err != nil {
		return trace.Wrap(err)
	}
	emptyRoleV6WithDefaults, err := getEmptyRoleV6(roleName, roleVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	switch prefix {
	case standardRolePrefixName:
		return validateStandardRoleSpec(roleName, emptyRoleV6WithDefaults, allow, deny)
	case awsicRolePrefixName:
		return validateAWSICRoleSpec(roleName, emptyRoleV6WithDefaults, allow, deny)
	}
	return nil
}

// UpdateJSONResponse is the structured response for `tctl acl update
// --format=json`.
type UpdateJSONResponse struct {
	AccessList            *accesslist.AccessList `json:"access_list"`
	AccessType            string                 `json:"access_type,omitempty"`
	UpdatedOrCreatedRoles []string               `json:"updated_or_created_roles,omitempty"`
	RolesToDelete         []string               `json:"roles_to_delete,omitempty"`
	OwnersRemoved         []string               `json:"owners_removed,omitempty"`
	MembersRemoved        []string               `json:"members_removed,omitempty"`
}
