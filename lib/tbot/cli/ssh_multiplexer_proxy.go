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

// SSHMultiplexerProxyCommand includes fields for `tbot ssh-multiplexer-proxy-command`
type SSHMultiplexerProxyCommand struct {
	*genericExecutorHandler[SSHMultiplexerProxyCommand]

	Socket string
	Data   string
}

// NewSSHMultiplexerProxyCommand initializes and parses args for `tbot ssh-multiplexer-proxy-command`
func NewSSHMultiplexerProxyCommand(app KingpinClause, action func(*SSHMultiplexerProxyCommand) error) *SSHMultiplexerProxyCommand {
	cmd := app.Command(
		"ssh-multiplexer-proxy-command",
		"An OpenSSH compatible ProxyCommand which connects to a long-lived tbot running the ssh-multiplexer service",
	).Hidden()

	c := &SSHMultiplexerProxyCommand{}
	c.genericExecutorHandler = newGenericExecutorHandler(cmd, c, action)

	cmd.Arg("path", "Path to the listener socket.").Required().StringVar(&c.Socket)
	cmd.Arg("data", "Connection target.").Required().StringVar(&c.Data)

	return c
}
