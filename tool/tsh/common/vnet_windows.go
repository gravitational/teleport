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

// vnetServiceCommand is the command that runs the Windows service.
type vnetServiceCommand struct {
	*kingpin.CmdClause
}

func newPlatformVnetServiceCommand(app *kingpin.Application) *vnetServiceCommand {
	cmd := &vnetServiceCommand{
		CmdClause: app.Command(vnet.ServiceCommand, "Start the VNet service.").Hidden(),
	}
	return cmd
}

func (c *vnetServiceCommand) run(_ *CLIConf) error {
	if !isWindowsService() {
		return trace.Errorf("not running as a Windows service, cannot run %s command", vnet.ServiceCommand)
	}
	if err := vnet.ServiceMain(); err != nil {
		return trace.Wrap(err, "running VNet Windows service")
	}
	return nil
}

func isWindowsService() bool {
	isSvc, err := svc.IsWindowsService()
	return err == nil && isSvc
}

// the admin-setup command is only supported on darwin.
func newPlatformVnetAdminSetupCommand(*kingpin.Application) vnetCommandNotSupported {
	return vnetCommandNotSupported{}
}
