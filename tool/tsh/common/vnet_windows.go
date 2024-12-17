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
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/svc"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/vnet"
)

func isWindowsService() bool {
	isSvc, err := svc.IsWindowsService()
	return err == nil && isSvc
}

func newVnetCommands(app *kingpin.Application) *vnetCommands {
	vnetCommand := newVnetCommand(app)
	subcommands := []vnetCLICommand{
		vnetCommand,
	}
	if isWindowsService() {
		subcommands = append(subcommands, newVnetServiceCommand(app))
	} else {
		subcommands = append(subcommands, newVnetInstallServiceCommand(app))
	}
	return &vnetCommands{
		subcommands: subcommands,
	}
}

type vnetInstallServiceCommand struct {
	*kingpin.CmdClause
	username string
	home     string
}

func newVnetInstallServiceCommand(app *kingpin.Application) *vnetInstallServiceCommand {
	cmd := &vnetInstallServiceCommand{
		CmdClause: app.Command("vnet-install-service", "Install the VNet Windows service.").Hidden(),
	}
	cmd.Flag("username", "username of the user that the service should be installed for.").Required().StringVar(&cmd.username)
	cmd.Flag("home", "User's TELEPORT_HOME path.").Required().StringVar(&cmd.home)
	return cmd
}

func (c *vnetInstallServiceCommand) tryRun(_ *CLIConf, command string) (bool, error) {
	if c.FullCommand() != command {
		return false, nil
	}
	return true, trace.Wrap(vnet.InstallService(c.username, c.home))
}

// vnetServiceCommand is the command that runs the Windows service.
type vnetServiceCommand struct {
	*kingpin.CmdClause
	// home is the path to the user's TELEPORT_HOME.
	home string
}

func newVnetServiceCommand(app *kingpin.Application) *vnetServiceCommand {
	cmd := &vnetServiceCommand{
		CmdClause: app.Command("vnet-service", "Start the VNet service.").Hidden(),
	}
	cmd.Flag("home", "User's TELEPORT_HOME path.").Required().StringVar(&cmd.home)
	return cmd
}

func (c *vnetServiceCommand) tryRun(cf *CLIConf, command string) (bool, error) {
	if c.FullCommand() != command {
		return false, nil
	}
	return true, trace.Wrap(c.run(cf))
}

func (c *vnetServiceCommand) run(cf *CLIConf) error {
	if err := os.Setenv(types.HomeEnvVar, c.home); err != nil {
		return trace.Wrap(err)
	}
	if err := vnet.ServiceMain(cf.Context); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
