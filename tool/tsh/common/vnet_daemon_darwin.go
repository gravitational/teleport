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

//go:build vnetdaemon
// +build vnetdaemon

package common

import (
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/vnet"
)

const (
	// On darwin the command must match the command provided in the .plist file.
	vnetDaemonSubCommand = "vnet-daemon"
)

// vnetDaemonCommand implements the vnet-daemon subcommand to run the VNet MacOS
// daemon.
type vnetDaemonCommand struct {
	*kingpin.CmdClause
	// Launch daemons added through SMAppService are launched from a static .plist file, hence
	// why this command does not accept any arguments.
	// Instead, the daemon expects the arguments to be sent over XPC from an unprivileged process.
}

func newPlatformVnetDaemonCommand(app *kingpin.Application) *vnetDaemonCommand {
	return &vnetDaemonCommand{
		CmdClause: app.Command(vnetDaemonSubCommand, "Start the VNet daemon").Hidden(),
	}
}

func (c *vnetDaemonCommand) run(cf *CLIConf) error {
	if cf.Debug {
		utils.InitLogger(utils.LoggingForDaemon, slog.LevelDebug)
	} else {
		utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)
	}

	return trace.Wrap(vnet.DaemonSubcommand(cf.Context))
}
