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

// AppsCommand implements "tctl apps" group of commands.
type AppsCommand struct {
	config *servicecfg.Config

	// format is the output format (text, json, or yaml)
	format string

	searchKeywords string
	predicateExpr  string
	labels         string

	// verbose sets whether full table output should be shown for labels
	verbose bool

	// appsList implements the "tctl apps ls" subcommand.
	appsList *kingpin.CmdClause
}

// Initialize allows AppsCommand to plug itself into the CLI parser
func (c *AppsCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	apps := app.Command("apps", "Operate on applications registered with the cluster.")
	c.appsList = apps.Command("ls", "List all applications registered with the cluster.")
	c.appsList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default(teleport.Text).StringVar(&c.format)
	c.appsList.Arg("labels", labelHelp).StringVar(&c.labels)
	c.appsList.Flag("search", searchHelp).StringVar(&c.searchKeywords)
	c.appsList.Flag("query", queryHelp).StringVar(&c.predicateExpr)
	c.appsList.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&c.verbose)
}

// TryRun attempts to run subcommands like "apps ls".
func (c *AppsCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.appsList.FullCommand():
		commandFunc = c.ListApps
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

// ListApps prints the list of applications that have recently sent heartbeats
// to the cluster.
func (c *AppsCommand) ListApps(ctx context.Context, clt *authclient.Client) error {
	labels, err := libclient.ParseLabelSpec(c.labels)
	if err != nil {
		return trace.Wrap(err)
	}

	var servers []types.AppServer
	resources, err := client.GetResourcesWithFilters(ctx, clt, proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		Labels:              labels,
		PredicateExpression: c.predicateExpr,
		SearchKeywords:      libclient.ParseSearchKeywords(c.searchKeywords, ','),
	})
	switch {
	case err != nil:
		if utils.IsPredicateError(err) {
			return trace.Wrap(utils.PredicateError{Err: err})
		}
		return trace.Wrap(err)
	default:
		servers, err = types.ResourcesWithLabels(resources).AsAppServers()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	coll := &appServerCollection{servers: servers}

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

var appMessageTemplate = template.Must(template.New("app").Parse(`The invite token: {{.token}}
This token will expire in {{.minutes}} minutes.

Fill out and run this command on a node to make the application available:

> teleport app start \
   --token={{.token}} \{{range .ca_pins}}
   --ca-pin={{.}} \{{end}}
   --auth-server={{.auth_server}} \
   --name={{printf "%-30v" .app_name}} ` + "`" + `# Change "{{.app_name}}" to the name of your application.` + "`" + ` \
   --uri={{printf "%-31v" .app_uri}} ` + "`" + `# Change "{{.app_uri}}" to the address of your application.` + "`" + `

Your application will be available at {{.app_public_addr}}.

Please note:

  - This invitation token will expire in {{.minutes}} minutes.
  - {{.auth_server}} must be reachable from the new application service.
  - Update DNS to point {{.app_public_addr}} to the Teleport proxy.
  - Add a TLS certificate for {{.app_public_addr}} to the Teleport proxy under "https_keypairs".
`))
