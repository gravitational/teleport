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
)

var windowsServiceNotImplemented = &trace.NotImplementedError{Message: "VNet Windows service is not yet implemented"}

type vnetInstallServiceCommand struct {
	*kingpin.CmdClause
}

func newPlatformVnetInstallServiceCommand(app *kingpin.Application) *vnetInstallServiceCommand {
	cmd := &vnetInstallServiceCommand{
		CmdClause: app.Command("vnet-install-service", "Install the VNet Windows service.").Hidden(),
	}
	return cmd
}

func (c *vnetInstallServiceCommand) run(cf *CLIConf) error {
	// TODO(nklaassen): implement VNet Windows service installation.
	return trace.Wrap(windowsServiceNotImplemented)
}

type vnetUninstallServiceCommand struct {
	*kingpin.CmdClause
}

func newPlatformVnetUninstallServiceCommand(app *kingpin.Application) *vnetUninstallServiceCommand {
	cmd := &vnetUninstallServiceCommand{
		CmdClause: app.Command("vnet-uninstall-service", "Uninstall (delete) the VNet Windows service.").Hidden(),
	}
	return cmd
}

func (c *vnetUninstallServiceCommand) run(cf *CLIConf) error {
	// TODO(nklaassen): implement VNet Windows service uninstallation.
	return trace.Wrap(windowsServiceNotImplemented)
}

// vnetServiceCommand is the command that runs the Windows service.
type vnetServiceCommand struct {
	*kingpin.CmdClause
}

func newPlatformVnetServiceCommand(app *kingpin.Application) *vnetServiceCommand {
	cmd := &vnetServiceCommand{
		CmdClause: app.Command("vnet-service", "Start the VNet service.").Hidden(),
	}
	return cmd
}

func (c *vnetServiceCommand) run(_ *CLIConf) error {
	if !isWindowsService() {
		return trace.Errorf("not running as a Windows service, cannot run vnet-service command")
	}
	// TODO(nklaassen): implement VNet Windows service.
	return trace.Wrap(windowsServiceNotImplemented)
}

func isWindowsService() bool {
	isSvc, err := svc.IsWindowsService()
	return err == nil && isSvc
}

// the admin-setup command is only supported on darwin.
func newPlatformVnetAdminSetupCommand(*kingpin.Application) vnetCommandNotSupported {
	return vnetCommandNotSupported{}
}
