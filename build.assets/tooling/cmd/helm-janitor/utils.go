package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"

	"github.com/gravitational/trace"
)

const (
	yamllintBinName    = "yamllint"
	helmBinName        = "helm"
	yamlLintConfigPath = "examples/chart/.lint-config.yaml"
)

func checkDependencies(names ...string) error {
	for _, name := range names {
		_, err := exec.LookPath(name)
		if err != nil {
			return trace.NotFound("%s not found in $PATH", name)
		}
	}
	return nil
}

func run(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			return stdout.Bytes(), stderr.Bytes(), trace.Wrap(err, "command %s exited with status %d", command, exiterr.ExitCode())
		}
		return stdout.Bytes(), stderr.Bytes(), trace.Wrap(err)
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}
