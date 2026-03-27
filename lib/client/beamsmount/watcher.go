/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package beamsmount

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/gravitational/trace"
)

// spawnWatcher starts a detached tsh subprocess that monitors the sshfs PID
// and removes the mount entry from the state file when sshfs exits.
//
// The subprocess is fully detached (new session, no stdin/stdout/stderr)
// so it survives the parent tsh process exiting. If the watcher itself dies
// (e.g., machine reboot), stale detection on the next tsh command catches it.
func spawnWatcher(tshPath, mountPoint string, sshfsPID int, stateFile string) (int, error) {
	cmd := exec.Command(tshPath, "beams", "mount",
		"--cleanup",
		"--cleanup-mount-point", mountPoint,
		"--cleanup-pid", strconv.Itoa(sshfsPID),
		"--cleanup-state-file", stateFile,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return 0, trace.Wrap(err, "spawning mount watcher")
	}

	// Release the process so it isn't waited on by this (parent) process.
	if err := cmd.Process.Release(); err != nil {
		return 0, trace.Wrap(err, "releasing watcher process")
	}

	return cmd.Process.Pid, nil
}

// RunWatcher is the entry point for the watcher subprocess. It polls for
// sshfs PID liveness and removes the mount entry from the state file when
// the sshfs process exits.
func RunWatcher(mountPoint string, sshfsPID int, stateFile string) error {
	// Poll every 5 seconds — low overhead, fast enough detection.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if isProcessAlive(sshfsPID) {
			continue
		}
		// sshfs exited — clean up state.
		return trace.Wrap(WithStateLock(stateFile, func(state *MountState) error {
			state.RemoveByMountPoint(mountPoint)
			return nil
		}))
	}
	return nil
}

// ParseWatcherPID parses the --cleanup-pid flag value.
func ParseWatcherPID(raw string) (int, error) {
	pid, err := strconv.Atoi(raw)
	if err != nil {
		return 0, trace.BadParameter("invalid PID %q: %v", raw, err)
	}
	if pid <= 0 {
		return 0, trace.BadParameter("PID must be positive, got %d", pid)
	}
	return pid, nil
}
