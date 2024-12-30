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
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/svc"

	"github.com/gravitational/teleport/lib/vnet"
)

func isWindowsService() bool {
	isSvc, err := svc.IsWindowsService()
	return err == nil && isSvc
}

func newVnetCommands(app *kingpin.Application) *vnetCommands {
	if isWindowsService() {
		return &vnetCommands{
			subcommands: []vnetCLICommand{
				newVnetServiceCommand(app),
			},
		}
	}
	return &vnetCommands{
		subcommands: []vnetCLICommand{
			newVnetCommand(app),
			newVnetInstallServiceCommand(app),
			newVnetUninstallServiceCommand(app),
		},
	}
}

type vnetInstallServiceCommand struct {
	*kingpin.CmdClause
	userSID string
}

func newVnetInstallServiceCommand(app *kingpin.Application) *vnetInstallServiceCommand {
	cmd := &vnetInstallServiceCommand{
		CmdClause: app.Command("vnet-install-service", "Install the VNet Windows service.").Hidden(),
	}
	cmd.Flag("userSID", "SID of the user that the service should be installed for.").Required().StringVar(&cmd.userSID)
	return cmd
}

func (c *vnetInstallServiceCommand) run(cf *CLIConf) error {
	return trace.Wrap(vnet.InstallService(cf.Context, c.userSID))
}

type vnetUninstallServiceCommand struct {
	*kingpin.CmdClause
}

func newVnetUninstallServiceCommand(app *kingpin.Application) *vnetUninstallServiceCommand {
	cmd := &vnetUninstallServiceCommand{
		CmdClause: app.Command("vnet-uninstall-service", "Uninstall (delete) the VNet Windows service.").Hidden(),
	}
	return cmd
}

func (c *vnetUninstallServiceCommand) run(cf *CLIConf) error {
	return trace.Wrap(vnet.UninstallService(cf.Context))
}

// vnetServiceCommand is the command that runs the Windows service.
type vnetServiceCommand struct {
	*kingpin.CmdClause
}

func newVnetServiceCommand(app *kingpin.Application) *vnetServiceCommand {
	cmd := &vnetServiceCommand{
		CmdClause: app.Command("vnet-service", "Start the VNet service.").Hidden(),
	}
	return cmd
}

func (c *vnetServiceCommand) run(_ *CLIConf) error {
	if err := vnet.ServiceMain(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
