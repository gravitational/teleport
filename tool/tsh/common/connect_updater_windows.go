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

func newPlatformConnectUpdaterServiceRunCommand(app *kingpin.Application) *updateServiceCommand {
	return &updateServiceCommand{
		CmdClause: app.Command(autoupdate.ServiceCommand, "Start the Teleport Connect updater service.").Hidden(),
	}
}

func (c *updateServiceCommand) run(_ *CLIConf) error {
	if !isWindowsService() {
		return trace.Errorf("not running as a Windows service, cannot run %s command", c.FullCommand())
	}
	if err := autoupdate.PrivilegedServiceMain(); err != nil {
		return trace.Wrap(err, "running Teleport Connect updater service")
	}
	return nil
}

type connectUpdaterServiceInstallCommand struct {
	*kingpin.CmdClause
}

func newPlatformConnectUpdaterServiceInstallCommand(app *kingpin.Application) *connectUpdaterServiceInstallCommand {
	return &connectUpdaterServiceInstallCommand{
		CmdClause: app.Command("connect-updater-install-service", "Install the Teleport Connect updater service.").Hidden(),
	}
}

func (c *connectUpdaterServiceInstallCommand) run(cf *CLIConf) error {
	return trace.Wrap(autoupdate.InstallPrivilegedService(cf.Context), "installing updater service")
}

type connectUpdaterServiceUninstallCommand struct {
	*kingpin.CmdClause
}

func newPlatformConnectUpdaterServiceUninstallCommand(app *kingpin.Application) *connectUpdaterServiceUninstallCommand {
	return &connectUpdaterServiceUninstallCommand{
		CmdClause: app.Command("connect-updater-uninstall-service", "Uninstall the Teleport Connect updater service.").Hidden(),
	}
}

func (c *connectUpdaterServiceUninstallCommand) run(cf *CLIConf) error {
	return trace.Wrap(autoupdate.UninstallPrivilegedService(cf.Context), "uninstalling updater service")
}

type connectUpdaterServiceInstallUpdateCommand struct {
	*kingpin.CmdClause
	path     string
	forceRun bool
	version  string
}

func newPlatformConnectUpdaterServiceInstallUpdateCommand(app *kingpin.Application) *connectUpdaterServiceInstallUpdateCommand {
	cmd := &connectUpdaterServiceInstallUpdateCommand{
		CmdClause: app.Command("connect-updater-install-update", "Install the update with the Teleport Connect updater service.").Hidden(),
	}
	cmd.Flag("path", "Path to the update.").Required().StringVar(&cmd.path)
	cmd.Flag("update-version", "Update version").Required().StringVar(&cmd.version)
	cmd.Flag("force-run", "Run the app after installing the update.").BoolVar(&cmd.forceRun)
	return cmd
}

func (c *connectUpdaterServiceInstallUpdateCommand) run(cf *CLIConf) error {
	return trace.Wrap(
		autoupdate.RunServiceAndInstallUpdateFromClient(cf.Context, c.path, c.forceRun, c.version),
		"installing update via Teleport Connect updater service",
	)
}
