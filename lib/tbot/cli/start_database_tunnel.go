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
	"fmt"
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/config"
)

// DatabaseTunnelCommand implements `tbot start database-tunnel` and
// `tbot configure database-tunnel`.
type DatabaseTunnelCommand struct {
	*sharedStartArgs
	*genericMutatorHandler

	Listen   string
	Service  string
	Username string
	Database string
}

// NewDatabaseTunnelCommand creates a command supporting `tbot start database-tunnel`
func NewDatabaseTunnelCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *DatabaseTunnelCommand {
	cmd := parentCmd.Command("database-tunnel", fmt.Sprintf("%s tbot with a database tunnel listener.", mode)).Alias("db-tunnel")

	c := &DatabaseTunnelCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("listen", "A socket URI to listen on, such as tcp://0.0.0.0:3306").Required().StringVar(&c.Listen)
	cmd.Flag("service", "The database service name").Required().StringVar(&c.Service)
	cmd.Flag("username", "The database user name").Required().StringVar(&c.Username)
	cmd.Flag("database", "The name of the database available in the requested database service").Required().StringVar(&c.Database)

	// Note: excluding roles from the CLI; will default to all available.

	return c
}

func (c *DatabaseTunnelCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &config.DatabaseTunnelService{
		Listen:   c.Listen,
		Username: c.Username,
		Database: c.Database,
		Service:  c.Service,
	})

	return nil
}
