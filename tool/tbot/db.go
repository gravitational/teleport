/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/tshwrap"
	"github.com/gravitational/teleport/lib/utils"
)

func onDBCommand(botConfig *config.BotConfig, cf *config.CLIConf) error {
	wrapper, err := tshwrap.New()
	if err != nil {
		return trace.Wrap(err)
	}

	destination, err := tshwrap.GetDestinationDirectory(botConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	env, err := tshwrap.GetEnvForTSH(destination.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	identityPath := filepath.Join(destination.Path, config.IdentityFilePath)
	identity, err := tshwrap.LoadIdentity(identityPath)
	if err != nil {
		return trace.Wrap(err)
	}

	args := []string{"-i", identityPath, "db", "--proxy=" + cf.ProxyServer}
	if cf.Cluster != "" {
		// If we caught --cluster in our args, pass it through.
		args = append(args, "--cluster="+cf.Cluster)
	} else if !utils.HasPrefixAny("--cluster", cf.RemainingArgs) {
		// If no `--cluster` was provided after a `--`, pass along the cluster
		// name in the identity.
		args = append(args, "--cluster="+identity.RouteToCluster)
	}
	args = append(args, cf.RemainingArgs...)

	// Pass through the debug flag, and prepend to satisfy argument ordering
	// needs (`-d` must precede `db`).
	if botConfig.Debug {
		args = append([]string{"-d"}, args...)
	}

	return trace.Wrap(wrapper.Exec(env, args...), "executing `tsh db`")
}
