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

//go:build !vnetdaemon
// +build !vnetdaemon

package common

import (
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/teleport/lib/vnet/daemon"
)

func (c *vnetDaemonCommand) run(cf *CLIConf) error {
	return trace.NotImplemented("tsh was built without support for VNet daemon")
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
			Valid: true,
			Egid:  c.egid,
			Euid:  c.euid,
		},
	}

	return trace.Wrap(vnet.AdminSetup(cf.Context, config))
}
