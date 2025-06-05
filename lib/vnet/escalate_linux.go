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

	// TODO: find a proper way to start the service without just running sudo
	// and hoping there's no password requirement...
	//
	// Also need to figure out how we want to set up a service that runs as root
	// that can be started from Connect, maybe some systemd service but that
	// doesn't solve how we allow the service to be started.
	cmd := exec.CommandContext(ctx, "sudo", executableName, "-d",
		"vnet-service",
		"--addr", cfg.ClientApplicationServiceAddr,
		"--cred-path", cfg.ServiceCredentialPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.DebugContext(ctx, "Escalating to root with sudo")
	return trace.Wrap(cmd.Run(), "escalating to root with sudo")
}
