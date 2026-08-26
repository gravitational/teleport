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
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	libhwk "github.com/gravitational/teleport/lib/hardwarekey"
)

type pivCommands struct {
	agent *pivAgentCommand
}

func newPIVCommands(app *kingpin.Application) pivCommands {
	piv := app.Command("piv", "PIV commands.").Hidden()
	return pivCommands{
		agent: newPIVAgentCommand(piv),
	}
}

// pivAgentCommand implements `tsh piv agent`.
type pivAgentCommand struct {
	*kingpin.CmdClause
}

func newPIVAgentCommand(parent *kingpin.CmdClause) *pivAgentCommand {
	cmd := &pivAgentCommand{
		CmdClause: parent.Command("agent", "Start PIV key agent."),
	}
	return cmd
}

func (c *pivAgentCommand) run(cf *CLIConf) error {
	cf.disableHardwareKeyAgentClient = true
	store := cf.getClientStore()
	s, err := libhwk.NewAgentServer(cf.Context, store.HardwareKeyService, libhwk.DefaultAgentDir(), store.KnownHardwareKey)
	if err != nil {
		return trace.Wrap(err)
	}
	return s.Serve(cf.Context)
}
