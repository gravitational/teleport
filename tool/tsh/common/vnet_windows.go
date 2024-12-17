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
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/teleport/lib/vnet/daemon"
)

type vnetCommand struct {
	*kingpin.CmdClause
}

func newVnetCommand(app *kingpin.Application) *vnetCommand {
	cmd := &vnetCommand{
		CmdClause: app.Command("vnet", "Start Teleport VNet, a virtual network for TCP application access.").Hidden(),
	}
	return cmd
}

func (c *vnetCommand) run(cf *CLIConf) error {
	appProvider, err := newVnetAppProvider(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	processManager, err := vnet.Run(cf.Context, &vnet.RunConfig{AppProvider: appProvider})
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

// vnetAdminSetupCommand is the fallback command run as root when tsh isn't
// compiled with the vnetdaemon build tag. This is typically the case when
// running tsh in development where it's not signed and bundled in tsh.app.
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
}

func newVnetAdminSetupCommand(app *kingpin.Application) *vnetAdminSetupCommand {
	cmd := &vnetAdminSetupCommand{
		CmdClause: app.Command(teleport.VnetAdminSetupSubCommand, "Start the VNet admin subprocess.").Hidden(),
	}
	cmd.Flag("socket", "socket path").StringVar(&cmd.socketPath)
	cmd.Flag("ipv6-prefix", "IPv6 prefix for the VNet").StringVar(&cmd.ipv6Prefix)
	cmd.Flag("dns-addr", "VNet DNS address").StringVar(&cmd.dnsAddr)
	return cmd
}

func (c *vnetAdminSetupCommand) run(cf *CLIConf) error {
	homePath := os.Getenv(types.HomeEnvVar)
	if homePath == "" {
		// This runs as root so we need to be configured with the user's home path.
		return trace.BadParameter("%s must be set", types.HomeEnvVar)
	}

	config := daemon.Config{
		SocketPath: c.socketPath,
		IPv6Prefix: c.ipv6Prefix,
		DNSAddr:    c.dnsAddr,
		HomePath:   homePath,
		ClientCred: daemon.ClientCred{
			// TODO(nklaassen): figure out how to pass some form of user
			// identifier. For now Valid: true is a hack to make
			// CheckAndSetDefaults pass.
			Valid: true,
		},
	}

	return trace.Wrap(vnet.RunAdminProcess(cf.Context, config))
}
