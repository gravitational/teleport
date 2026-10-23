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

package discovery

import (
	"context"
	"io"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/discoverydoctor"
)

type databasesArgs struct {
	cmd          *kingpin.CmdClause
	format       string
	failuresOnly bool
}

func (d *databasesArgs) initDatabases(app *kingpin.CmdClause) {
	databasesCmd := app.Command("databases", `Report the availability of automatically discovered AWS databases registered as
Teleport database resources.
The command correlates these resources with Database Service heartbeats and open
RDS IAM authentication issue tasks.

A database is Offline if no Database Service heartbeat exists, a matching open
RDS IAM authentication issue exists, or all serving Database Service instances
report an unhealthy target. Otherwise, it is Online. The Details column describes
the issues that cause an Offline status. A database can have multiple issues.

The Time column shows the most recent timestamp among discovery syncs from
matching open RDS issue tasks and target health transitions, in UTC. It is blank
when neither is available.

Missing or unknown target health information does not by itself mark a database
as Offline. An Online status does not verify database user provisioning, user
permissions, or successful authentication.

The command only checks database resources already registered with Teleport and
visible to the caller. It does not report cloud databases that have not been
registered. With --failures-only, an empty result means no failures were reported
for the databases examined.

RDS discovery issue checks require read and list permissions on user_task
resources. If access is denied, these checks are skipped and the command uses
only Database Service heartbeats and target health. An Online status in that case
does not confirm that there are no RDS discovery issues.`)
	databasesCmd.Alias(`
Examples:

  List discovered AWS databases:
  $ tctl discovery databases

  Only show databases with enrollment failures:
  $ tctl discovery databases --failures-only

  Output as JSON:
  $ tctl discovery databases --format=json
`)

	databasesCmd.Flag("format", "Output format.").
		Default(teleport.Text).
		EnumVar(&d.format, teleport.Text, teleport.JSON, teleport.YAML)
	databasesCmd.Flag("failures-only", "Only show databases with enrollment failures.").
		BoolVar(&d.failuresOnly)

	d.cmd = databasesCmd
}

func (d *databasesArgs) run(ctx context.Context, clt discoveryClient, w io.Writer) error {
	diagnostics, err := discoverydoctor.DiagnoseDatabases(ctx, clt, clt.UserTasksClient(), d.failuresOnly)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(renderDatabases(w, diagnostics, d.format))
}
