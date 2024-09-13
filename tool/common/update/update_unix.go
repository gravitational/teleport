//go:build !windows

package update

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/gravitational/trace"
)

func lock(dir string) (func(), error) {
	ctx := context.Background()
	// Build the path to the lock file that will be used by flock.
	lockFile := filepath.Join(dir, ".lock")

	// Create the advisory lock using flock.
	lf, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rc, err := lf.SyscallConn()
	if err != nil {
		_ = lf.Close()
		return nil, trace.Wrap(err)
	}
	if err := rc.Control(func(fd uintptr) {
		err = syscall.Flock(int(fd), syscall.LOCK_EX)
	}); err != nil {
		_ = lf.Close()
		return nil, trace.Wrap(err)
	}

	return func() {
		rc, err := lf.SyscallConn()
		if err != nil {
			_ = lf.Close()
			slog.DebugContext(ctx, "failed to acquire syscall connection", "error", err)
			return
		}
		if err := rc.Control(func(fd uintptr) {
			err = syscall.Flock(int(fd), syscall.LOCK_UN)
		}); err != nil {
			slog.DebugContext(ctx, "failed to unlock file", "file", lockFile, "error", err)
		}
		if err := lf.Close(); err != nil {
			slog.DebugContext(ctx, "failed to close lock file", "file", lockFile, "error", err)
		}
	}, nil
}

// sendInterrupt sends a SIGINT to the process.
func sendInterrupt(cmd *exec.Cmd) error {
	return trace.Wrap(cmd.Process.Signal(syscall.SIGINT))
}
