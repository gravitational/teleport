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

package accesslist

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

// Command implements the `tctl acl` family of commands.
type Command struct {
	format string

	ls            *kingpin.CmdClause
	get           *kingpin.CmdClause
	usersAdd      *kingpin.CmdClause
	usersRemove   *kingpin.CmdClause
	usersList     *kingpin.CmdClause
	reviewsCreate *kingpin.CmdClause
	reviewsList   *kingpin.CmdClause
	remove        *kingpin.CmdClause
	create        *kingpin.CmdClause
	update        *kingpin.CmdClause

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

	// Defines the access type with `acl create`.
	// This is called "preset" but user facing docs
	// don't use that term.
	accessType string

	// Used to hold access list metadata.
	title                string
	description          string
	auditFrequency       int
	auditDay             int
	owners               string
	ownerAccessLists     string
	ownerRequiredRoles   string
	ownerRequiredTraits  string
	ownerGrantRoles      string
	ownerGrantTraits     string
	members              string
	memberAccessLists    string
	memberRequiredRoles  string
	memberRequiredTraits string
	memberGrantRoles     string
	memberGrantTraits    string

	// Used to hold resource access related fields.
	nodeLabels         string
	logins             string
	dbLabels           string
	dbUsers            string
	dbNames            string
	kubeLabels         string
	kubeUsers          string
	kubeGroups         string
	appLabels          string
	awsRoleARNs        string
	azureIdentities    string
	gcpServiceAccounts string
	mcpTools           string
	windowsLabels      string
	windowsLogins      string
	gitHubOrgs         string
	awsicAssignments   string

	// Removes "access related roles" from an access list created with an access type:
	// - reviewer/requester role specs get emptied
	// - for long-term access lists, the members grant gets removed
	// - for short-term access lists, no grants are changed (but their allow specs gets emptied)
	removeAccess bool

	// Flags to determine if user set resource access related fields.
	nodeLabelsSet         bool
	loginsSet             bool
	dbLabelsSet           bool
	dbUsersSet            bool
	dbNamesSet            bool
	kubeLabelsSet         bool
	kubeUsersSet          bool
	kubeGroupsSet         bool
	appLabelsSet          bool
	awsRoleARNsSet        bool
	azureIdentitiesSet    bool
	gcpServiceAccountsSet bool
	mcpToolsSet           bool
	windowsLabelsSet      bool
	windowsLoginsSet      bool
	gitHubOrgsSet         bool
	awsicAssignmentsSet   bool

	// Flags to determine if user set these access list metadata fields.
	titleSet                bool
	descriptionSet          bool
	auditFrequencySet       bool
	auditDaySet             bool
	ownersSet               bool
	ownerAccessListsSet     bool
	ownerGrantRolesSet      bool
	ownerGrantTraitsSet     bool
	ownerRequiredRolesSet   bool
	ownerRequiredTraitsSet  bool
	membersSet              bool
	memberAccessListsSet    bool
	memberGrantRolesSet     bool
	memberGrantTraitsSet    bool
	memberRequiredRolesSet  bool
	memberRequiredTraitsSet bool

	// Stdout allows to switch the standard output source. Used in tests.
	Stdout io.Writer
}

const (
	memberKindUser = "user"
	memberKindList = "list"
)

const auditFrequencyText = "Audit recurrence in months (1, 3, 6, or 12)."
const auditDayText = "Day of month for audit (1, 15, or 31)."

const updateHelpText = "Update an existing access list. Each flag you pass replaces that field (no\n" +
	"merge or append), so list-valued flags like --members or --logins overwrite\n" +
	"the whole list; anything you omit is left unchanged.\n\n" +
	"For an access list created with an access type, resource flags (--node-labels, etc.) edit the\n" +
	"supporting roles."

const createHelpText = `Create an access list.

Use --access-type to have Teleport create the access list's supporting
roles for you. The --access-type value controls how members are granted
access. Possible values are:

  standing        members receive persistent access to the resources set below.
  access-request  members must request that access; owners approve.

The supporting roles are built from the resource flags you set. The available
resource flags (grouped by target) are:

  SSH servers          --node-labels, --logins
  Databases            --db-labels, --db-users, --db-names
  Kubernetes           --kubernetes-labels, --kubernetes-users, --kubernetes-groups
  Windows desktops     --windows-labels, --windows-logins
  Git servers          --github-orgs
  AWS Identity Center  --aws-ic-assignments
  Web applications     --app-labels, --aws-role-arns, --azure-identities
                       --gcp-service-accounts, --mcp-tools

If --access-type is omitted, a plain access list is created with whatever
grants you set directly (--member-grant-roles, etc.) — there are no
auto-generated supporting roles.`

// Initialize allows Command to plug itself into the CLI parser
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
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

	c.remove = acl.Command("rm", "Delete an Access List.").Alias("del").Alias("delete")
	c.remove.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.remove.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)

	c.update = acl.Command("update", updateHelpText)
	c.update.Arg("access-list-name", "The Access List name.").Required().StringVar(&c.accessListName)
	c.update.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)
	c.update.Flag("title", "New display name for the access list.").IsSetByUser(&c.titleSet).StringVar(&c.title)
	c.update.Flag("description", "New description.").IsSetByUser(&c.descriptionSet).StringVar(&c.description)
	c.update.Flag("audit-frequency", auditFrequencyText+" Changing this resets the next audit date to now + frequency.").PlaceHolder("6").IsSetByUser(&c.auditFrequencySet).IntVar(&c.auditFrequency)
	c.update.Flag("audit-day", auditDayText+" Changing this resets the next audit date to now + frequency.").PlaceHolder("1").IsSetByUser(&c.auditDaySet).IntVar(&c.auditDay)
	c.update.Flag("owners", "Replace the user owners with this list of usernames or emails.").PlaceHolder("user1,user2,...").IsSetByUser(&c.ownersSet).StringVar(&c.owners)
	c.update.Flag("owner-access-lists", "Replace the access-list owners with this list of access list names.").PlaceHolder("name1,name2,...").IsSetByUser(&c.ownerAccessListsSet).StringVar(&c.ownerAccessLists)
	c.update.Flag("members", "Replace the user members with this list of usernames or emails. Not combinable with non-member update flags.").PlaceHolder("user1,user2,...").IsSetByUser(&c.membersSet).StringVar(&c.members)
	c.update.Flag("member-access-lists", "Replace the nested access-list members with this list of access list names. Not combinable with non-member update flags.").PlaceHolder("name1,name2,...").IsSetByUser(&c.memberAccessListsSet).StringVar(&c.memberAccessLists)
	c.initSharedOwnerMemberFlags(c.update)
	c.initSharedResourceAccessFlags(c.update)

	c.update.Flag("remove-access", "Remove resource access from an access-typed list. Detaches the resource-access roles from the list's grants and from the supporting reviewer/requester roles.").BoolVar(&c.removeAccess)

	c.create = acl.Command("create", createHelpText)
	c.create.Flag("access-type", "How members are granted access: 'standing' (members receive persistent access to the resources described by the resource flags) or 'access-request' (members must request access; owners review). When omitted, a plain access list is created with no auto-generated roles.").EnumVar(&c.accessType, accessTypeLongTerm, accessTypeShortTerm)
	c.create.Flag("format", "Output format.").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)
	c.create.Flag("title", "Display name for the access list.").IsSetByUser(&c.titleSet).StringVar(&c.title)
	c.create.Flag("audit-frequency", auditFrequencyText).Default("6").PlaceHolder("6").IntVar(&c.auditFrequency)
	c.create.Flag("audit-day", auditDayText).Default("1").PlaceHolder("1").IntVar(&c.auditDay)
	c.create.Flag("description", "Optional description.").StringVar(&c.description)
	c.create.Flag("owners", "Usernames or emails who own this access list and review membership.").PlaceHolder("user1,user2,...").IsSetByUser(&c.ownersSet).StringVar(&c.owners)
	c.create.Flag("owner-access-lists", "Access list names to add as owners; their members inherit owner permissions on this access list (nested access lists).").PlaceHolder("name1,name2,...").IsSetByUser(&c.ownerAccessListsSet).StringVar(&c.ownerAccessLists)
	c.create.Flag("members", "Usernames or emails to add as members of the new access list. Members can also be added later with `tctl acl users add`.").PlaceHolder("user1,user2,...").IsSetByUser(&c.membersSet).StringVar(&c.members)
	c.create.Flag("member-access-lists", "Access list names to add as members; their members inherit this list's member grants (nested access lists).").PlaceHolder("name1,name2,...").IsSetByUser(&c.memberAccessListsSet).StringVar(&c.memberAccessLists)
	c.initSharedOwnerMemberFlags(c.create)
	c.initSharedResourceAccessFlags(c.create)

	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
}

func (c *Command) initSharedResourceAccessFlags(cmd *kingpin.CmdClause) {
	// Nodes
	cmd.Flag("node-labels", "Selects SSH servers members may access by label match.").PlaceHolder("key=value,...").IsSetByUser(&c.nodeLabelsSet).StringVar(&c.nodeLabels)
	cmd.Flag("logins", "OS logins members may use to connect to matched SSH servers.").PlaceHolder("login1,login2,...").IsSetByUser(&c.loginsSet).StringVar(&c.logins)
	// Dbs
	cmd.Flag("db-labels", "Selects databases members may access by label match.").PlaceHolder("key=value,...").IsSetByUser(&c.dbLabelsSet).StringVar(&c.dbLabels)
	cmd.Flag("db-users", "Database users members may connect as on matched databases.").PlaceHolder("user1,user2,...").IsSetByUser(&c.dbUsersSet).StringVar(&c.dbUsers)
	cmd.Flag("db-names", "Database names members may connect to on matched databases.").PlaceHolder("name1,name2,...").IsSetByUser(&c.dbNamesSet).StringVar(&c.dbNames)
	// Kubes
	cmd.Flag("kubernetes-labels", "Selects Kubernetes clusters members may access by label match.").PlaceHolder("key=value,...").IsSetByUser(&c.kubeLabelsSet).StringVar(&c.kubeLabels)
	cmd.Flag("kubernetes-users", "Kubernetes users members may impersonate on matched clusters.").PlaceHolder("user1,user2,...").IsSetByUser(&c.kubeUsersSet).StringVar(&c.kubeUsers)
	cmd.Flag("kubernetes-groups", "Kubernetes groups members may impersonate on matched clusters.").PlaceHolder("group1,group2,...").IsSetByUser(&c.kubeGroupsSet).StringVar(&c.kubeGroups)
	// Apps
	cmd.Flag("app-labels", "Selects web apps members may access by label match. For AWS Identity Center apps, use --aws-ic-assignments instead.").PlaceHolder("key=value,...").IsSetByUser(&c.appLabelsSet).StringVar(&c.appLabels)
	cmd.Flag("aws-role-arns", "AWS role ARNs members may assume via matched apps.").PlaceHolder("arn1,arn2,...").IsSetByUser(&c.awsRoleARNsSet).StringVar(&c.awsRoleARNs)
	cmd.Flag("azure-identities", "Azure managed identities members may assume via matched apps.").PlaceHolder("id1,id2,...").IsSetByUser(&c.azureIdentitiesSet).StringVar(&c.azureIdentities)
	cmd.Flag("gcp-service-accounts", "GCP service accounts members may assume via matched apps.").PlaceHolder("account1,account2,...").IsSetByUser(&c.gcpServiceAccountsSet).StringVar(&c.gcpServiceAccounts)
	cmd.Flag("mcp-tools", "MCP tools members may call on matched MCP apps.").PlaceHolder("tool1,tool2,...").IsSetByUser(&c.mcpToolsSet).StringVar(&c.mcpTools)
	// Windows
	cmd.Flag("windows-labels", "Selects Windows desktops members may access by label match.").PlaceHolder("key=value,...").IsSetByUser(&c.windowsLabelsSet).StringVar(&c.windowsLabels)
	cmd.Flag("windows-logins", "Logins members may use to connect to matched Windows desktops.").PlaceHolder("login1,login2,...").IsSetByUser(&c.windowsLoginsSet).StringVar(&c.windowsLogins)
	// GitHub
	cmd.Flag("github-orgs", "Selects git servers members may access by GitHub organization name.").PlaceHolder("org1,org2,...").IsSetByUser(&c.gitHubOrgsSet).StringVar(&c.gitHubOrgs)
	// AWS IC
	cmd.Flag("aws-ic-assignments", "Selects AWS Identity Center apps members may access by AWS account ID + permission set ARN ('accountID^permissionSetARN' pairs).").PlaceHolder("accountID^permSetARN,...").IsSetByUser(&c.awsicAssignmentsSet).StringVar(&c.awsicAssignments)
}

func (c *Command) initSharedOwnerMemberFlags(cmd *kingpin.CmdClause) {
	// Owners
	cmd.Flag("owner-grant-roles", "Roles granted to owners of this access list.").PlaceHolder("role1,role2,...").IsSetByUser(&c.ownerGrantRolesSet).StringVar(&c.ownerGrantRoles)
	cmd.Flag("owner-grant-traits", "Traits granted to owners of this access list.").PlaceHolder("key=value,...").IsSetByUser(&c.ownerGrantTraitsSet).StringVar(&c.ownerGrantTraits)
	cmd.Flag("owner-required-roles", "Roles a user must already have to be an owner of this access list.").PlaceHolder("role1,role2,...").IsSetByUser(&c.ownerRequiredRolesSet).StringVar(&c.ownerRequiredRoles)
	cmd.Flag("owner-required-traits", "Traits a user must already have to be an owner of this access list.").PlaceHolder("key=value,...").IsSetByUser(&c.ownerRequiredTraitsSet).StringVar(&c.ownerRequiredTraits)
	// Members
	cmd.Flag("member-grant-roles", "Roles granted to members of this access list.").PlaceHolder("role1,role2,...").IsSetByUser(&c.memberGrantRolesSet).StringVar(&c.memberGrantRoles)
	cmd.Flag("member-grant-traits", "Traits granted to members of this access list.").PlaceHolder("key=value,...").IsSetByUser(&c.memberGrantTraitsSet).StringVar(&c.memberGrantTraits)
	cmd.Flag("member-required-roles", "Roles a user must already have to be a member of this access list.").PlaceHolder("role1,role2,...").IsSetByUser(&c.memberRequiredRolesSet).StringVar(&c.memberRequiredRoles)
	cmd.Flag("member-required-traits", "Traits a user must already have to be a member of this access list.").PlaceHolder("key=value,...").IsSetByUser(&c.memberRequiredTraitsSet).StringVar(&c.memberRequiredTraits)
}

// TryRun takes the CLI command as an argument (like "acl ls") and executes it.
func (c *Command) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
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
	case c.remove.FullCommand():
		commandFunc = c.Remove
	case c.update.FullCommand():
		commandFunc = c.Update
	case c.create.FullCommand():
		commandFunc = c.Create
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
func (c *Command) List(ctx context.Context, client *authclient.Client) error {
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

func (c *Command) accessListQualifiedName() (scopes.QualifiedName, error) {
	sqn, err := parseACLQualifiedName(c.accessListName)
	return sqn, trace.Wrap(err, "validating access list name as a scope-qualified name")
}

func (c *Command) memberQualifiedName() (scopes.QualifiedName, error) {
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
func (c *Command) Get(ctx context.Context, client *authclient.Client) error {
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
func (c *Command) UsersAdd(ctx context.Context, client *authclient.Client) error {
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
func (c *Command) UsersRemove(ctx context.Context, client *authclient.Client) error {
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
func (c *Command) UsersList(ctx context.Context, client *authclient.Client) error {
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

func (c *Command) ReviewsCreate(ctx context.Context, client *authclient.Client) error {
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

func (c *Command) makeReview() (*accesslist.Review, error) {
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

func (c *Command) ReviewsList(ctx context.Context, client *authclient.Client) error {
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

func (c *Command) displayAccessListReviews(reviews []*accesslist.Review) error {
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

func (c *Command) displayAccessListReviewsText(reviews []*accesslist.Review) error {
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

func (c *Command) displayAccessLists(accessLists ...*accesslist.AccessList) error {
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

func (c *Command) displayAccessListsText(accessLists ...*accesslist.AccessList) error {
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

func (c *Command) collectAllMembers(ctx context.Context, client *authclient.Client, aclName scopes.QualifiedName) ([]*accesslist.AccessListMember, error) {
	return stream.Collect(clientutils.Resources(ctx,
		func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
			return client.AccessListClient().ListAccessListMembersV2(ctx, accesslistv1.ListAccessListMembersRequest_builder{
				PageSize:        int32(pageSize),
				PageToken:       pageToken,
				AccessListScope: aclName.Scope,
				AccessList:      aclName.Name,
			}.Build())
		}))
}
