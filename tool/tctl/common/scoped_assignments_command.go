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

package common

import (
	"context"
	"io"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/itertools/stream"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/services"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
)

// scopedAssignmentsCommand implements `tctl scoped assignments` subcommands.
type scopedAssignmentsCommand struct {
	// list is the 'tctl scoped assignments list' command.
	list scopedAssignmentsListCommand
}

// initialize allows the scoped assignments commands to plug themselves into the CLI parser.
func (c *scopedAssignmentsCommand) initialize(parent *kingpin.CmdClause, stdout io.Writer) {
	assignments := parent.Command("assignments", "Manage scoped role assignments")

	c.list.initialize(assignments, stdout)
}

// TryRun attempts to run subcommands like `scoped assignments list`.
func (c *scopedAssignmentsCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	for _, subCmd := range []RunnableCommand{&c.list} {
		match, err := subCmd.TryRun(ctx, cmd, clientFunc)
		if match || err != nil {
			return match, trace.Wrap(err)
		}
	}
	return false, nil
}

// scopedAssignmentsListCommand implements `tctl scoped assignments list`.
type scopedAssignmentsListCommand struct {
	stdout io.Writer
	cmd    *kingpin.CmdClause

	// format is the output format: text, yaml, or json.
	format string
	// user allows filtering by user to whom assignments apply.
	user string
	// role allows filtering by scoped role name.
	role string
}

// initialize allows the scoped assignments list command to plug itself into the CLI parser.
func (c *scopedAssignmentsListCommand) initialize(parent *kingpin.CmdClause, stdout io.Writer) {
	c.stdout = stdout

	c.cmd = parent.Command("list", "List scoped role assignments").Alias("ls")
	c.cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Short('f').
		Default(teleport.Text).
		EnumVar(&c.format, defaults.DefaultFormats...)
	c.cmd.Flag("user", "Filter by user.").StringVar(&c.user)
	c.cmd.Flag("role", "Filter by assigned role.").StringVar(&c.role)
}

// TryRun attempts to run `scoped assignments list`.
func (c *scopedAssignmentsListCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	if cmd != c.cmd.FullCommand() {
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	defer closeFn(ctx)

	return true, trace.Wrap(c.list(ctx, client.ScopedAccessServiceClient()))

}

// list retrieves scoped role assignments matching the configured filters and writes them.
func (c *scopedAssignmentsListCommand) list(ctx context.Context, client services.ScopedRoleAssignmentReader) error {
	items, err := stream.Collect(scopedutils.RangeScopedRoleAssignments(ctx, client, &scopedaccessv1.ListScopedRoleAssignmentsRequest{
		User: c.user,
		Role: c.role,
	}))
	if err != nil {
		return trace.Wrap(err, "collecting filtered scoped role assignments")
	}

	collection := resources.NewScopedRoleAssignmentCollection(items)
	switch c.format {
	case teleport.Text:
		return collection.WriteText(c.stdout, true)
	case teleport.YAML:
		return writeYAML(collection, c.stdout)
	case teleport.JSON:
		return writeJSON(collection, c.stdout)
	}
	return trace.BadParameter("unsupported format")
}
