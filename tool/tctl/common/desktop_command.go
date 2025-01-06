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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
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

	// desktopBootstrap implements the "tctl desktop bootstrap" subcommand.
	desktopBootstrap *kingpin.CmdClause
}

// Initialize allows DesktopCommand to plug itself into the CLI parser
func (c *DesktopCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	desktop := app.Command("desktop", "Operate on registered desktops.").Alias("desktops").Alias("windows_desktop").Alias("windows_desktops")

	c.desktopList = desktop.Command("ls", "List all desktops registered with the cluster.")
	c.desktopList.Flag("format", "Output format, 'text', 'json' or 'yaml'").Default(teleport.Text).StringVar(&c.format)
	c.desktopList.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&c.verbose)

	c.desktopBootstrap = desktop.Command("bootstrap", "Generate a PowerShell script to bootstrap Active Directory.")
}

// TryRun attempts to run subcommands like "desktop ls".
func (c *DesktopCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.desktopList.FullCommand():
		commandFunc = c.ListDesktop
	case c.desktopBootstrap.FullCommand():
		commandFunc = c.BootstrapAD
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

// ListDesktop prints the list of desktops that have recently sent heartbeats
// to the cluster.
func (c *DesktopCommand) ListDesktop(ctx context.Context, client *authclient.Client) error {
	desktops, err := client.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return trace.Wrap(err)
	}
	coll := windowsDesktopCollection{
		desktops: desktops,
	}
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

// BootstrapAD generates a PowerShell script that can be used to bootstrap Active Directory.
func (c *DesktopCommand) BootstrapAD(ctx context.Context, client *authclient.Client) error {
	script, err := client.GetDesktopBootstrapScript(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = os.Stdout.Write([]byte(script))
	return trace.Wrap(err)
}

var desktopMessageTemplate = template.Must(template.New("desktop").Parse(`The invite token: {{.token}}
This token will expire in {{.minutes}} minutes.

This token enables Desktop Access.  See https://goteleport.com/docs/desktop-access/
for detailed information on configuring Teleport Desktop Access with this token.

`))
