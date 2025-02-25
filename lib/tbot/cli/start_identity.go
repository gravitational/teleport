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

// IdentityCommand implements `tbot start identity` and
// `tbot configure identity`.
type IdentityCommand struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	Cluster      string
	AllowReissue bool
}

// NewIdentityCommand initializes the command and flags for identity outputs
// and returns a struct that will contain the parse result.
func NewIdentityCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *IdentityCommand {
	cmd := parentCmd.Command("identity", fmt.Sprintf("%s tbot with an identity output for SSH and Teleport API access.", mode)).Alias("ssh").Alias("id")

	c := &IdentityCommand{}
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.sharedStartArgs = newSharedStartArgs(cmd)

	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("cluster", "The name of a specific cluster for which to issue an identity if using a leaf cluster").StringVar(&c.Cluster)
	cmd.Flag("allow-reissue", "Allow the credentials output by this command to be reissued.").BoolVar(&c.AllowReissue)
	// Note: roles and ssh_config mode are excluded for now.

	return c
}

func (c *IdentityCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &config.IdentityOutput{
		Destination:  dest,
		Cluster:      c.Cluster,
		AllowReissue: c.AllowReissue,
	})

	return nil
}
