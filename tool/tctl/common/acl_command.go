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
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
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
	userName string
	expires  string
	reason   string

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
	c.ls.Flag("format", "Output format, 'yaml', 'json', or 'text'").Default(teleport.YAML).EnumVar(&c.format, teleport.YAML, teleport.JSON, teleport.Text)
	c.ls.Flag("review-only", "List only access lists that are due for review within the next 2 weeks or past due").BoolVar(&c.reviewOnly)

	c.get = acl.Command("get", "Get detailed information for an Access List.")
	c.get.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.get.Flag("format", "Output format, 'yaml', 'json', or 'text'").Default(teleport.YAML).EnumVar(&c.format, teleport.YAML, teleport.JSON, teleport.Text)

	users := acl.Command("users", "Manage user membership to Access Lists.")

	c.usersAdd = users.Command("add", "Add a user to an Access List.")
	c.usersAdd.Flag("kind", "Access list member kind, 'user' or 'list'").Default(memberKindUser).EnumVar(&c.memberKind, memberKindUser, memberKindList)
	c.usersAdd.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.usersAdd.Arg("user", "The user to add to the Access List.").Required().StringVar(&c.userName)
	c.usersAdd.Arg("expires", "When the user's access expires (must be in RFC3339). Defaults to the expiration time of the Access List.").StringVar(&c.expires)
	c.usersAdd.Arg("reason", "The reason the user has been added to the Access List. Defaults to empty.").StringVar(&c.reason)

	c.usersRemove = users.Command("rm", "Remove a user from an Access List.")
	c.usersRemove.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.usersRemove.Arg("user", "The user to remove from the Access List.").Required().StringVar(&c.userName)

	c.usersList = users.Command("ls", "List users that are members of an Access List.")
	c.usersList.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.usersList.Flag("format", "Output format 'json', or 'text'").Default(teleport.Text).EnumVar(&c.format, teleport.JSON, teleport.Text)

	reviews := acl.Command("reviews", "Manage access list reviews.")

	c.reviewsCreate = reviews.Command("create", "Submit a new review for a given access list.")
	c.reviewsCreate.Arg("access-list-name", "The access list name to submit review for.").Required().StringVar(&c.accessListName)
	c.reviewsCreate.Flag("notes", "Optional review notes.").StringVar(&c.notes)
	c.reviewsCreate.Flag("remove-members", "Comma-separated list of members to remove as part of this review.").StringVar(&c.removeMembers)

	c.reviewsList = reviews.Command("ls", "List past audit history for a given access list.")
	c.reviewsList.Arg("access-list-name", "The access list name to fetch review history for.").Required().StringVar(&c.accessListName)
	c.reviewsList.Flag("format", "Output format 'yaml', 'json', or 'text'").Default(teleport.Text).EnumVar(&c.format, teleport.YAML, teleport.JSON, teleport.Text)

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
		accessLists, err = stream.Collect(clientutils.Resources(ctx, client.AccessListClient().ListAccessLists))
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

// Get will display information about an access list visible to the user.
func (c *ACLCommand) Get(ctx context.Context, client *authclient.Client) error {
	accessList, err := client.AccessListClient().GetAccessList(ctx, c.accessListName)
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

	var membershipKind string
	switch c.memberKind {
	case memberKindList:
		membershipKind = accesslist.MembershipKindList
	case "", memberKindUser:
		membershipKind = accesslist.MembershipKindUser
	}

	member, err := accesslist.NewAccessListMember(header.Metadata{
		Name: c.userName,
	}, accesslist.AccessListMemberSpec{
		AccessList: c.accessListName,
		Name:       c.userName,
		Reason:     c.reason,
		Expires:    expires,

		// The following fields will be updated in the backend, so their values here don't matter.
		Joined:         time.Now(),
		AddedBy:        "dummy",
		MembershipKind: membershipKind,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = client.AccessListClient().UpsertAccessListMember(ctx, member)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(c.Stdout, "successfully added user %s to access list %s", c.userName, c.accessListName)

	return nil
}

// UsersRemove will remove a user to an access list.
func (c *ACLCommand) UsersRemove(ctx context.Context, client *authclient.Client) error {
	err := client.AccessListClient().DeleteAccessListMember(ctx, c.accessListName, c.userName)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(c.Stdout, "successfully removed user %s from access list %s\n", c.userName, c.accessListName)

	return nil
}

// UsersList will list the users in an access list.
func (c *ACLCommand) UsersList(ctx context.Context, client *authclient.Client) error {
	var (
		allMembers []*accesslist.AccessListMember
		nextToken  string
		err        error
		members    []*accesslist.AccessListMember
	)

	for {
		members, nextToken, err = client.AccessListClient().ListAccessListMembers(ctx, c.accessListName, 0, nextToken)
		if err != nil {
			return trace.Wrap(err)
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
			fmt.Fprintf(c.Stdout, "No members found for access list %s.\nYou may not have access to see the members for this list.\n", c.accessListName)
			return nil
		}
		fmt.Fprintf(c.Stdout, "Members of %s:\n", c.accessListName)
		for _, member := range allMembers {
			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				fmt.Fprintf(c.Stdout, "- (Access List) %s \n", member.Spec.Name)
			} else {
				fmt.Fprintf(c.Stdout, "- %s\n", member.Spec.Name)
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
	var removeMembers []string
	for _, member := range strings.Split(c.removeMembers, ",") {
		member = strings.TrimSpace(member)
		if member != "" {
			removeMembers = append(removeMembers, member)
		}
	}
	return accesslist.NewReview(
		header.Metadata{
			Name: uuid.NewString(),
		},
		accesslist.ReviewSpec{
			AccessList: c.accessListName,
			Reviewers:  []string{"placeholder"}, // Reviewers is set server-side but API requires it.
			ReviewDate: time.Now(),              // Review date will be set server-side as well.
			Notes:      c.notes,
			Changes: accesslist.ReviewChanges{
				RemovedMembers: removeMembers,
			},
		})
}

func (c *ACLCommand) ReviewsList(ctx context.Context, client *authclient.Client) error {
	reviews, err := stream.Collect(clientutils.Resources(ctx,
		func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.Review, string, error) {
			return client.AccessListClient().ListAccessListReviews(ctx, c.accessListName, pageSize, pageToken)
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
			strings.Join(review.Spec.Changes.RemovedMembers, ","),
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
		grantedRoles := strings.Join(accessList.GetGrants().Roles, ",")
		traitStrings := make([]string, 0, len(accessList.GetGrants().Traits))
		for k, values := range accessList.GetGrants().Traits {
			traitStrings = append(traitStrings, fmt.Sprintf("%s:{%s}", k, strings.Join(values, ",")))
		}

		grantedTraits := strings.Join(traitStrings, ",")
		table.AddRow([]string{
			accessList.GetName(),
			accessList.Spec.Title,
			accessList.Spec.Audit.NextAuditDate.Format(time.DateOnly),
			grantedRoles,
			grantedTraits,
		})
	}
	_, err := fmt.Fprintln(c.Stdout, table.AsBuffer().String())
	return trace.Wrap(err)
}
