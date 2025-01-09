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

package common

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
)

// gitSSHCommand implements `tsh git ssh`.
//
// Note that this is a hidden command as it is only meant for 'git` to call.
// TODO(greedy52) support Git protocol v2.
type gitSSHCommand struct {
	*kingpin.CmdClause

	gitHubOrg string
	userHost  string
	command   []string
	options   []string
}

func newGitSSHCommand(parent *kingpin.CmdClause) *gitSSHCommand {
	cmd := &gitSSHCommand{
		CmdClause: parent.Command("ssh", "Proxy Git commands using SSH").Hidden(),
	}

	cmd.Flag("github-org", "GitHub organization.").Required().StringVar(&cmd.gitHubOrg)
	cmd.Arg("[user@]host", "Remote hostname and the login to use").Required().StringVar(&cmd.userHost)
	cmd.Arg("command", "Command to execute on a remote host").StringsVar(&cmd.command)
	cmd.Flag("option", "OpenSSH options in the format used in the configuration file").Short('o').AllowDuplicate().StringsVar(&cmd.options)
	return cmd
}

func (c *gitSSHCommand) run(cf *CLIConf) error {
	_, host, ok := strings.Cut(c.userHost, "@")
	if !ok || host != "github.com" {
		return trace.BadParameter("user-host %q is not GitHub", c.userHost)
	}

	// TODO(greedy52) when git calls tsh, tsh cannot prompt for password (e.g.
	// user session expired) using provided stdin pipe. `tc.Login` should try
	// hijacking "/dev/tty" and replace `prompt.Stdin` temporarily.
	identity, err := getGitHubIdentity(cf, c.gitHubOrg)
	if err != nil {
		return trace.Wrap(err)
	}
	logger.DebugContext(cf.Context, "Proxying git command for GitHub user.", "command", c.command, "user", identity.Username)

	cf.RemoteCommand = c.command
	cf.Options = c.options
	cf.UserHost = fmt.Sprintf("git@%s", types.MakeGitHubOrgServerDomain(c.gitHubOrg))

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.Stdin = os.Stdin
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.SSH(cf.Context, cf.RemoteCommand)
	})
	return trace.Wrap(convertSSHExitCode(tc, err))
}
