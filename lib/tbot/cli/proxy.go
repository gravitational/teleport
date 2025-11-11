/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package cli

import (
	"context"
)

// ProxyCommand supports `tbot proxy`
type ProxyCommand struct {
	*genericExecutorHandler[ProxyCommand]

	DestinationDir string
	Cluster        string
	ProxyServer    string

	// LegacyProxy is the legacy --proxy flag.
	// TODO(timothyb89): DELETE IN 17.0.0
	// TODO(timothyb89): Or maybe remove in this PR.
	LegacyProxyFlag string

	ProxyRemaining *[]string
}

// NewProxyCommand initializes the subcommand for `tbot proxy`
func NewProxyCommand(app KingpinClause, action func(*ProxyCommand) error) *ProxyCommand {
	cmd := app.Command("proxy", "Start a local TLS proxy via tsh to connect to Teleport in single-port mode.")

	c := &ProxyCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, func(c *ProxyCommand) error {
		// Prepend an action to handle --proxy deprecation.
		if c.LegacyProxyFlag != "" {
			c.ProxyServer = c.LegacyProxyFlag
			log.WarnContext(context.TODO(), "The --proxy flag is deprecated and will be removed in v17.0.0. Use --proxy-server instead")
		}

		return nil
	}, action)

	// We're migrating from --proxy to --proxy-server so this flag is hidden
	// but still supported.
	// TODO(strideynet): DELETE IN 17.0.0
	cmd.Flag("proxy", "The Teleport proxy server to use, in host:port form.").Hidden().StringVar(&c.LegacyProxyFlag)

	cmd.Flag("proxy-server", "The Teleport proxy server to use, in host:port form.").Envar(ProxyServerEnvVar).StringVar(&c.ProxyServer)
	cmd.Flag("destination-dir", "The destination directory with which to authenticate tsh").StringVar(&c.DestinationDir)
	cmd.Flag("cluster", "The cluster name. Extracted from the certificate if unset.").StringVar(&c.Cluster)
	c.ProxyRemaining = RemainingArgs(cmd.Arg(
		"args",
		"Arguments to `tsh proxy ...`; prefix with `-- ` to ensure flags are passed correctly.",
	))

	return c
}
