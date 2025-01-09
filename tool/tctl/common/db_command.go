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
	"os"
	"text/template"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// DBCommand implements "tctl db" group of commands.
type DBCommand struct {
	config *servicecfg.Config

	// format is the output format (text, json or yaml).
	format string

	searchKeywords string
	predicateExpr  string
	labels         string

	// verbose sets whether full table output should be shown for labels
	verbose bool

	// dbList implements the "tctl db ls" subcommand.
	dbList *kingpin.CmdClause
}

// Initialize allows DBCommand to plug itself into the CLI parser.
func (c *DBCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	db := app.Command("db", "Operate on databases registered with the cluster.")
	c.dbList = db.Command("ls", "List all databases registered with the cluster.")
	c.dbList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default(teleport.Text).StringVar(&c.format)
	c.dbList.Arg("labels", labelHelp).StringVar(&c.labels)
	c.dbList.Flag("search", searchHelp).StringVar(&c.searchKeywords)
	c.dbList.Flag("query", queryHelp).StringVar(&c.predicateExpr)
	c.dbList.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&c.verbose)
}

// TryRun attempts to run subcommands like "db ls".
func (c *DBCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.dbList.FullCommand():
		commandFunc = c.ListDatabases
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

// ListDatabases prints the list of database proxies that have recently sent
// heartbeats to the cluster.
func (c *DBCommand) ListDatabases(ctx context.Context, clt *authclient.Client) error {
	labels, err := libclient.ParseLabelSpec(c.labels)
	if err != nil {
		return trace.Wrap(err)
	}

	servers, err := client.GetAllResources[types.DatabaseServer](ctx, clt, &proto.ListResourcesRequest{
		ResourceType:        types.KindDatabaseServer,
		Labels:              labels,
		PredicateExpression: c.predicateExpr,
		SearchKeywords:      libclient.ParseSearchKeywords(c.searchKeywords, ','),
	})
	if err != nil {
		if utils.IsPredicateError(err) {
			return trace.Wrap(utils.PredicateError{Err: err})
		}
		return trace.Wrap(err)
	}

	coll := &databaseServerCollection{servers: servers}
	switch c.format {
	case teleport.Text:
		return trace.Wrap(coll.writeText(os.Stdout, c.verbose))
	case teleport.JSON:
		return trace.Wrap(coll.writeJSON(os.Stdout))
	case teleport.YAML:
		return trace.Wrap(coll.writeYAML(os.Stdout))
	default:
		return trace.BadParameter("unknown format %q", c.format)
	}
}

var dbMessageTemplate = template.Must(template.New("db").Parse(`The invite token: {{.token}}
This token will expire in {{.minutes}} minutes.

Generate the configuration and start a Teleport agent using it:

> teleport db configure create \
   --token={{.token}} \{{range .ca_pins}}
   --ca-pin={{.}} \{{end}}
   --proxy={{.auth_server}} \
   --name={{.db_name}} \
   --protocol={{.db_protocol}} \
   --uri={{.db_uri}} \
   --output file:///etc/teleport.yaml

> teleport start -c /etc/teleport.yaml

Please note:

  - This invitation token will expire in {{.minutes}} minutes.
  - Database address {{.db_uri}} must be reachable from the new database
    service.
  - When proxying an on-prem database, it must be configured with Teleport CA
    and key pair issued by "tctl auth sign --format=db" command.
  - When proxying an AWS RDS or Aurora database, the region must also be
    specified with --db-aws-region flag.
`))
