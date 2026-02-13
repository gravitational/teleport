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

//go:build !windows

package common

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

func newPlatformConnectUpdaterServiceRunCommand(app *kingpin.Application) privilegedUpdaterCLICommand {
	return updateServiceCommandNotSupported{}
}

func newPlatformConnectUpdaterServiceInstallCommand(app *kingpin.Application) privilegedUpdaterCLICommand {
	return updateServiceCommandNotSupported{}
}

func newPlatformConnectUpdaterServiceUninstallCommand(app *kingpin.Application) privilegedUpdaterCLICommand {
	return updateServiceCommandNotSupported{}
}

func newPlatformConnectUpdaterServiceInstallUpdateCommand(app *kingpin.Application) privilegedUpdaterCLICommand {
	return updateServiceCommandNotSupported{}
}

type updateServiceCommandNotSupported struct{}

func (updateServiceCommandNotSupported) FullCommand() string {
	return ""
}

func (updateServiceCommandNotSupported) run(*CLIConf) error {
	return trace.NotImplemented("this command is not supported on this platform")
}
