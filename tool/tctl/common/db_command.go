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
	"text/template"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
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
	filterByStatus string

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

	statusFlagDescription := fmt.Sprintf("If specified, list only databases with a specific status (%s).", strings.Join(dbStatuses, ", "))
	c.dbList.Flag("status", statusFlagDescription).EnumVar(&c.filterByStatus, dbStatuses...)
	c.dbList.Alias(`
Examples:
  Search databases with keywords:
  $ tctl db ls --search foo,bar

  Filter databases with labels:
  $ tctl db ls key1=value1,key2=value2

  Find databases that failed health check
  $ tctl db ls --status unhealthy

  Find dynamic database resources that are not claimed by any database service
  $ tctl db ls --status unclaimed`)
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

func (c *DBCommand) listClaimedDatabases(ctx context.Context, clt authclient.ClientI) ([]types.DatabaseServer, error) {
	labels, err := libclient.ParseLabelSpec(c.labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Add extra status filter if necessary.
	predicateExpr := c.predicateExpr
	if c.filterByStatus != "" && c.filterByStatus != dbStatusUnclaimed {
		statusFilterExpr := fmt.Sprintf("health.status == \"%s\"", c.filterByStatus)
		predicateExpr = common.MakePredicateConjunction(predicateExpr, statusFilterExpr)
	}

	servers, err := client.GetAllResources[types.DatabaseServer](ctx, clt, &proto.ListResourcesRequest{
		ResourceType:        types.KindDatabaseServer,
		Labels:              labels,
		PredicateExpression: predicateExpr,
		SearchKeywords:      libclient.ParseSearchKeywords(c.searchKeywords, ','),
	})
	if err != nil {
		if utils.IsPredicateError(err) {
			return nil, trace.Wrap(utils.PredicateError{Err: err})
		}
		return nil, trace.Wrap(err)
	}
	return servers, nil
}

func (c *DBCommand) listUnclaimedDatabases(ctx context.Context, clt services.DatabaseGetter, claimedDBs []types.DatabaseServer) ([]types.DatabaseServer, error) {
	labels, err := libclient.ParseLabelSpec(c.labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(okraport) DELETE IN v21.0.0, replace with regular Collect
	allDBs, err := clientutils.CollectWithFallback(ctx, clt.ListDatabases, clt.GetDatabases)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Apply command filters on client side.
	// TODO(greedy52) implement resource filtering on the backend.
	filter, err := services.MatchResourceFilterFromListResourceRequest(&proto.ListResourcesRequest{
		ResourceType:        types.KindDatabase,
		Labels:              labels,
		SearchKeywords:      libclient.ParseSearchKeywords(c.searchKeywords, ','),
		PredicateExpression: c.predicateExpr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	claimedDBNames := make(map[string]struct{}, len(claimedDBs))
	for name := range types.ResourceNames(claimedDBs) {
		claimedDBNames[name] = struct{}{}
	}

	var unclaimed []types.DatabaseServer
	for _, db := range allDBs {
		if _, claimed := claimedDBNames[db.GetName()]; claimed {
			continue
		}
		if match, err := services.MatchResourceByFilters(db, filter, nil); err != nil {
			return nil, trace.Wrap(err)
		} else if !match {
			continue
		}

		dbServer, err := toUnclaimedDatabaseServer(db)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		unclaimed = append(unclaimed, dbServer)
	}
	return unclaimed, nil
}

func (c *DBCommand) listDatabases(ctx context.Context, clt authclient.ClientI) ([]types.DatabaseServer, error) {
	claimed, err := c.listClaimedDatabases(ctx, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch c.filterByStatus {
	case "":
		unclaimed, err := c.listUnclaimedDatabases(ctx, clt, claimed)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return append(unclaimed, claimed...), nil

	case dbStatusUnclaimed:
		return c.listUnclaimedDatabases(ctx, clt, claimed)

	default:
		return claimed, nil
	}
}

// ListDatabases prints the list of database proxies that have recently sent
// heartbeats to the cluster.
func (c *DBCommand) ListDatabases(ctx context.Context, clt *authclient.Client) error {
	servers, err := c.listDatabases(ctx, clt)
	if err != nil {
		return trace.Wrap(err)
	}

	coll := &databaseServerCollection{
		servers:             servers,
		allowStatusFootnote: c.filterByStatus == "",
	}
	switch c.format {
	case teleport.Text:
		return trace.Wrap(coll.WriteText(os.Stdout, c.verbose))
	case teleport.JSON:
		return trace.Wrap(coll.writeJSON(os.Stdout))
	case teleport.YAML:
		return trace.Wrap(coll.writeYAML(os.Stdout))
	default:
		return trace.BadParameter("unknown format %q", c.format)
	}
}

func toUnclaimedDatabaseServer(db types.Database) (types.DatabaseServer, error) {
	dbV3, ok := db.(*types.DatabaseV3)
	if !ok {
		return nil, trace.BadParameter("expected types.DatabaseV3 but got %T", db)
	}
	dbServer, err := types.NewDatabaseServerV3(
		types.Metadata{
			Name: db.GetName(),
		}, types.DatabaseServerSpecV3{
			Hostname: "placeholder",
			HostID:   "placeholder",
			Database: dbV3,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dbServer.Spec.Hostname = ""
	dbServer.Spec.HostID = ""
	dbServer.Spec.Version = ""
	dbServer.SetTargetHealthStatus(dbStatusUnclaimed)
	return dbServer, nil
}

const (
	// dbStatusUnclaimed represents a database status for dynamic database
	// resources that are not claimed by any database service. note that this is
	// not an "official" target health status but a special status just used for
	// "tctl db ls".
	dbStatusUnclaimed = "unclaimed"
)

var dbStatuses = []string{
	string(types.TargetHealthStatusHealthy),
	string(types.TargetHealthStatusUnhealthy),
	string(types.TargetHealthStatusUnknown),
	dbStatusUnclaimed,
}

func maybeAddDBStatusFilter(predicateExpr, filterByStatus string) string {
	statusFilterExpr := ""
	if filterByStatus != "" && filterByStatus != dbStatusUnclaimed {
		statusFilterExpr = "health.status == \"%s\""
	}
	return common.MakePredicateConjunction(predicateExpr, statusFilterExpr)
}

var dbMessageTemplate = template.Must(template.New("db").Parse(`The invite token: {{.token}}
This token will expire in {{.minutes}} minutes.

Generate the configuration and start a Teleport agent using it:

> teleport db configure create \
   --token={{.token}} \{{range .ca_pins}}
   --ca-pin={{.}} \{{end}}
   --proxy={{.proxy_server}} \
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
