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

// ApplicationCommand implements `tbot start application` and
// `tbot configure application`.
type ApplicationCommand struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	AppName               string
	SpecificTLSExtensions bool
}

// NewApplicationCommand initializes a command and flag for application outputs
// and returns a struct that will contain the parse result.
func NewApplicationCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *ApplicationCommand {
	cmd := parentCmd.Command("application", fmt.Sprintf("%s tbot with an application output.", mode)).Alias("app")

	c := &ApplicationCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("app", "The name of the app in Teleport").Required().StringVar(&c.AppName)
	cmd.Flag("specific-tls-extensions", "If set, include additional `tls.crt`, `tls.key`, and `tls.cas` for apps that require these file extensions").BoolVar(&c.SpecificTLSExtensions)

	// Note: CLI will not support roles; all will be requested.

	return c
}

func (c *ApplicationCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &config.ApplicationOutput{
		Destination:           dest,
		AppName:               c.AppName,
		SpecificTLSExtensions: c.SpecificTLSExtensions,
	})

	return nil
}
