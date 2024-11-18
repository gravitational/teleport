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

// DatabaseCommand implements `tbot start database` and
// `tbot configure database`.
type DatabaseCommand struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	Format   string
	Service  string
	Username string
	Database string
}

// NewDatabaseCommand initializes a command and flags for database outputs and
// returns a struct that will contain the parse result.
func NewDatabaseCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *DatabaseCommand {
	cmd := parentCmd.Command("database", fmt.Sprintf("%s tbot with a database output.", mode)).Alias("db")

	c := &DatabaseCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("format", "The database output format if necessary").Default("").EnumVar(&c.Format, config.SupportedDatabaseFormatStrings()...)
	cmd.Flag("service", "The database service name").Required().StringVar(&c.Service)
	cmd.Flag("username", "The database user name").Required().StringVar(&c.Username)
	cmd.Flag("database", "The name of the database available in the requested database service").Required().StringVar(&c.Database)

	return c
}

func (c *DatabaseCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &config.DatabaseOutput{
		Destination: dest,
		Format:      config.DatabaseFormat(c.Format),
		Username:    c.Username,
		Database:    c.Database,
		Service:     c.Service,
	})

	return nil
}
