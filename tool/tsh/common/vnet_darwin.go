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
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/vnet"
)

func newVnetCommand(app *kingpin.Application) *vnetCommand {
	return newVnetCommandBase(app)
}

func (c *vnetCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	var customDNSZones []string
	if len(c.customDNSZones) > 0 {
		customDNSZones = strings.Split(c.customDNSZones, ",")
	}
	return trace.Wrap(vnet.Run(cf.Context, tc, customDNSZones))
}

func (c *vnetAdminSetupCommand) run(cf *CLIConf) error {
	var customDNSZones []string
	if len(c.customDNSZones) > 0 {
		customDNSZones = strings.Split(c.customDNSZones, ",")
	}
	return trace.Wrap(vnet.AdminSubcommand(cf.Context, c.socketPath, c.pidFilePath, c.baseIPv6Address, customDNSZones))
}
