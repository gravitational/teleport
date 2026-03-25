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

package common

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/scopes"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// ScopedCommand implements scoped variants of tctl command groups, such as
// `tctl scoped tokens`.
type ScopedCommand struct {
	config *servicecfg.Config
	tokens *ScopedTokensCommand
	Stdout io.Writer

	status *kingpin.CmdClause
}

// Initialize allows ScopedCommand to plug itself into the CLI parser
func (c *ScopedCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	scoped := app.Command("scoped", "Run a subcommand using scoped auth").Alias("scopes")

	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}

	c.tokens = &ScopedTokensCommand{
		Stdout: c.Stdout,
	}

	c.tokens.Initialize(scoped, config)

	c.status = scoped.Command("status", "Show the status of scoped resources.")
}

// TryRun takes the CLI command as an argument (like "scoped tokens") and executes it.
func (c *ScopedCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.status.FullCommand():
		commandFunc = c.Status
	default:
		// may match the token family of commands
		return c.tokens.TryRun(ctx, cmd, clientFunc)
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(commandFunc(ctx, client))
}

// Status shows the status of scoped resources.
func (c *ScopedCommand) Status(ctx context.Context, client *authclient.Client) error {
	// TODO(fspmarshall/scopes): move the calculation of counts server-side and based on
	// scoped access cache. Ideally we want to allow for a paginated view of scope stats
	// with one item/row per scope.  We should also consider how to optionally include
	// additional columns conditional on user's permissions (e.g. number of tokens if
	// user has permission to view tokens... in that case specifically we want readnosecrets
	// to allow seeing the token count without seeing actual tokens).
	type scopeMetricRow struct {
		scope       string
		roles       int
		assignments int
	}

	// aggregate per scope counts
	rows := make(map[string]scopeMetricRow)

	for role, err := range scopedutils.RangeScopedRoles(ctx, client.ScopedAccessServiceClient(), &scopedaccessv1.ListScopedRolesRequest{}) {
		if err != nil {
			return trace.Wrap(err)
		}

		scope := scopes.NormalizeForEquality(role.GetScope())
		row := rows[scope]
		row.scope = scope
		row.roles++
		rows[scope] = row
	}

	for assignment, err := range scopedutils.RangeScopedRoleAssignments(ctx, client.ScopedAccessServiceClient(), &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
		if err != nil {
			return trace.Wrap(err)
		}

		scope := scopes.NormalizeForEquality(assignment.GetScope())
		row := rows[scope]
		row.scope = scope
		row.assignments++
		rows[scope] = row
	}

	// scope sort order differs subtly from lexographic order, so we need a custom sort func here
	sortedScopes := slices.SortedFunc(maps.Keys(rows), scopes.Sort)

	table := asciitable.MakeTable([]string{"Scope", "Roles", "Assignments"})
	for _, scope := range sortedScopes {
		table.AddRow([]string{
			scope,
			fmt.Sprintf("%d", rows[scope].roles),
			fmt.Sprintf("%d", rows[scope].assignments),
		})
	}

	fmt.Fprint(c.Stdout, table.AsBuffer().String())
	return nil
}
