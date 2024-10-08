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
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/config"
)

// InitCommand implements a command for `tbot init`
type InitCommand struct {
	*AuthProxyArgs
	*LegacyDestinationDirArgs
	*genericExecutorHandler[InitCommand]

	Owner      string
	BotUser    string
	ReaderUser string
	InitDir    string
	Clean      bool
}

// NewInitCommand constructs an InitCommand at the top level of the given
// application. It will execute `action` when selected by the user.
func NewInitCommand(app KingpinClause, action func(*InitCommand) error) *InitCommand {
	cmd := app.Command("init", "Initialize a certificate destination directory for writes from a separate bot user.")

	c := &InitCommand{}
	c.AuthProxyArgs = newAuthProxyArgs(cmd)
	c.LegacyDestinationDirArgs = newLegacyDestinationDirArgs(cmd)
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Flag("owner", "Defines Linux \"user:group\" owner of \"--destination-dir\". Defaults to the Linux user running tbot if unspecified.").StringVar(&c.Owner)
	cmd.Flag("bot-user", "Enables POSIX ACLs and defines Linux user that can read/write short-lived certificates to \"--destination-dir\".").StringVar(&c.BotUser)
	cmd.Flag("reader-user", "Enables POSIX ACLs and defines Linux user that will read short-lived certificates from \"--destination-dir\".").StringVar(&c.ReaderUser)
	cmd.Flag("init-dir", "If using a config file and multiple destinations are configured, controls which destination dir to configure.").StringVar(&c.InitDir)
	cmd.Flag("clean", "If set, remove unexpected files and directories from the destination.").BoolVar(&c.Clean)

	return c
}

func (c *InitCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if c.AuthProxyArgs != nil {
		if err := c.AuthProxyArgs.ApplyConfig(cfg, l); err != nil {
			return trace.Wrap(err)
		}
	}

	if c.LegacyDestinationDirArgs != nil {
		if err := c.LegacyDestinationDirArgs.ApplyConfig(cfg, l); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
