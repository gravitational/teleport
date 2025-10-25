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

package common

import (
	"context"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// ScopedCommand implements scoped variants of tctl command groups, such as
// `tctl scoped tokens`.
type ScopedCommand struct {
	config *servicecfg.Config
	tokens *ScopedTokensCommand
	Stdout io.Writer
}

// Initialize allows ScopedCommand to plug itself into the CLI parser
func (c *ScopedCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	scoped := app.Command("scoped", "Run a subcommand using scoped auth")

	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}

	c.tokens = &ScopedTokensCommand{
		Stdout: c.Stdout,
	}

	c.tokens.Initialize(scoped, config)
}

// TryRun takes the CLI command as an argument (like "scoped tokens") and executes it.
func (c *ScopedCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	return c.tokens.TryRun(ctx, cmd, clientFunc)
}
