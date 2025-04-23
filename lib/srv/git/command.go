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

package git

import (
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/gravitational/trace"
	"github.com/mattn/go-shellwords"

	"github.com/gravitational/teleport/api/types"
)

// Repository is the repository path in the SSH command.
type Repository string

// Owner returns the first part of the repository path. If repository does not
// have multiple parts, empty string will be returned.
//
// For GitHub, owner is either the user or the organization that owns the repo.
func (r Repository) Owner() string {
	if owner, _, ok := strings.Cut(string(r), "/"); ok {
		return owner
	}
	return ""
}

// Command is the Git command to be executed.
type Command struct {
	// SSHCommand is the original SSH command.
	SSHCommand string
	// Service is the git service of the command (either git-upload-pack or
	// git-receive-pack).
	Service string
	// Repository returns the repository path of the command.
	Repository Repository
}

// ParseSSHCommand parses the provided SSH command and returns the plumbing
// command details.
func ParseSSHCommand(sshCommand string) (*Command, error) {
	args, err := shellwords.Parse(sshCommand)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(args) == 0 {
		return nil, trace.BadParameter("invalid SSH command %s", sshCommand)
	}

	// There are a number of plumbing commands but only upload-pack and
	// receive-pack are expected over SSH transport.
	// https://git-scm.com/docs/pack-protocol#_transports
	switch args[0] {
	// git-receive-pack - Receive what is pushed into the repository
	// Example: git-receive-pack 'my-org/my-repo.git'
	// https://git-scm.com/docs/git-receive-pack
	case transport.ReceivePackServiceName:
		if len(args) != 2 {
			return nil, trace.CompareFailed("invalid SSH command %s: expecting 2 arguments for %q but got %d", sshCommand, args[0], len(args))
		}
		return &Command{
			SSHCommand: sshCommand,
			Service:    args[0],
			Repository: Repository(args[1]),
		}, nil

	// git-upload-pack - Send objects packed back to git-fetch-pack
	// Example: git-upload-pack 'my-org/my-repo.git'
	// https://git-scm.com/docs/git-upload-pack
	case transport.UploadPackServiceName:
		args = skipSSHCommandFlags(args)
		if len(args) != 2 {
			return nil, trace.CompareFailed("invalid SSH command %s: expecting 2 non-flag arguments for %q but got %d", sshCommand, args[0], len(args))
		}

		return &Command{
			SSHCommand: sshCommand,
			Service:    args[0],
			Repository: Repository(args[1]),
		}, nil
	default:
		return nil, trace.BadParameter("unsupported SSH command %q", sshCommand)
	}
}

// skipSSHCommandFlags filters out flags from the provided arguments.
//
// A flag typically has "--" as prefix. If a flag requires a value, it is
// specified in the same arg with "=" (e.g. "--timeout=60") so there is no need
// to skip an extra arg.
func skipSSHCommandFlags(args []string) (ret []string) {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			ret = append(ret, arg)
		}
	}
	return
}

// checkSSHCommand performs basic checks against the SSH command.
func checkSSHCommand(server types.Server, command *Command) error {
	// Only supporting GitHub for now.
	if server.GetGitHub() == nil {
		return trace.BadParameter("the git_server is misconfigured due to missing GitHub spec, contact your Teleport administrator")
	}
	if server.GetGitHub().Organization != command.Repository.Owner() {
		return trace.AccessDenied("expect organization %q but got %q",
			server.GetGitHub().Organization,
			command.Repository.Owner(),
		)
	}
	return nil
}
