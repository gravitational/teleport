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
	"github.com/gravitational/teleport/lib/tbot/services/ssh"
)

// SSHMultiplexerCommand implements `tbot start ssh-multiplexer` and
// `tbot configure ssh-multiplexer`.
type SSHMultiplexerCommand struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	// EnableResumption toggles session resumption. It is only set on the target
	// bot config if `enableResumptionSetByUser` is true.
	EnableResumption          bool
	enableResumptionSetByUser bool

	ProxyCommand       []string
	ProxyTemplatesPath string
}

// NewSSHMultiplexerCommand initializes the command and flags for kubernetes outputs
// and returns a struct to contain the parse result.
func NewSSHMultiplexerCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *SSHMultiplexerCommand {
	cmd := parentCmd.Command("ssh-multiplexer", fmt.Sprintf("%s tbot with an SSH Multiplexer service.", mode))

	c := &SSHMultiplexerCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("enable-resumption", "If set, disables SSH session resumption.").IsSetByUser(&c.enableResumptionSetByUser).BoolVar(&c.EnableResumption)
	cmd.Flag("proxy-command", "The command to run as the SSH ProxyCommand, such as `fdpass-teleport`. Defaults to this tbot binary. Repeatable to add additional args.").StringsVar(&c.ProxyCommand)
	cmd.Flag("proxy-templates-path", "A path to a proxy template config file. Optional.").StringVar(&c.ProxyTemplatesPath)

	return c
}

func (c *SSHMultiplexerCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	multiplexer := &ssh.MultiplexerConfig{
		Destination:        dest,
		ProxyCommand:       c.ProxyCommand,
		ProxyTemplatesPath: c.ProxyTemplatesPath,
	}
	if c.enableResumptionSetByUser {
		multiplexer.EnableResumption = &c.EnableResumption
	}

	cfg.Services = append(cfg.Services, multiplexer)

	return nil
}
