//go:build darwin
// +build darwin

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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/vnet"
)

type vnetCommand struct {
	*kingpin.CmdClause
}

func newVnetCommand(app *kingpin.Application) *vnetCommand {
	cmd := &vnetCommand{
		CmdClause: app.Command("vnet", "Start Teleport VNet, a virtual network for TCP application access."),
	}
	return cmd
}

func (c *vnetCommand) run(cf *CLIConf) error {
	appProvider, err := newVnetAppProvider(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	processManager, err := vnet.SetupAndRun(cf.Context, &vnet.SetupAndRunConfig{AppProvider: appProvider})
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

// vnetAdminSetupCommand is the fallback command ran as root when tsh wasn't compiled with the
// vnetdaemon build tag. This is typically the case when running tsh in development where it's not
// signed and bundled in tsh.app.
//
// This command expects TELEPORT_HOME to be set to the tsh home of the user who wants to run VNet.
type vnetAdminSetupCommand struct {
	*kingpin.CmdClause
	// socketPath is a path to a unix socket used for passing a TUN device from the admin process to
	// the unprivileged process.
	socketPath string
	// ipv6Prefix is the IPv6 prefix for the VNet.
	ipv6Prefix string
	// dnsAddr is the IP address for the VNet DNS server.
	dnsAddr string
	// egid of the user starting VNet. Unsafe for production use, as the egid comes from an unstrusted
	// source.
	egid int
	// euid of the user starting VNet. Unsafe for production use, as the euid comes from an unstrusted
	// source.
	euid int
}

func newVnetAdminSetupCommand(app *kingpin.Application) *vnetAdminSetupCommand {
	cmd := &vnetAdminSetupCommand{
		CmdClause: app.Command(teleport.VnetAdminSetupSubCommand, "Start the VNet admin subprocess.").Hidden(),
	}
	cmd.Flag("socket", "unix socket path").StringVar(&cmd.socketPath)
	cmd.Flag("ipv6-prefix", "IPv6 prefix for the VNet").StringVar(&cmd.ipv6Prefix)
	cmd.Flag("dns-addr", "VNet DNS address").StringVar(&cmd.dnsAddr)
	cmd.Flag("egid", "effective group ID of the user starting VNet").IntVar(&cmd.egid)
	cmd.Flag("euid", "effective user ID of the user starting VNet").IntVar(&cmd.euid)
	return cmd
}

type vnetDaemonCommand struct {
	*kingpin.CmdClause
	// Launch daemons added through SMAppService are launched from a static .plist file, hence
	// why this command does not accept any arguments.
	// Instead, the daemon expects the arguments to be sent over XPC from an unprivileged process.
}

func newVnetDaemonCommand(app *kingpin.Application) *vnetDaemonCommand {
	return &vnetDaemonCommand{
		CmdClause: app.Command(vnetDaemonSubCommand, "Start the VNet daemon").Hidden(),
	}
}

// The command must match the command provided in the .plist file.
const vnetDaemonSubCommand = "vnet-daemon"
