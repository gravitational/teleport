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

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	auth "github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// AppsCommand implements "tctl apps" group of commands.
type AppsCommand struct {
	config *service.Config

	// format is the output format (text, json, or yaml)
	format string

	// appsList implements the "tctl apps ls" subcommand.
	appsList *kingpin.CmdClause
}

// Initialize allows AppsCommand to plug itself into the CLI parser
func (c *AppsCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	apps := app.Command("apps", "Operate on applications registered with the cluster.")
	c.appsList = apps.Command("ls", "List all applications registered with the cluster.")
	c.appsList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default("text").StringVar(&c.format)
}

// TryRun attempts to run subcommands like "apps ls".
func (c *AppsCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.appsList.FullCommand():
		err = c.ListApps(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// ListApps prints the list of applications that have recently sent heartbeats
// to the cluster.
func (c *AppsCommand) ListApps(client auth.ClientI) error {
	servers, err := client.GetAppServers(context.TODO(), defaults.Namespace, resource.SkipValidation())
	if err != nil {
		return trace.Wrap(err)
	}
	coll := &appCollection{servers: servers}

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

const appMessage = `The invite token: %v
This token will expire in %d minutes

Fill out and run this command on a node to make the application available:

> teleport start \
   --roles=%v \
   --token=%v \
   --ca-pin=%v \
   --auth-server=%v \
   --app-name=%-30v ` + "`" + `# Change "%v" to the name of your application.` + "`" + ` \
   --app-uri=%-31v ` + "`" + `# Change "%v" to the address of your application.` + "`" + `

Your application will be available at %v.

Please note:

  - This invitation token will expire in %d minutes.
  - %v must be reachable from the new application service.
  - Update DNS to point %v to the Teleport proxy.
  - Add a TLS certificate for %v to the Teleport proxy under "https_keypairs".
`
