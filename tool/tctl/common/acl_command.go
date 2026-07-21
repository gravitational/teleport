/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/accesslists"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// ACLCommand implements the `tctl acl` family of commands.
type ACLCommand struct {
	format string

	ls            *kingpin.CmdClause
	get           *kingpin.CmdClause
	usersAdd      *kingpin.CmdClause
	usersRemove   *kingpin.CmdClause
	usersList     *kingpin.CmdClause
	reviewsCreate *kingpin.CmdClause
	reviewsList   *kingpin.CmdClause

	// Used for managing a particular access list.
	accessListName string
	// Used to add an access list to another one
	memberKind string

	// Used for managing membership to an access list.
	memberName string
	expires    string
	reason     string

	// Used for creating reviews.
	notes         string
	removeMembers string

	// Some extra options that control output.
	reviewOnly bool // lists only access lists due for review

	// Stdout allows to switch the standard output source. Used in tests.
	Stdout io.Writer
}

const (
	memberKindUser = "user"
	memberKindList = "list"
)

// Initialize allows ACLCommand to plug itself into the CLI parser
func (c *ACLCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	acl := app.Command("acl", "Manage Access Lists.").Alias("access-lists")

	c.ls = acl.Command("ls", "List cluster Access Lists.")
	c.ls.Flag("format", "Output format.").Default(teleport.YAML).EnumVar(&c.format, teleport.YAML, teleport.JSON, teleport.Text)
	c.ls.Flag("review-only", "List only access lists that are due for review within the next 2 weeks or past due").BoolVar(&c.reviewOnly)

	c.get = acl.Command("get", "Get detailed information for an Access List.")
	c.get.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.get.Flag("format", "Output format.").Default(teleport.YAML).EnumVar(&c.format, teleport.YAML, teleport.JSON, teleport.Text)

	users := acl.Command("users", "Manage user membership to Access Lists.")

	c.usersAdd = users.Command("add", "Add a user to an Access List.")
	c.usersAdd.Flag("kind", "Access list member kind.").Default(memberKindUser).EnumVar(&c.memberKind, memberKindUser, memberKindList)
	c.usersAdd.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.usersAdd.Arg("user", "The member to add to the Access List.").Required().StringVar(&c.memberName)
	c.usersAdd.Arg("expires", "When the user's access expires (must be in RFC3339). Defaults to the expiration time of the Access List.").StringVar(&c.expires)
	c.usersAdd.Arg("reason", "The reason the user has been added to the Access List. Defaults to empty.").StringVar(&c.reason)

	c.usersRemove = users.Command("rm", "Remove a user from an Access List.")
	c.usersRemove.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.usersRemove.Arg("user", "The member to remove from the Access List.").Required().StringVar(&c.memberName)

	c.usersList = users.Command("ls", "List users that are members of an Access List.")
	c.usersList.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.usersList.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.format, teleport.JSON, teleport.Text)

	reviews := acl.Command("reviews", "Manage access list reviews.")

	c.reviewsCreate = reviews.Command("create", "Submit a new review for a given access list.")
	c.reviewsCreate.Arg("access-list-name", "The access list name to submit review for.").Required().StringVar(&c.accessListName)
	c.reviewsCreate.Flag("notes", "Optional review notes.").StringVar(&c.notes)
	c.reviewsCreate.Flag("remove-members", "Comma-separated list of members to remove as part of this review.").StringVar(&c.removeMembers)

	c.reviewsList = reviews.Command("ls", "List past audit history for a given access list.")
	c.reviewsList.Arg("access-list-name", "The access list name to fetch review history for.").Required().StringVar(&c.accessListName)
	c.reviewsList.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.format, teleport.YAML, teleport.JSON, teleport.Text)

	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
}

// TryRun takes the CLI command as an argument (like "acl ls") and executes it.
func (c *ACLCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.ls.FullCommand():
		commandFunc = c.List
	case c.get.FullCommand():
		commandFunc = c.Get
	case c.usersAdd.FullCommand():
		commandFunc = c.UsersAdd
	case c.usersRemove.FullCommand():
		commandFunc = c.UsersRemove
	case c.usersList.FullCommand():
		commandFunc = c.UsersList
	case c.reviewsCreate.FullCommand():
		commandFunc = c.ReviewsCreate
	case c.reviewsList.FullCommand():
		commandFunc = c.ReviewsList
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

// List will list access lists visible to the user.
func (c *ACLCommand) List(ctx context.Context, client *authclient.Client) error {
	var accessLists []*accesslist.AccessList
	var err error

	if c.reviewOnly {
		accessLists, err = client.AccessListClient().GetAccessListsToReview(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		accessLists, err = stream.Collect(clientutils.Resources(ctx,
			func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
				return client.AccessListClient().ListAccessListsV2(ctx, accesslistv1.ListAccessListsV2Request_builder{
					PageSize:  int32(pageSize),
					PageToken: pageToken,
					ScopeFilter: scopesv1.Filter_builder{
						Mode: scopesv1.Mode_MODE_ALL,
					}.Build(),
				}.Build())
			}))
		if trace.IsNotImplemented(err) {
			accessLists, err = stream.Collect(clientutils.Resources(ctx, client.AccessListClient().ListAccessLists))
		}
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if len(accessLists) == 0 && c.format == teleport.Text {
		fmt.Fprintln(c.Stdout, "no access lists")
		return nil
	}

	return trace.Wrap(c.displayAccessLists(accessLists...))
}

func parseACLQualifiedName(name string) (scopes.QualifiedName, error) {
	// For backward compatibility, an argument is considered a scope-qualified
	// name only if it:
	// 1. parses as a qualified name (contains :: somewhere in the middle)
	// 2. contains /
	// otherwise it is considered a plain unscoped name. This allows access
	// lists with names like 'example::list' to continue to be referenced in
	// CLI commands as unscoped lists, only params like '/example::list' will
	// be considered scoped.
	if sqn, err := scopes.ParseQualifiedName(name); err == nil && strings.Contains(name, "/") {
		return sqn, sqn.StrongValidate()
	}
	return scopes.QualifiedName{Name: name}, nil
}

func (c *ACLCommand) accessListQualifiedName() (scopes.QualifiedName, error) {
	sqn, err := parseACLQualifiedName(c.accessListName)
	return sqn, trace.Wrap(err, "validating access list name as a scope-qualified name")
}

func (c *ACLCommand) memberQualifiedName() (scopes.QualifiedName, error) {
	sqn, err := parseACLQualifiedName(c.memberName)
	return sqn, trace.Wrap(err, "validating member name as a scope-qualified name")
}

func splitACLQualifiedNames(names string) ([]scopes.QualifiedName, error) {
	var sqns []scopes.QualifiedName
	for _, name := range utils.SplitIdentifiers(names) {
		sqn, err := parseACLQualifiedName(strings.TrimSpace(name))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sqns = append(sqns, sqn)
	}
	return sqns, nil
}

// Get will display information about an access list visible to the user.
func (c *ACLCommand) Get(ctx context.Context, client *authclient.Client) error {
	aclName, err := c.accessListQualifiedName()
	if err != nil {
		return trace.Wrap(err)
	}
	accessList, err := client.AccessListClient().GetAccessListV2(ctx, accesslistv1.GetAccessListRequest_builder{
		Scope: aclName.Scope,
		Name:  aclName.Name,
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.displayAccessLists(accessList))
}

// UsersAdd will add a user to an access list.
func (c *ACLCommand) UsersAdd(ctx context.Context, client *authclient.Client) error {
	var expires time.Time
	if c.expires != "" {
		var err error
		expires, err = time.Parse(time.RFC3339, c.expires)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	aclName, err := c.accessListQualifiedName()
	if err != nil {
		return trace.Wrap(err)
	}
	memberName, err := c.memberQualifiedName()
	if err != nil {
		return trace.Wrap(err)
	}

	var membershipKind string
	switch c.memberKind {
	case memberKindList:
		membershipKind = accesslist.MembershipKindList
		if memberName.Scope != "" {
			membershipKind = accesslist.MembershipKindScopedList
		}
	case "", memberKindUser:
		if memberName.Scope != "" {
			return trace.BadParameter("user members cannot be scoped, got %q", c.memberName)
		}
		membershipKind = accesslist.MembershipKindUser
	}

	member, err := accesslist.NewAccessListMemberWithScope(header.Metadata{
		Name: memberName.String(),
	}, accesslist.AccessListMemberSpec{
		AccessList: aclName.String(),
		Name:       memberName.String(),
		Reason:     c.reason,
		Expires:    expires,

		// The following fields will be updated in the backend, so their values here don't matter.
		Joined:         time.Now(),
		AddedBy:        "dummy",
		MembershipKind: membershipKind,
	}, aclName.Scope)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = client.AccessListClient().UpsertAccessListMember(ctx, member)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(c.Stdout, "successfully added member %s to access list %s", memberName.String(), aclName.String())

	return nil
}

// UsersRemove will remove a user to an access list.
func (c *ACLCommand) UsersRemove(ctx context.Context, client *authclient.Client) error {
	aclName, err := c.accessListQualifiedName()
	if err != nil {
		return trace.Wrap(err)
	}
	memberName, err := c.memberQualifiedName()
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.AccessListClient().DeleteAccessListMemberV2(ctx, accesslistv1.DeleteAccessListMemberRequest_builder{
		AccessListScope: aclName.Scope,
		AccessList:      aclName.Name,
		MemberScope:     memberName.Scope,
		MemberName:      memberName.Name,
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(c.Stdout, "successfully removed member %s from access list %s\n", memberName.String(), aclName.String())

	return nil
}

// UsersList will list the users in an access list.
func (c *ACLCommand) UsersList(ctx context.Context, client *authclient.Client) error {
	aclName, err := c.accessListQualifiedName()
	if err != nil {
		return trace.Wrap(err)
	}
	var (
		allMembers []*accesslist.AccessListMember
		nextToken  string
		listErr    error
		members    []*accesslist.AccessListMember
	)

	for {
		members, nextToken, listErr = client.AccessListClient().ListAccessListMembersV2(ctx, accesslistv1.ListAccessListMembersRequest_builder{
			PageToken:       nextToken,
			AccessListScope: aclName.Scope,
			AccessList:      aclName.Name,
		}.Build())
		if listErr != nil {
			return trace.Wrap(listErr)
		}
		allMembers = append(allMembers, members...)
		if nextToken == "" {
			break
		}
	}

	switch c.format {
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(c.Stdout, allMembers))
	case teleport.Text:
		if len(allMembers) == 0 {
			fmt.Fprintf(c.Stdout, "No members found for access list %s.\nYou may not have access to see the members for this list.\n", aclName.String())
			return nil
		}
		fmt.Fprintf(c.Stdout, "Members of %s:\n", aclName.String())
		for _, member := range allMembers {
			memberName, err := accesslists.MemberScopeQualifiedName(member)
			if err != nil {
				return trace.Wrap(err)
			}
			if member.IsList() {
				fmt.Fprintf(c.Stdout, "- (Access List) %s \n", memberName.String())
			} else {
				fmt.Fprintf(c.Stdout, "- %s\n", memberName.String())
			}
		}
		return nil
	default:
		return trace.BadParameter("unsupported output format %q", c.format)
	}
}

func (c *ACLCommand) ReviewsCreate(ctx context.Context, client *authclient.Client) error {
	review, err := c.makeReview()
	if err != nil {
		return trace.Wrap(err)
	}

	_, nextAuditDate, err := client.AccessListClient().CreateAccessListReview(ctx, review)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(c.Stdout, "Successfully submitted review for access list %s\n", c.accessListName)
	fmt.Fprintf(c.Stdout, "Next audit date: %s\n", nextAuditDate.Format(time.DateOnly))
	return nil
}

func (c *ACLCommand) makeReview() (*accesslist.Review, error) {
	aclName, err := c.accessListQualifiedName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var removeMembers []string
	var removeScopedMembers []string
	removedMemberNames, err := splitACLQualifiedNames(c.removeMembers)
	if err != nil {
		return nil, trace.Wrap(err, "parsing removed member name")
	}
	for _, member := range removedMemberNames {
		if member.Scope == "" {
			removeMembers = append(removeMembers, member.String())
		} else {
			removeScopedMembers = append(removeScopedMembers, member.String())
		}
	}
	return accesslist.NewReviewWithScope(
		header.Metadata{
			Name: uuid.NewString(),
		},
		accesslist.ReviewSpec{
			AccessList: aclName.String(),
			Reviewers:  []string{"placeholder"}, // Reviewers is set server-side but API requires it.
			ReviewDate: time.Now(),              // Review date will be set server-side as well.
			Notes:      c.notes,
			Changes: accesslist.ReviewChanges{
				RemovedMembers:       removeMembers,
				ScopedRemovedMembers: removeScopedMembers,
			},
		},
		aclName.Scope)
}

func (c *ACLCommand) ReviewsList(ctx context.Context, client *authclient.Client) error {
	aclName, err := c.accessListQualifiedName()
	if err != nil {
		return trace.Wrap(err)
	}
	reviews, err := stream.Collect(clientutils.Resources(ctx,
		func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.Review, string, error) {
			return client.AccessListClient().ListAccessListReviewsV2(ctx, accesslistv1.ListAccessListReviewsRequest_builder{
				PageSize:        int32(pageSize),
				NextToken:       pageToken,
				AccessListScope: aclName.Scope,
				AccessList:      aclName.Name,
			}.Build())
		}))
	if err != nil {
		return trace.Wrap(err)
	}
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].Spec.ReviewDate.After(reviews[j].Spec.ReviewDate)
	})
	return trace.Wrap(c.displayAccessListReviews(reviews))
}

func (c *ACLCommand) displayAccessListReviews(reviews []*accesslist.Review) error {
	switch c.format {
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(c.Stdout, reviews))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(c.Stdout, reviews))
	case teleport.Text:
		return trace.Wrap(c.displayAccessListReviewsText(reviews))
	}
	return trace.BadParameter("invalid format %q", c.format)
}

func (c *ACLCommand) displayAccessListReviewsText(reviews []*accesslist.Review) error {
	table := asciitable.MakeTable([]string{"ID", "Reviewer", "Review Date", "Removed Members", "Notes"})
	for _, review := range reviews {
		table.AddRow([]string{
			review.GetName(),
			strings.Join(review.Spec.Reviewers, ","),
			review.Spec.ReviewDate.Format(time.DateOnly),
			strings.Join(append(review.Spec.Changes.RemovedMembers, review.Spec.Changes.ScopedRemovedMembers...), ","),
			review.Spec.Notes,
		})
	}
	_, err := fmt.Fprintln(c.Stdout, table.AsBuffer().String())
	return trace.Wrap(err)
}

func (c *ACLCommand) displayAccessLists(accessLists ...*accesslist.AccessList) error {
	switch c.format {
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(c.Stdout, accessLists))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(c.Stdout, accessLists))
	case teleport.Text:
		return trace.Wrap(c.displayAccessListsText(accessLists...))
	}

	// technically unreachable since kingpin validates the EnumVar
	return trace.BadParameter("invalid format %q", c.format)
}

func (c *ACLCommand) displayAccessListsText(accessLists ...*accesslist.AccessList) error {
	table := asciitable.MakeTable([]string{"ID", "Title", "Next Audit", "Granted Roles", "Granted Traits"})
	for _, accessList := range accessLists {
		grantedRoles := append([]string(nil), accessList.GetGrants().Roles...)
		for _, grant := range accessList.GetGrants().ScopedRoles {
			grantedRoles = append(grantedRoles, grant.Role)
		}
		traitStrings := make([]string, 0, len(accessList.GetGrants().Traits))
		for k, values := range accessList.GetGrants().Traits {
			traitStrings = append(traitStrings, fmt.Sprintf("%s:{%s}", k, strings.Join(values, ",")))
		}

		grantedTraits := strings.Join(traitStrings, ",")
		table.AddRow([]string{
			accesslists.ScopeQualifiedName(accessList).String(),
			accessList.Spec.Title,
			accessList.Spec.Audit.NextAuditDate.Format(time.DateOnly),
			strings.Join(grantedRoles, ","),
			grantedTraits,
		})
	}
	_, err := fmt.Fprintln(c.Stdout, table.AsBuffer().String())
	return trace.Wrap(err)
}
