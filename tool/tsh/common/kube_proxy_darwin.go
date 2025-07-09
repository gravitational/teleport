//go:build darwin
// +build darwin

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

package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

func reexecToShell(ctx context.Context, kubeconfigData []byte) (err error) {
	// Prepare to re-exec shell
	command := "/bin/bash"
	if shell, ok := os.LookupEnv("SHELL"); ok {
		command = shell
	}

	f, err := os.CreateTemp("", "proxy-kubeconfig-*")
	if err != nil {
		return trace.Wrap(err, "failed to create temporary file")
	}
	defer func() { err = trace.NewAggregate(err, utils.RemoveFileIfExist(f.Name())) }()
	defer func() { err = trace.NewAggregate(err, f.Close()) }()

	_, err = f.Write(kubeconfigData)
	if err != nil {
		return trace.Wrap(err, "failed to write kubeconfig into temporary file")
	}

	cmd := exec.CommandContext(ctx, command)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	// Set KUBECONFIG in the environment. Even if it was already set, we override it.
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", teleport.EnvKubeConfig, f.Name()))

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	if err := cmd.Wait(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
