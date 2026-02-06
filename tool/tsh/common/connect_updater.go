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

import "github.com/alecthomas/kingpin/v2"

type privilegedUpdaterCLICommand interface {
	FullCommand() string
	run(*CLIConf) error
}

func newConnectUpdaterServiceInstallCommand(app *kingpin.Application) privilegedUpdaterCLICommand {
	return newPlatformConnectUpdaterServiceInstallCommand(app)
}

func newConnectUpdaterServiceUninstallCommand(app *kingpin.Application) privilegedUpdaterCLICommand {
	return newPlatformConnectUpdaterServiceUninstallCommand(app)
}

func newConnectUpdaterServiceRunCommand(app *kingpin.Application) privilegedUpdaterCLICommand {
	return newPlatformConnectUpdaterServiceRunCommand(app)
}

func newConnectUpdaterServiceInstallUpdateCommand(app *kingpin.Application) privilegedUpdaterCLICommand {
	return newPlatformConnectUpdaterServiceInstallUpdateCommand(app)
}
