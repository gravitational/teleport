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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// CheckSSHCommand performs basic checks against the SSH command.
func CheckSSHCommand(server types.Server, command string) error {
	cmd, err := parseSSHCommand(command)
	if err != nil {
		return trace.Wrap(err)
	}
	if server.GetGitHub() == nil {
		return trace.BadParameter("missing GitHub spec")
	}
	if server.GetGitHub().Organization != cmd.org {
		return trace.AccessDenied("expect organization %q but got %q", server.GetGitHub().Organization, cmd.org)
	}
	return nil
}

type sshCommand struct {
	gitService string
	path       string
	org        string
}

// parseSSHCommand parses the provided SSH command and returns details of the
// parts.
//
// Sample command: git-upload-pack 'my-org/my-repo.git'
func parseSSHCommand(command string) (*sshCommand, error) {
	gitService, path, ok := strings.Cut(strings.TrimSpace(command), " ")
	if !ok {
		return nil, trace.BadParameter("invalid git command %s", command)
	}

	path = strings.TrimLeft(path, quotesAndSpace)
	path = strings.TrimRight(path, quotesAndSpace)
	org, _, ok := strings.Cut(path, "/")
	if !ok {
		return nil, trace.BadParameter("invalid git command %s", command)
	}
	return &sshCommand{
		gitService: gitService,
		path:       path,
		org:        org,
	}, nil
}

const quotesAndSpace = `"' `
