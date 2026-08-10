/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils"
)

// Create handles `tctl acl create`.
func (c *Command) Create(ctx context.Context, client *authclient.Client) error {
	if err := c.validateCreate(); err != nil {
		return trace.Wrap(err)
	}

	newAccessList, err := c.buildAccessListForCreate()
	if err != nil {
		return trace.Wrap(err)
	}

	members, err := c.buildMembers(accesslists.ScopeQualifiedName(newAccessList).ToScopesQualifiedName())
	if err != nil {
		return trace.Wrap(err)
	}

	createResponse, err := c.createAccessList(ctx, client, newAccessList, members)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.printCreateResult(createResponse))
}

func (c *Command) validateCreate() error {
	if c.accessType == "" {
		if c.anyAccessFlagsSet() {
			return trace.BadParameter("resource access flags (--node-labels, --logins, --aws-ic-assignments, etc.) require --access-type")
		}
	} else {
		if presetType(c.accessType) == "" {
			return trace.BadParameter("--access-type must be %s or %s (got %q)", accessTypeLongTerm, accessTypeShortTerm, c.accessType)
		}
		if c.anyGrantsSet() {
			return trace.BadParameter("grant flags (--member-grant-*, --owner-grant-*) cannot be combined with --access-type; grants are automatically assigned by Teleport")
		}
	}

	if !c.titleSet {
		return trace.BadParameter("--title is required")
	}

	if !c.ownersSet && !c.ownerAccessListsSet {
		return trace.BadParameter("at least one of --owners or --owner-access-lists is required")
	}

	return nil
}

func (c *Command) buildOwners() ([]accesslist.Owner, error) {
	var owners []accesslist.Owner
	for _, name := range utils.SplitIdentifiers(c.owners) {
		owners = append(owners, accesslist.Owner{Name: name, MembershipKind: accesslist.MembershipKindUser})
	}
	ownerNames, err := splitACLQualifiedNames(c.ownerAccessLists)
	if err != nil {
		return nil, trace.BadParameter("parsing owner access list name")
	}
	for _, ownerName := range ownerNames {
		kind := accesslist.MembershipKindList
		if ownerName.Scope != "" {
			kind = accesslist.MembershipKindScopedList
		}
		owners = append(owners, accesslist.Owner{Name: ownerName.String(), MembershipKind: kind})
	}
	return owners, nil
}

func (c *Command) buildAccessListForCreate() (*accesslist.AccessList, error) {
	reviewFreq, err := getReviewFrequency(c.auditFrequency)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reviewMonth, err := getReviewDayOfMonth(c.auditDay)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	owners, err := c.buildOwners()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newAccessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: uuid.NewString(),
		},
		accesslist.Spec{
			Title:       strings.TrimSpace(c.title),
			Description: strings.TrimSpace(c.description),
			Owners:      owners,
			Audit: accesslist.Audit{
				Recurrence: accesslist.Recurrence{
					Frequency:  reviewFreq,
					DayOfMonth: reviewMonth,
				},
			},
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := c.applyGrantsAndRequirements(newAccessList); err != nil {
		return nil, trace.Wrap(err)
	}

	return newAccessList, nil
}

func (c *Command) createAccessList(ctx context.Context, client *authclient.Client, newAccessList *accesslist.AccessList, newMembers []*accesslist.AccessListMember) (CreateResponse, error) {
	createResponse := CreateResponse{
		AccessType: c.accessType,
	}

	var createdAccessList *accesslist.AccessList

	if c.accessType != "" {
		accessRoles, err := c.buildResourceAccessRoles()
		if err != nil {
			return CreateResponse{}, trace.Wrap(err)
		}

		grpcClient := accesslistv1.NewAccessListServiceClient(client.GetConnection())
		resp, err := grpcClient.CreateAccessListWithPreset(ctx, accesslistv1.CreateAccessListWithPresetRequest_builder{
			PresetType: presetType(c.accessType),
			AccessList: conv.ToProto(newAccessList),
			Roles:      accessRoles,
		}.Build())
		if err != nil {
			return CreateResponse{}, printPresetCreateError(ctx, client, newAccessList.GetName(), err)
		}

		createdAccessList, err = conv.FromProto(resp.GetAccessList())
		if err != nil {
			return CreateResponse{}, printPresetCreateError(ctx, client, newAccessList.GetName(), err)
		}
		createResponse.CreatedRoles = createdAccessList.PresetRoleNames()

		if len(newMembers) > 0 {
			_, _, err = client.AccessListClient().UpsertAccessListWithMembers(ctx, createdAccessList, newMembers)
			if err != nil {
				return CreateResponse{}, c.printMemberCreateError(newAccessList.GetName(), err)
			}
		}

	} else {
		// Regular access list create.
		var err error
		createdAccessList, _, err = client.AccessListClient().UpsertAccessListWithMembers(ctx, newAccessList, newMembers)
		if err != nil {
			return CreateResponse{}, trace.Wrap(err)
		}
	}
	createResponse.AccessList = createdAccessList

	return createResponse, nil
}

func (c *Command) printCreateResult(resp CreateResponse) error {
	if c.format == teleport.JSON {
		return trace.Wrap(utils.WriteJSON(c.Stdout, resp), "failed to marshal access list create response")
	}

	fmt.Fprintf(c.Stdout, "Created access list %q (%s)\n", resp.AccessList.Spec.Title, resp.AccessList.GetName())
	if c.accessType != "" {
		fmt.Fprintf(c.Stdout, "Access type: %s\n", c.accessType)
	}
	if len(resp.CreatedRoles) > 0 {
		fmt.Fprintf(c.Stdout, "Roles created for the access list:\n")
		for _, name := range resp.CreatedRoles {
			fmt.Fprintf(c.Stdout, "  - %s\n", name)
		}
	}
	return nil
}

func printPresetCreateError(ctx context.Context, client *authclient.Client, accessListName string, createErr error) error {
	_, err := client.AccessListClient().GetAccessList(ctx, accessListName)
	if err == nil {
		return trace.Errorf(
			"%s\n\n"+
				"An access list named %q was created before the operation failed.\n\n"+
				"Run `tctl acl rm %s` and follow the cleanup instructions before retrying.",
			trace.UserMessage(createErr),
			accessListName,
			accessListName,
		)
	} else if !trace.IsNotFound(err) {
		return trace.Errorf(
			"%s\n\n"+
				"Teleport could not verify whether partial resources were created.\n\n"+
				"Try running `tctl acl rm %s` and follow the cleanup instructions before retrying.",
			trace.UserMessage(createErr),
			accessListName,
		)
	}

	// Not found error, means no resources were created so no cleanup instructions required.
	return trace.Wrap(createErr)
}

func (c *Command) printMemberCreateError(accessListName string, createErr error) error {
	updateCommand := fmt.Sprintf("  tctl acl update %s", accessListName)
	if c.membersSet {
		updateCommand = fmt.Sprintf("%s --members=%q", updateCommand, c.members)
	}
	if c.memberAccessListsSet {
		updateCommand = fmt.Sprintf("%s --member-access-lists=%q", updateCommand, c.memberAccessLists)
	}
	return trace.Errorf(
		"%s\n\n"+
			"Access list %q was created, but Teleport could not add all requested members.\n\n"+
			"To retry member setup, run:\n"+
			updateCommand,
		trace.UserMessage(createErr),
		accessListName,
	)
}

func (c *Command) buildResourceAccessRoles() ([]*types.RoleV6, error) {
	var roles []*types.RoleV6

	if c.anyStandardAccessFlagsSet() {
		var allow types.RoleConditions
		if err := c.applyStandardAccessFlagsToRole(&allow); err != nil {
			return nil, trace.Wrap(err)
		}
		role, err := buildRole(standardRolePrefixName, allow)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role)
	}

	if c.awsicAssignmentsSet {
		var allow types.RoleConditions
		if err := c.applyAWSICFlagsToRole(&allow); err != nil {
			return nil, trace.Wrap(err)
		}
		role, err := buildRole(awsicRolePrefixName, allow)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func (c *Command) buildMembers(accessListName scopes.QualifiedName) ([]*accesslist.AccessListMember, error) {
	var members []*accesslist.AccessListMember
	for _, name := range utils.SplitIdentifiers(c.members) {
		m, err := newMember(accessListName, scopes.QualifiedName{Name: name}, accesslist.MembershipKindUser)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		members = append(members, m)
	}
	memberListNames, err := splitACLQualifiedNames(c.memberAccessLists)
	if err != nil {
		return nil, trace.Wrap(err, "parsing member access list name")
	}
	for _, name := range memberListNames {
		kind := accesslist.MembershipKindList
		if name.Scope != "" {
			kind = accesslist.MembershipKindScopedList
		}
		m, err := newMember(accessListName, name, kind)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		members = append(members, m)
	}
	return members, nil
}

// CreateResponse is a structured response when `format=json`
// is requested.
type CreateResponse struct {
	AccessList   *accesslist.AccessList `json:"access_list"`
	AccessType   string                 `json:"access_type,omitempty"`
	CreatedRoles []string               `json:"created_roles,omitempty"`
}
