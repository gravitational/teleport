// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configure

import (
	"context"
	"os"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// SSOConfigureCommand implements common.CLICommand interface
type SSOConfigureCommand struct {
	Config       *service.Config
	ConfigureCmd *kingpin.CmdClause
	AuthCommands []*AuthKindCommand
	Logger       *logrus.Entry
}

type AuthKindCommand struct {
	Parsed bool
	Run    func(ctx context.Context, clt auth.ClientI) error
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *SSOConfigureCommand) Initialize(app *kingpin.Application, cfg *service.Config) {
	cmd.Config = cfg
	cmd.Logger = cfg.Log.WithField(trace.Component, teleport.ComponentClient)

	sso := app.Command("sso", "A family of commands for configuring and testing auth connectors (SSO).")
	cmd.ConfigureCmd = sso.Command("configure", "Create auth connector configuration.")
	cmd.AuthCommands = []*AuthKindCommand{addGithubCommand(cmd)}
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *SSOConfigureCommand) TryRun(ctx context.Context, selectedCommand string, clt auth.ClientI) (match bool, err error) {
	for _, subCommand := range cmd.AuthCommands {
		if subCommand.Parsed {
			// the default tctl logging behavior is to ignore all logs, unless --debug is present.
			// we want different behavior: log messages as normal, but with compact format (no time, no caller info).
			if !cmd.Config.Debug {
				formatter := utils.NewDefaultTextFormatter(trace.IsTerminal(os.Stderr))
				formatter.FormatCaller = func() (caller string) { return "" }
				cmd.Logger.Logger.SetFormatter(formatter)
				cmd.Logger.Logger.SetOutput(os.Stderr)
			}

			return true, trace.Wrap(subCommand.Run(ctx, clt))
		}
	}

	return false, nil
}
