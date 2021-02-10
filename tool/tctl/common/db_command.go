/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"os"
	"text/template"

	"github.com/gravitational/teleport"
	auth "github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
)

// DBCommand implements "tctl db" group of commands.
type DBCommand struct {
	config *service.Config

	// format is the output format (text, json or yaml).
	format string

	// dbList implements the "tctl db ls" subcommand.
	dbList *kingpin.CmdClause
}

// Initialize allows DBCommand to plug itself into the CLI parser.
func (c *DBCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	db := app.Command("db", "Operate on databases registered with the cluster.")
	c.dbList = db.Command("ls", "List all databases registered with the cluster.")
	c.dbList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default("text").StringVar(&c.format)
}

// TryRun attempts to run subcommands like "db ls".
func (c *DBCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.dbList.FullCommand():
		err = c.ListDatabases(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// ListDatabases prints the list of database proxies that have recently sent
// heartbeats to the cluster.
func (c *DBCommand) ListDatabases(client auth.ClientI) error {
	servers, err := client.GetDatabaseServers(context.TODO(), defaults.Namespace, resource.SkipValidation())
	if err != nil {
		return trace.Wrap(err)
	}
	coll := &dbCollection{servers: servers}
	switch c.format {
	case teleport.Text:
		err = coll.writeText(os.Stdout)
	case teleport.JSON:
		err = coll.writeJSON(os.Stdout)
	case teleport.YAML:
		err = coll.writeYAML(os.Stdout)
	default:
		return trace.BadParameter("unknown format %q", c.format)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

var dbMessageTemplate = template.Must(template.New("db").Parse(`The invite token: {{.token}}.
This token will expire in {{.minutes}} minutes.

Fill out and run this command on a node to start proxying the database:

> teleport start \
   --roles={{.roles}} \
   --token={{.token}} \
   --ca-pin={{.ca_pin}} \
   --auth-server={{.auth_server}} \
   --db-name={{.db_name}} \
   --db-protocol={{.db_protocol}} \
   --db-uri={{.db_uri}}

Please note:

  - This invitation token will expire in {{.minutes}} minutes.
  - Database address {{.db_uri}} must be reachable from the new database
    service.
  - When proxying an on-prem database, it must be configured with Teleport CA
    and key pair issued by "tctl auth sign --format=db" command.
  - When proxying an AWS RDS or Aurora database, the region must also be
    specified with --db-aws-region flag.
`))
