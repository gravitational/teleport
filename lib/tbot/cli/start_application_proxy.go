/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"fmt"
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/services/application"
)

// ApplicationProxyCommand implements `tbot start application-proxy` and
// `tbot configure application-proxy`.
type ApplicationProxyCommand struct {
	*sharedStartArgs
	*genericMutatorHandler

	Listen string
}

// NewApplicationProxyCommand initializes flags for an app proxy command and
// returns a struct to contain the parse result.
func NewApplicationProxyCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *ApplicationProxyCommand {
	cmd := parentCmd.Command("application-proxy", fmt.Sprintf("%s tbot with an application proxy.", mode)).Alias("app-proxy")

	c := &ApplicationProxyCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("listen", "A socket URI, such as tcp://0.0.0.0:8080").Required().StringVar(&c.Listen)

	return c
}

func (c *ApplicationProxyCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &application.ProxyServiceConfig{
		Listen: c.Listen,
	})

	return nil
}
