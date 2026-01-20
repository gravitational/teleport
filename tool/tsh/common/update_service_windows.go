// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

	"github.com/gravitational/teleport/lib/teleterm/autoupdate"
)

type updateServiceCommand struct {
	*kingpin.CmdClause
}

func newPlatformUpdateServiceCommand(app *kingpin.Application) *updateServiceCommand {
	return &updateServiceCommand{
		CmdClause: app.Command(autoupdate.ServiceCommand, "Start the privileged updater Windows service.").Hidden(),
	}
}

func (c *updateServiceCommand) run(_ *CLIConf) error {
	if !isWindowsService() {
		return trace.Errorf("not running as a Windows service, cannot run %s command", c.FullCommand())
	}
	if err := autoupdate.ServiceMain(); err != nil {
		return trace.Wrap(err, "running update Windows service")
	}
	return nil
}

type updateServiceInstallCommand struct {
	*kingpin.CmdClause
}

func newPlatformUpdateServiceInstallCommand(app *kingpin.Application) *updateServiceInstallCommand {
	return &updateServiceInstallCommand{
		CmdClause: app.Command("privileged-updater-install-service", "Install the privileged updater Windows service.").Hidden(),
	}
}

func (c *updateServiceInstallCommand) run(cf *CLIConf) error {
	return trace.Wrap(autoupdate.InstallService(cf.Context), "installing update Windows service")
}

type updateServiceUninstallCommand struct {
	*kingpin.CmdClause
}

func newPlatformUpdateServiceUninstallCommand(app *kingpin.Application) *updateServiceUninstallCommand {
	return &updateServiceUninstallCommand{
		CmdClause: app.Command("privileged-updater-uninstall-service", "Uninstall the privileged updater Windows service.").Hidden(),
	}
}

func (c *updateServiceUninstallCommand) run(cf *CLIConf) error {
	return trace.Wrap(autoupdate.UninstallService(cf.Context), "uninstalling update Windows service")
}

type updateServiceInstallUpdateCommand struct {
	*kingpin.CmdClause
	path     string
	forceRun bool
	version  string
}

func newPlatformUpdateServiceInstallUpdateCommand(app *kingpin.Application) *updateServiceInstallUpdateCommand {
	cmd := &updateServiceInstallUpdateCommand{
		CmdClause: app.Command("privileged-updater-install-update", "Install the update with the privileged updater service.").Hidden(),
	}
	cmd.Flag("path", "Path to the installer to send to the update service.").Required().StringVar(&cmd.path)
	cmd.Flag("force-run", "Force running the installer even if the update service is already active.").BoolVar(&cmd.forceRun)
	cmd.Flag("update-ver", "Update version").Required().StringVar(&cmd.version)
	return cmd
}

func (c *updateServiceInstallUpdateCommand) run(cf *CLIConf) error {
	return trace.Wrap(
		autoupdate.RunServiceAndInstallFromClient(cf.Context, c.path, c.forceRun, c.version),
		"installing update via Windows service",
	)
}
