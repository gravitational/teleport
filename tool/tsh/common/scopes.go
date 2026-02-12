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
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
)

// scopesCommands is a set of commands for viewing and managing Teleport scopes.
type scopesCommands struct {
	ls *scopesLSCommand
}

// newScopesCommand registers a new set of scopes commands with the provided app.
func newScopesCommand(app *kingpin.Application) scopesCommands {
	scopes := app.Command("scopes", "View and manage Teleport scopes.").Alias("scoped")
	cmds := scopesCommands{
		ls: newScopesLSCommand(scopes),
	}
	return cmds
}

// scopesLSCommand implements `tsh scopes ls` command, which is intended to support users
// in discovering which scopes they have been granted access to. Notably, this command does
// not actually return all scopes that are extant/in-use, or even all scopes that are extant/in-use
// for which the user's privileges might grant access.  Instead, it returns only those scopes
// *at which* the user is directly assigned privileges.  So, for example, if a user has access
// to nodes in /staging/east and /staging/west, but said access is determined by a role assigned
// at /staging, this command will only show /staging. The intent is not to provide a comprehensive
// overview, but rather to provide the set of most obvious targets for `tsh login --scope=...`.
type scopesLSCommand struct {
	*kingpin.CmdClause
	verbose bool
}

// newScopesLSCommand registers a new `tsh scopes ls` command.
func newScopesLSCommand(parent *kingpin.CmdClause) *scopesLSCommand {
	c := &scopesLSCommand{
		CmdClause: parent.Command("ls", "List scopes at which user has assigned privileges."),
	}
	c.Flag("verbose", "Show table with details of per-scope privileges.").Short('v').BoolVar(&c.verbose)
	return c
}

func (c *scopesLSCommand) run(cf *CLIConf) error {
	ctx, cancel := context.WithCancel(cf.Context)
	defer cancel()

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var assignments []*scopedaccessv1.ScopedRoleAssignment
	if err := tc.WithRootClusterClient(ctx, func(clt authclient.ClientI) error {
		assignments, err = stream.Collect(scopedutils.RangeScopedRoleAssignments(ctx, clt.ScopedAccessServiceClient(), &scopedaccessv1.ListScopedRoleAssignmentsRequest{
			// note that we are using the AllCallerAssignments flag here rather than just looking for
			// our assignments by username. This flag suppresses standard scope-pinning, which allows
			// us to see assignments in parent/orthogonal scopes. Generally, scoped commands only show
			// the subset of state subject to the currently pinned scope, but the purpose of 'tsh scopes ls'
			// is specifically to discover potential 'tsh login --scope=...' targets, so we want to see
			// everything regardless of current scope.
			AllCallerAssignments: true,
		}))
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}

	if c.verbose {
		// in verbose mode, we produce a table showing each scope and the roles
		// assigned at that scope so that users can get a sense of the specific
		// privileges they have at each scope.
		type assignmentTableRow struct {
			scope string
			roles []string
		}

		var rows []*assignmentTableRow
		for _, assignment := range assignments {
		SubAssignments:
			for _, subAssignment := range assignment.GetSpec().GetAssignments() {
				for _, row := range rows {
					if scopes.Compare(row.scope, subAssignment.GetScope()) == scopes.Equivalent {
						row.roles = append(row.roles, subAssignment.GetRole())
						continue SubAssignments
					}
				}
				rows = append(rows, &assignmentTableRow{
					scope: subAssignment.GetScope(),
					roles: []string{subAssignment.GetRole()},
				})
			}
		}

		// scope sort order differs subtly from lexographic order, so we need a custom sort func here
		slices.SortFunc(rows, func(a, b *assignmentTableRow) int {
			return scopes.Sort(a.scope, b.scope)
		})

		table := asciitable.MakeTable([]string{"Scope", "Roles"})
		for _, row := range rows {
			slices.Sort(row.roles)
			table.AddRow([]string{
				row.scope,
				strings.Join(row.roles, ", "),
			})
		}

		fmt.Fprint(cf.Stdout(), table.AsBuffer().String())
		return nil
	}

	// when not in verbose mode, the default output is a simple newline-delimited
	// list of scopes. this keeps things simple, and has the nice upside of producing
	// an output compatible with visualization tools like `tree` when piped appropriately.
	var assignedScopes []string
	for _, assignment := range assignments {
		for _, subAssignment := range assignment.GetSpec().GetAssignments() {
			assignedScopes = append(assignedScopes, subAssignment.GetScope())
		}
	}

	// apply canonical sorting and deduplication
	slices.SortFunc(assignedScopes, scopes.Sort)
	assignedScopes = slices.CompactFunc(assignedScopes, func(a, b string) bool {
		return scopes.Compare(a, b) == scopes.Equivalent
	})

	for _, scope := range assignedScopes {
		fmt.Fprintln(cf.Stdout(), scope)
	}

	return nil
}
