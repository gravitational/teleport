package vnet

import (
	"context"
	"os"
	"os/exec"

	"github.com/gravitational/trace"
)

func execAdminProcess(ctx context.Context, cfg LinuxAdminProcessConfig) error {
	executableName, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}

	cmd := exec.CommandContext(ctx, "sudo", executableName, "-d",
		"vnet-service",
		"--addr", cfg.ClientApplicationServiceAddr,
		"--cred-path", cfg.ServiceCredentialPath,
	)
	log.DebugContext(ctx, "Escalating to root with sudo")
	return trace.Wrap(cmd.Run(), "escalating to root with sudo")
}
