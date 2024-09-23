//go:build !windows

package update

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
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
func sendInterrupt(pid int) error {
	err := syscall.Kill(pid, syscall.SIGINT)
	if errors.Is(err, syscall.ESRCH) {
		return trace.BadParameter("can't find the process: %v", pid)
	}
	return trace.Wrap(err)
}

// freeDiskWithReserve returns the available disk space.
func freeDiskWithReserve(dir string) (uint64, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(dir, &stat)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if stat.Bsize < 0 {
		return 0, trace.Errorf("invalid size")
	}
	avail := stat.Bavail * uint64(stat.Bsize)
	if reservedFreeDisk > avail {
		return 0, trace.Errorf("no free space left")
	}
	return avail - reservedFreeDisk, nil
}
