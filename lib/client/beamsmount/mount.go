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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
)

// MountOptions contains the parameters for mounting a beam filesystem.
type MountOptions struct {
	// BeamID is the UUID of the beam resource.
	BeamID string
	// BeamAlias is the human-friendly beam name.
	BeamAlias string
	// NodeID is the SSH node identifier for the beam.
	NodeID string
	// MountPoint is the local directory to mount the beam at.
	MountPoint string
	// RemotePath is the remote filesystem path to mount (default "/").
	RemotePath string
	// TshPath is the absolute path to the tsh binary.
	TshPath string
	// Debug enables tsh --debug flag for the SSH command.
	Debug bool
	// SshfsDebug enables sshfs_debug output (adds -d flag on top of -f).
	SshfsDebug bool
	// NodeLogin overrides the default "beams" login user.
	NodeLogin string
	// StateFile is the path to the mount state JSON file.
	StateFile string
	// ProxyHost is the proxy hostname, passed to the watcher subprocess.
	ProxyHost string
	// Stdout is the writer for user-facing output.
	Stdout io.Writer
	// Stderr is the writer for warnings (stale mount messages).
	Stderr io.Writer
}

// Mount mounts a beam filesystem via SSHFS with state tracking.
//
// The flow is:
//  1. Resolve mount point to absolute path.
//  2. Prune stale entries from the state file and warn the user.
//  3. Start sshfs with -f (foreground) via cmd.Start().
//     We use -f so the sshfs process doesn't daemonize, which lets us
//     capture the real PID. When tsh exits, the orphaned sshfs process
//     is reparented to init/launchd and keeps running.
//  4. Poll briefly to confirm the mount is established.
//  5. Write mount entry to the state file.
//  6. Spawn a detached watcher subprocess that cleans up the state entry
//     when sshfs exits.
func Mount(opts MountOptions) error {
	absMountPoint, err := filepath.Abs(opts.MountPoint)
	if err != nil {
		return trace.Wrap(err, "resolving mount point")
	}
	opts.MountPoint = absMountPoint

	// Prune stale mounts and warn the user.
	if err := WithStateLock(opts.StateFile, func(state *MountState) error {
		for _, w := range PruneStale(state) {
			fmt.Fprintln(opts.Stderr, "WARNING:", w)
		}
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	login := "beams"
	if opts.NodeLogin != "" {
		login = opts.NodeLogin
	}

	sshfsTarget := fmt.Sprintf("%s@%s:%s", login, opts.NodeID, opts.RemotePath)
	sshCmd := opts.TshPath + " ssh"
	if opts.Debug {
		sshCmd = opts.TshPath + " --debug ssh"
	}

	// -f keeps sshfs in the foreground so we get the real, stable PID.
	// Without -f, sshfs daemonizes and the parent PID we'd capture from
	// cmd.Start() dies immediately, making PID tracking useless.
	// Map all remote files to the local user's uid/gid. Beam containers
	// run as uid/gid 1000, but we don't use idmap=user because it tries
	// to auto-detect the remote UID by stat-ing the mount root — that
	// fails when mounting "/" (owned by root, not the beams user).
	args := []string{
		"-f",
		"-o", fmt.Sprintf("ssh_command=%s", sshCmd),
		"-o", fmt.Sprintf("uid=%d", os.Getuid()),
		"-o", fmt.Sprintf("gid=%d", os.Getgid()),
		"-o", "no_check_root",
	}
	if opts.SshfsDebug {
		args = append(args, "-o", "sshfs_debug", "-d")
	}
	args = append(args, sshfsTarget, opts.MountPoint)

	if _, err := exec.LookPath("sshfs"); err != nil {
		return trace.NotFound("sshfs is not installed. %s", sshfsInstallHint())
	}

	cmd := exec.Command("sshfs", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = opts.Stdout
	// In normal mode, suppress sshfs stderr. With -f (foreground), sshfs
	// keeps stderr open for the lifetime of the mount, producing non-fatal
	// warnings like "failed to detect remote user ID" from idmap=user.
	// Before -f, daemonization closed stderr so these were invisible.
	// Real mount failures are caught by waitForMount below.
	// In debug mode, show everything.
	if opts.SshfsDebug {
		cmd.Stderr = os.Stderr
	}

	if err := os.MkdirAll(opts.MountPoint, 0755); err != nil {
		return trace.Wrap(err, "creating mount point directory")
	}

	fmt.Fprintf(opts.Stdout, "Mounting beam %q (%s) at %s\n", beamDisplayRef(opts), opts.RemotePath, opts.MountPoint)

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err, "starting sshfs")
	}

	sshfsPID := cmd.Process.Pid

	// Wait briefly for the mount to be established. sshfs with -f stays in
	// the foreground but the mount is available almost immediately after Start.
	if err := waitForMount(opts.MountPoint, 10*time.Second); err != nil {
		_ = cmd.Process.Kill()
		if !opts.SshfsDebug {
			return trace.Wrap(err, "run with --sshfs-debug for more details")
		}
		return trace.Wrap(err)
	}

	fmt.Fprintf(opts.Stdout, "Mounted successfully.\n")

	// Record mount in state file.
	entry := MountEntry{
		BeamID:     opts.BeamID,
		BeamAlias:  opts.BeamAlias,
		MountPoint: opts.MountPoint,
		RemotePath: opts.RemotePath,
		SshfsPID:   sshfsPID,
		MountedAt:  time.Now().UTC(),
	}

	if err := WithStateLock(opts.StateFile, func(state *MountState) error {
		state.Mounts = append(state.Mounts, entry)
		return nil
	}); err != nil {
		return trace.Wrap(err, "saving mount state")
	}

	// Spawn watcher subprocess to clean up state when sshfs exits.
	watcherPID, err := spawnWatcher(opts.TshPath, opts.MountPoint, sshfsPID, opts.StateFile)
	if err != nil {
		// Non-fatal: state will be cleaned by stale detection on next command.
		fmt.Fprintf(opts.Stderr, "WARNING: failed to start mount watcher: %v\n", err)
	} else {
		// Update entry with watcher PID.
		if err := WithStateLock(opts.StateFile, func(state *MountState) error {
			if e := state.FindByMountPoint(opts.MountPoint); e != nil {
				e.WatcherPID = watcherPID
			}
			return nil
		}); err != nil {
			fmt.Fprintf(opts.Stderr, "WARNING: failed to record watcher PID: %v\n", err)
		}
	}

	return nil
}

// waitForMount polls until the mount point is stat-able or the timeout elapses.
// TODO: use device ID comparison (mountPoint vs parent) to detect an actual
// FUSE mount rather than just directory existence.
func waitForMount(mountPoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(mountPoint); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return trace.Errorf("timed out waiting for mount at %s", mountPoint)
}

func beamDisplayRef(opts MountOptions) string {
	if opts.BeamAlias != "" {
		return opts.BeamAlias
	}
	return opts.BeamID
}
