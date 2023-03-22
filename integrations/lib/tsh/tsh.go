/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
