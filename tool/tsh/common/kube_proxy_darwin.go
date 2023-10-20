//go:build darwin
// +build darwin

/*
Copyright 2023 Gravitational, Inc.

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

func reexecToShell(ctx context.Context, kubeconfigData string) (err error) {
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

	_, err = f.Write([]byte(kubeconfigData))
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
