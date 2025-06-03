package vnet

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gravitational/trace"
)

func execAdminProcess(ctx context.Context, cfg LinuxAdminProcessConfig) error {
	executableName, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}

	logFile, err := os.Create(filepath.Join("/", "var", "log", "vnet.log"))
	if err != nil {
		return trace.Wrap(err, "creating log file")
	}
	defer logFile.Close()

	cmd := exec.CommandContext(ctx, "sudo", executableName, "-d",
		"vnet-service",
		"--addr", cfg.ClientApplicationServiceAddr,
		"--cred-path", cfg.ServiceCredentialPath,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return trace.Wrap(cmd.Run(), "escalating to root with sudo")
}
