/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package configure

import (
	"context"
	"log/slog"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// SSOConfigureCommand implements common.CLICommand interface
type SSOConfigureCommand struct {
	Config       *servicecfg.Config
	ConfigureCmd *kingpin.CmdClause
	AuthCommands []*AuthKindCommand
	Logger       *slog.Logger
}

type AuthKindCommand struct {
	Parsed bool
	Run    func(ctx context.Context, clt *authclient.Client) error
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *SSOConfigureCommand) Initialize(app *kingpin.Application, flags *tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config) {
	cmd.Config = cfg
	cmd.Logger = cfg.Logger.With(teleport.ComponentKey, teleport.ComponentClient)

	sso := app.Command("sso", "A family of commands for configuring and testing auth connectors (SSO).")
	cmd.ConfigureCmd = sso.Command("configure", "Create auth connector configuration.")
	cmd.AuthCommands = []*AuthKindCommand{addGithubCommand(cmd), addSAMLCommand(cmd), addOIDCCommand(cmd)}
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *SSOConfigureCommand) TryRun(ctx context.Context, selectedCommand string, clientFunc commonclient.InitFunc) (match bool, err error) {
	for _, subCommand := range cmd.AuthCommands {
		if subCommand.Parsed {
			// the default tctl logging behavior is to ignore all logs, unless --debug is present.
			// we want different behavior: log messages as normal, but with compact format (no time, no caller info).
			if !cmd.Config.Debug {
				cmd.Logger = slog.New(logutils.NewSlogTextHandler(os.Stderr, logutils.SlogTextHandlerConfig{
					Level:            cmd.Config.GetLogLevel(),
					EnableColors:     utils.IsTerminal(os.Stderr),
					ConfiguredFields: []string{logutils.LevelField, logutils.ComponentField},
				}))

			}
			client, closeFn, err := clientFunc(ctx)
			if err != nil {
				return false, trace.Wrap(err)
			}
			err = subCommand.Run(ctx, client)
			closeFn(ctx)

			return true, trace.Wrap(err)
		}
	}

	return false, nil
}
