// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/vnet"
)

type vnetCommand struct {
	*kingpin.CmdClause
}

func newVnetCommand(app *kingpin.Application) *vnetCommand {
	cmd := &vnetCommand{
		CmdClause: app.Command("vnet", "Start Teleport VNet, a virtual network for TCP application access.").Hidden(),
	}
	return cmd
}

func (c *vnetCommand) run(cf *CLIConf) error {
	appProvider, err := newVnetAppProvider(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	processManager, err := vnet.Run(cf.Context, &vnet.RunConfig{AppProvider: appProvider})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		<-cf.Context.Done()
		processManager.Close()
	}()

	fmt.Println("VNet is ready.")

	return trace.Wrap(processManager.Wait())
}

// vnetAdminSetupCommand is the command that is run as the Windows service.
type vnetAdminSetupCommand struct {
	*kingpin.CmdClause
	// home is the path to the user's TELEPORT_HOME.
	home string
}

func newVnetAdminSetupCommand(app *kingpin.Application) *vnetAdminSetupCommand {
	cmd := &vnetAdminSetupCommand{
		CmdClause: app.Command(teleport.VnetAdminSetupSubCommand, "Start the VNet service.").Hidden(),
	}
	cmd.Flag("home", "User's TELEPORT_HOME path.").Required().StringVar(&cmd.home)
	return cmd
}

func (c *vnetAdminSetupCommand) run(cf *CLIConf) error {
	if err := os.Setenv(types.HomeEnvVar, c.home); err != nil {
		return trace.Wrap(err)
	}
	if err := vnet.ServiceMain(cf.Context); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type vnetInstallServiceCommand struct {
	*kingpin.CmdClause
	parent   *vnetAdminSetupCommand
	username string
	home     string
}

func newVnetInstallServiceCommand(parent *vnetAdminSetupCommand) *vnetInstallServiceCommand {
	cmd := &vnetInstallServiceCommand{
		parent:    parent,
		CmdClause: parent.Command("install-service", "Install the VNet service.").Hidden(),
	}
	cmd.Flag("username", "username of the user that the service should be installed for.").Required().StringVar(&cmd.username)
	return cmd
}

func (c *vnetInstallServiceCommand) run() error {
	return trace.Wrap(vnet.InstallService(c.username, c.parent.home))
}
