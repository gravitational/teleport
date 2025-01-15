// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/utils"
)

type agentCommand struct {
	*kingpin.CmdClause
}

func newAgentCommand(app *kingpin.Application) *agentCommand {
	cmd := &agentCommand{
		CmdClause: app.Command("agent", "Start Teleport key agent."),
	}
	return cmd
}

func (c *agentCommand) run(cf *CLIConf) error {
	ctx, cancel := context.WithCancel(cf.Context)
	defer cancel()

	cf.EnableAgentExtensions = true
	clt, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	profile, err := clt.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	keyAgentPath := profile.KeyAgentPath()

	l, err := net.Listen("unix", keyAgentPath)
	if err != nil {
		if !errors.Is(err, syscall.EADDRINUSE) {
			return trace.Wrap(err)
		}

		ok, err := prompt.Confirmation(ctx, clt.Stderr, prompt.Stdin(), "Existing Teleport key agent found, would you like to overwrite it?")
		if err != nil || !ok {
			return trace.Wrap(err)
		}

		if err := os.Remove(keyAgentPath); err != nil {
			return trace.Wrap(err)
		}

		l, err = net.Listen("unix", keyAgentPath)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	context.AfterFunc(ctx, func() { l.Close() })

	fmt.Fprintln(clt.Stderr, "Listening for Teleport key agent")
	for {
		conn, err := l.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) {
				return nil
			}
			return trace.Wrap(err)
		}

		if err := agent.ServeAgent(clt.LocalAgent().ExtendedAgent, conn); err != nil && !errors.Is(err, io.EOF) {
			return trace.Wrap(err)
		}
	}
}
