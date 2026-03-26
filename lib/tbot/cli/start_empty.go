/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// NoopCommand implements `tbot start noop` and
// `tbot configure identity`.
type NoopCommand struct {
	*sharedStartArgs
	*genericMutatorHandler
}

// NewNoopCommand initializes the command and flags for identity outputs
// and returns a struct that will contain the parse result.
func NewNoopCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *NoopCommand {
	cmd := parentCmd.Command("noop", fmt.Sprintf("%s tbot with no configured services to test onboarding config.", mode)).Alias("no-op")

	c := &NoopCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)

	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	return c
}

func (c *NoopCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
