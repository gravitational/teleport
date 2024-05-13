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

package main

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// onSSHProxyCommand is meant to be used as an OpenSSH/PuTTY proxy command. While this
// provides the same functionality as `tbot proxy ssh` it does so without invoking
// `tsh proxy ssh` which results in much less memory and cpu consumption. This will
// eventually supersede `tbot proxy ssh` as it becomes more feature rich and supports
// all the edge cases.
func onSSHProxyCommand(botConfig *config.BotConfig, cf *config.CLIConf) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cf.Port == "" {
		cf.Port = "0"
	}

	proxySSHConfig := tbot.ProxySSHConfig{
		BotConfig:                 botConfig,
		ProxyServer:               cf.ProxyServer,
		Cluster:                   cf.Cluster,
		User:                      cf.User,
		Host:                      cf.Host,
		Port:                      cf.Port,
		EnableResumption:          cf.EnableResumption,
		TLSRoutingEnabled:         cf.TLSRoutingEnabled,
		ConnectionUpgradeRequired: cf.ConnectionUpgradeRequired,
		TSHConfigPath:             cf.TSHConfigPath,
		Log:                       log,
	}

	return trace.Wrap(tbot.ProxySSH(ctx, proxySSHConfig))
}
