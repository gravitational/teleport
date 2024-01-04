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

package tsh

import (
	"context"
	"os/exec"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/lib/logger"
)

// Tsh is a runner of tsh command.
type Tsh struct {
	Path     string
	Proxy    string
	Identity string
	Insecure bool
}

// CheckExecutable checks if `tsh` executable exists in the system.
func (tsh Tsh) CheckExecutable() error {
	_, err := exec.LookPath(tsh.cmd())
	return trace.Wrap(err, "tsh executable is not found")
}

// SSHCommand creates exec.CommandContext for tsh ssh --tty on behalf of userHost
func (tsh Tsh) SSHCommand(ctx context.Context, userHost string) *exec.Cmd {
	log := logger.Get(ctx)
	args := append(tsh.baseArgs(), "ssh")

	// Otherwise the ssh client would need to confirm that it accepts the server's public key
	/*
		The authenticity of host 'localhost:0@default@local-site' can't be established. Its public key is:
		ssh-rsa AAAA

		Are you sure you want to continue? [y/N]:
	*/
	args = append(args, "-o", "StrictHostKeyChecking no")
	args = append(args, "--tty", userHost)

	cmd := exec.CommandContext(ctx, tsh.cmd(), args...)
	log.Debugf("Running %s", cmd)

	return cmd
}

func (tsh Tsh) cmd() string {
	if tsh.Path != "" {
		return tsh.Path
	}
	return "tsh"
}

func (tsh Tsh) baseArgs() (args []string) {
	if tsh.Insecure {
		args = append(args, "--insecure")
	}
	if tsh.Identity != "" {
		args = append(args, "--identity", tsh.Identity)
	}
	if tsh.Proxy != "" {
		args = append(args, "--proxy", tsh.Proxy)
	}
	return
}
