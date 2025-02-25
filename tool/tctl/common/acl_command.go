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
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// ACLCommand implements the `tctl acl` family of commands.
type ACLCommand struct {
	format string

	ls          *kingpin.CmdClause
	get         *kingpin.CmdClause
	usersAdd    *kingpin.CmdClause
	usersRemove *kingpin.CmdClause
	usersList   *kingpin.CmdClause

	// Used for managing a particular access list.
	accessListName string
	// Used to add an access list to another one
	memberKind string

	// Used for managing membership to an access list.
	userName string
	expires  string
	reason   string
}

const (
	memberKindUser = "user"
	memberKindList = "list"
)

// Initialize allows ACLCommand to plug itself into the CLI parser
func (c *ACLCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	acl := app.Command("acl", "Manage access lists.").Alias("access-lists")

	c.ls = acl.Command("ls", "List cluster access lists.")
	c.ls.Flag("format", "Output format, 'yaml', 'json', or 'text'").Default(teleport.YAML).EnumVar(&c.format, teleport.YAML, teleport.JSON, teleport.Text)

	c.get = acl.Command("get", "Get detailed information for an access list.")
	c.get.Arg("access-list-name", "The access list name.").Required().StringVar(&c.accessListName)
	c.get.Flag("format", "Output format, 'yaml', 'json', or 'text'").Default(teleport.YAML).EnumVar(&c.format, teleport.YAML, teleport.JSON, teleport.Text)

	users := acl.Command("users", "Manage user membership to access lists.")

	c.usersAdd = users.Command("add", "Add a user to an access list.")
	c.usersAdd.Flag("kind", "Access list member kind, 'user' or 'list'").Default(memberKindUser).EnumVar(&c.memberKind, memberKindUser, memberKindList)
	c.usersAdd.Arg("access-list-name", "The access list name.").Required().StringVar(&c.accessListName)
	c.usersAdd.Arg("user", "The user to add to the access list.").Required().StringVar(&c.userName)
	c.usersAdd.Arg("expires", "When the user's access expires (must be in RFC3339). Defaults to the expiration time of the access list.").StringVar(&c.expires)
	c.usersAdd.Arg("reason", "The reason the user has been added to the access list. Defaults to empty.").StringVar(&c.reason)

	c.usersRemove = users.Command("rm", "Remove a user from an access list.")
	c.usersRemove.Arg("access-list-name", "The access list name.").Required().StringVar(&c.accessListName)
	c.usersRemove.Arg("user", "The user to remove from the access list.").Required().StringVar(&c.userName)

	c.usersList = users.Command("ls", "List users that are members of an access list.")
	c.usersList.Arg("access-list-name", "The access list name.").Required().StringVar(&c.accessListName)
	c.usersList.Flag("format", "Output format 'json', or 'text'").Default(teleport.Text).EnumVar(&c.format, teleport.JSON, teleport.Text)
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
	var nextKey string
	for {
		var page []*accesslist.AccessList
		var err error
		page, nextKey, err = client.AccessListClient().ListAccessLists(ctx, 0, nextKey)
		if err != nil {
			return trace.Wrap(err)
		}

		accessLists = append(accessLists, page...)

		if nextKey == "" {
			break
		}
	}

	if len(accessLists) == 0 && c.format == teleport.Text {
		fmt.Println("no access lists")
		return nil
	}

	return trace.Wrap(displayAccessLists(c.format, accessLists...))
}

// Get will display information about an access list visible to the user.
func (c *ACLCommand) Get(ctx context.Context, client *authclient.Client) error {
	accessList, err := client.AccessListClient().GetAccessList(ctx, c.accessListName)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(displayAccessLists(c.format, accessList))
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

	fmt.Printf("successfully added user %s to access list %s", c.userName, c.accessListName)

	return nil
}

// UsersRemove will remove a user to an access list.
func (c *ACLCommand) UsersRemove(ctx context.Context, client *authclient.Client) error {
	err := client.AccessListClient().DeleteAccessListMember(ctx, c.accessListName, c.userName)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("successfully removed user %s from access list %s\n", c.userName, c.accessListName)

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
		return trace.Wrap(utils.WriteJSONArray(os.Stdout, allMembers))
	case teleport.Text:
		if len(allMembers) == 0 {
			fmt.Printf("No members found for access list %s.\nYou may not have access to see the members for this list.\n", c.accessListName)
			return nil
		}
		fmt.Printf("Members of %s:\n", c.accessListName)
		for _, member := range allMembers {
			if member.Spec.MembershipKind == accesslist.MembershipKindList {
				fmt.Printf("- (Access List) %s \n", member.Spec.Name)
			} else {
				fmt.Printf("- %s\n", member.Spec.Name)
			}
		}
		return nil
	default:
		return trace.BadParameter("unsupported output format %q", c.format)
	}
}

func displayAccessLists(format string, accessLists ...*accesslist.AccessList) error {
	switch format {
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(os.Stdout, accessLists))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(os.Stdout, accessLists))
	case teleport.Text:
		return trace.Wrap(displayAccessListsText(accessLists...))
	}

	// technically unreachable since kingpin validates the EnumVar
	return trace.BadParameter("invalid format %q", format)
}

func displayAccessListsText(accessLists ...*accesslist.AccessList) error {
	table := asciitable.MakeTable([]string{"ID", "Review Frequency", "Review Day Of Month", "Granted Roles", "Granted Traits"})
	for _, accessList := range accessLists {
		grantedRoles := strings.Join(accessList.GetGrants().Roles, ",")
		traitStrings := make([]string, 0, len(accessList.GetGrants().Traits))
		for k, values := range accessList.GetGrants().Traits {
			traitStrings = append(traitStrings, fmt.Sprintf("%s:{%s}", k, strings.Join(values, ",")))
		}

		grantedTraits := strings.Join(traitStrings, ",")
		table.AddRow([]string{
			accessList.GetName(),
			accessList.Spec.Audit.Recurrence.Frequency.String(),
			accessList.Spec.Audit.Recurrence.DayOfMonth.String(),
			grantedRoles,
			grantedTraits,
		})
	}
	_, err := fmt.Println(table.AsBuffer().String())
	return trace.Wrap(err)
}
