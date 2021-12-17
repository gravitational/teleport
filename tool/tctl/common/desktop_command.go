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

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
)

// DesktopCommand implements "tctl desktop" group of commands.
type DesktopCommand struct {
	config *service.Config

	// format is the output format (text or yaml)
	format string

	// desktopList implements the "tctl desktop ls" subcommand.
	desktopList *kingpin.CmdClause
}

// Initialize allows DesktopCommand to plug itself into the CLI parser
func (c *DesktopCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	desktop := app.Command("desktop", "Operate on registered desktops.")
	c.desktopList = desktop.Command("ls", "List all desktops registered with the cluster.")
	c.desktopList.Flag("format", "Output format, 'text', or 'yaml'").Default("text").StringVar(&c.format)
}

// TryRun attempts to run subcommands like "desktop ls".
func (c *DesktopCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.desktopList.FullCommand():
		err = c.ListDesktop(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// ListDesktop prints the list of desktops that have recently sent heartbeats
// to the cluster.
func (c *DesktopCommand) ListDesktop(client auth.ClientI) error {
	desktops, err := client.GetWindowsDesktops(context.TODO(), types.WindowsDesktopFilter{})
	if err != nil {
		return trace.Wrap(err)
	}
	coll := windowsDesktopAndServiceCollection{desktops: []windowsDesktopAndService{}}
	ctx := context.Background()
	for _, desktop := range desktops {
		ds, err := client.GetWindowsDesktopService(ctx, desktop.GetHostID())
		if err != nil {
			return trace.Wrap(err)
		}
		coll.desktops = append(coll.desktops,
			windowsDesktopAndService{desktop: desktop, service: ds})
	}
	switch c.format {
	case teleport.Text:
		err = coll.writeText(os.Stdout)
	case teleport.YAML:
		desktopColl := windowsDesktopCollection{desktops: desktops}
		err = desktopColl.writeYaml(os.Stdout)
	default:
		return trace.BadParameter("unknown format %q", c.format)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
