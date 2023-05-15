/*
Copyright 2021 Gravitational, Inc.

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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// DesktopCommand implements "tctl desktop" group of commands.
type DesktopCommand struct {
	config *servicecfg.Config

	// format is the output format (text or yaml)
	format string

	// verbose sets whether full table output should be shown for labels
	verbose bool

	// desktopList implements the "tctl desktop ls" subcommand.
	desktopList *kingpin.CmdClause
}

// Initialize allows DesktopCommand to plug itself into the CLI parser
func (c *DesktopCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.config = config

	desktop := app.Command("windows_desktops", "Operate on registered desktops.").Alias("desktops")
	c.desktopList = desktop.Command("ls", "List all desktops registered with the cluster.")
	c.desktopList.Flag("format", "Output format, 'text', 'json' or 'yaml'").Default(teleport.Text).StringVar(&c.format)
	c.desktopList.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&c.verbose)
}

// TryRun attempts to run subcommands like "desktop ls".
func (c *DesktopCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.desktopList.FullCommand():
		err = c.ListDesktop(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// ListDesktop prints the list of desktops that have recently sent heartbeats
// to the cluster.
func (c *DesktopCommand) ListDesktop(ctx context.Context, client auth.ClientI) error {
	desktops, err := client.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return trace.Wrap(err)
	}
	coll := windowsDesktopCollection{
		desktops: desktops,
		verbose:  c.verbose,
	}
	switch c.format {
	case teleport.Text:
		return trace.Wrap(coll.writeText(os.Stdout))
	case teleport.JSON:
		return trace.Wrap(coll.writeJSON(os.Stdout))
	case teleport.YAML:
		return trace.Wrap(coll.writeYAML(os.Stdout))
	default:
		return trace.BadParameter("unknown format %q", c.format)
	}
}

var desktopMessageTemplate = template.Must(template.New("desktop").Parse(`The invite token: {{.token}}
This token will expire in {{.minutes}} minutes.

This token enables Desktop Access.  See https://goteleport.com/docs/desktop-access/
for detailed information on configuring Teleport Desktop Access with this token.

`))
