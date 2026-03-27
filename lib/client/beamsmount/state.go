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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

// MountEntry represents a single tracked SSHFS mount.
type MountEntry struct {
	BeamID     string    `json:"beam_id"`
	BeamAlias  string    `json:"beam_alias"`
	MountPoint string    `json:"mount_point"`
	RemotePath string    `json:"remote_path"`
	SshfsPID   int       `json:"sshfs_pid"`
	WatcherPID int       `json:"watcher_pid"`
	MountedAt  time.Time `json:"mounted_at"`
}

// MountState holds all tracked mounts for a single cluster.
type MountState struct {
	Mounts []MountEntry `json:"mounts"`
}

// RemoveByMountPoint removes all entries matching the given mount point.
func (s *MountState) RemoveByMountPoint(mountPoint string) {
	filtered := s.Mounts[:0]
	for _, m := range s.Mounts {
		if m.MountPoint != mountPoint {
			filtered = append(filtered, m)
		}
	}
	s.Mounts = filtered
}

// RemoveByBeam removes all entries matching the given beam ID or alias.
func (s *MountState) RemoveByBeam(beamRef string) {
	filtered := s.Mounts[:0]
	for _, m := range s.Mounts {
		if m.BeamID != beamRef && m.BeamAlias != beamRef {
			filtered = append(filtered, m)
		}
	}
	s.Mounts = filtered
}

// FindByBeam returns all entries matching the given beam ID or alias.
func (s *MountState) FindByBeam(beamRef string) []MountEntry {
	var found []MountEntry
	for _, m := range s.Mounts {
		if m.BeamID == beamRef || m.BeamAlias == beamRef {
			found = append(found, m)
		}
	}
	return found
}

// FindByMountPoint returns the entry matching the given mount point, or nil.
func (s *MountState) FindByMountPoint(mountPoint string) *MountEntry {
	for i := range s.Mounts {
		if s.Mounts[i].MountPoint == mountPoint {
			return &s.Mounts[i]
		}
	}
	return nil
}

// StateFilePath returns the path to the mount state file for the given
// tsh home directory and proxy host. Mount state is cluster-scoped, stored
// alongside other per-proxy data in ~/.tsh/keys/<proxy>/beams_mounts.json.
func StateFilePath(tshHome, proxyHost string) string {
	return filepath.Join(tshHome, "keys", proxyHost, "beams_mounts.json")
}

// WithStateLock acquires an exclusive file lock on the state file's sibling
// lock file, reads the current state, calls fn, and writes the result back.
// The lock file is created if it doesn't exist. If the state file doesn't
// exist, fn receives an empty MountState.
func WithStateLock(stateFile string, fn func(*MountState) error) error {
	if err := os.MkdirAll(filepath.Dir(stateFile), 0700); err != nil {
		return trace.ConvertSystemError(err)
	}

	lockPath := stateFile + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer lockFile.Close()

	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		return trace.ConvertSystemError(err)
	}
	defer unix.Flock(int(lockFile.Fd()), unix.LOCK_UN) //nolint:errcheck

	state, err := readState(stateFile)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := fn(state); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(writeState(stateFile, state))
}

// PruneStale removes entries whose sshfs process is no longer alive.
// Returns a human-readable warning string for each pruned entry so
// callers can inform the user. Uses kill(pid, 0) to check liveness
// without sending an actual signal.
func PruneStale(state *MountState) []string {
	var warnings []string
	alive := state.Mounts[:0]
	for _, m := range state.Mounts {
		if isProcessAlive(m.SshfsPID) {
			alive = append(alive, m)
		} else {
			warnings = append(warnings, fmt.Sprintf(
				"mount %s (beam %s) is no longer active, removing from state",
				m.MountPoint, beamDisplayName(m),
			))
		}
	}
	state.Mounts = alive
	return warnings
}

// isProcessAlive checks whether a process with the given PID exists.
// Sending signal 0 checks for existence without affecting the process.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func beamDisplayName(m MountEntry) string {
	if m.BeamAlias != "" {
		return m.BeamAlias
	}
	return m.BeamID
}

func readState(path string) (*MountState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &MountState{}, nil
		}
		return nil, trace.ConvertSystemError(err)
	}
	var state MountState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, trace.Wrap(err, "parsing mount state from %s", path)
	}
	return &state, nil
}

func writeState(path string, state *MountState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.ConvertSystemError(os.WriteFile(path, data, 0600))
}
