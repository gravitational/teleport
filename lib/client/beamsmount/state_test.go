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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMountState_ReadWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "beams_mounts.json")

	entry := MountEntry{
		BeamID:     "550e8400-e29b-41d4-a716-446655440000",
		BeamAlias:  "tidal-origin",
		MountPoint: "/tmp/tidal-origin",
		RemotePath: "/",
		SshfsPID:   12345,
		WatcherPID: 12346,
		MountedAt:  time.Date(2026, 3, 27, 14, 42, 55, 0, time.UTC),
	}

	// Write state.
	err := WithStateLock(stateFile, func(state *MountState) error {
		state.Mounts = append(state.Mounts, entry)
		return nil
	})
	require.NoError(t, err)

	// Read it back.
	var loaded MountState
	err = WithStateLock(stateFile, func(state *MountState) error {
		loaded = *state
		return nil
	})
	require.NoError(t, err)
	require.Len(t, loaded.Mounts, 1)
	require.Equal(t, entry, loaded.Mounts[0])
}

func TestMountState_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "beams_mounts.json")

	// Reading a non-existent file returns empty state.
	var loaded MountState
	err := WithStateLock(stateFile, func(state *MountState) error {
		loaded = *state
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, loaded.Mounts)
}

func TestMountState_AddMultiple(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "beams_mounts.json")

	e1 := MountEntry{BeamID: "aaa", MountPoint: "/mnt/a", RemotePath: "/"}
	e2 := MountEntry{BeamID: "bbb", MountPoint: "/mnt/b", RemotePath: "/home"}

	err := WithStateLock(stateFile, func(state *MountState) error {
		state.Mounts = append(state.Mounts, e1)
		return nil
	})
	require.NoError(t, err)

	err = WithStateLock(stateFile, func(state *MountState) error {
		state.Mounts = append(state.Mounts, e2)
		return nil
	})
	require.NoError(t, err)

	var loaded MountState
	err = WithStateLock(stateFile, func(state *MountState) error {
		loaded = *state
		return nil
	})
	require.NoError(t, err)
	require.Len(t, loaded.Mounts, 2)
	require.Equal(t, "/mnt/a", loaded.Mounts[0].MountPoint)
	require.Equal(t, "/mnt/b", loaded.Mounts[1].MountPoint)
}

func TestMountState_RemoveByMountPoint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "beams_mounts.json")

	err := WithStateLock(stateFile, func(state *MountState) error {
		state.Mounts = []MountEntry{
			{BeamID: "aaa", MountPoint: "/mnt/a"},
			{BeamID: "bbb", MountPoint: "/mnt/b"},
		}
		return nil
	})
	require.NoError(t, err)

	err = WithStateLock(stateFile, func(state *MountState) error {
		state.RemoveByMountPoint("/mnt/a")
		return nil
	})
	require.NoError(t, err)

	var loaded MountState
	err = WithStateLock(stateFile, func(state *MountState) error {
		loaded = *state
		return nil
	})
	require.NoError(t, err)
	require.Len(t, loaded.Mounts, 1)
	require.Equal(t, "/mnt/b", loaded.Mounts[0].MountPoint)
}

func TestMountState_FindByBeam(t *testing.T) {
	t.Parallel()
	state := MountState{
		Mounts: []MountEntry{
			{BeamID: "aaa", BeamAlias: "alpha", MountPoint: "/mnt/a"},
			{BeamID: "bbb", BeamAlias: "beta", MountPoint: "/mnt/b"},
			{BeamID: "aaa", BeamAlias: "alpha", MountPoint: "/mnt/c"},
		},
	}

	// Find by ID.
	found := state.FindByBeam("aaa")
	require.Len(t, found, 2)

	// Find by alias.
	found = state.FindByBeam("beta")
	require.Len(t, found, 1)
	require.Equal(t, "/mnt/b", found[0].MountPoint)

	// No match.
	found = state.FindByBeam("nonexistent")
	require.Empty(t, found)
}

func TestMountState_PruneStale(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "beams_mounts.json")

	// Use our own PID (alive) and a bogus PID (stale).
	alivePID := os.Getpid()
	stalePID := 999999999 // Almost certainly not a real process.

	err := WithStateLock(stateFile, func(state *MountState) error {
		state.Mounts = []MountEntry{
			{BeamID: "alive", MountPoint: "/mnt/alive", SshfsPID: alivePID},
			{BeamID: "stale", MountPoint: "/mnt/stale", SshfsPID: stalePID},
		}
		return nil
	})
	require.NoError(t, err)

	var warnings []string
	err = WithStateLock(stateFile, func(state *MountState) error {
		warnings = PruneStale(state)
		return nil
	})
	require.NoError(t, err)

	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "/mnt/stale")

	var loaded MountState
	err = WithStateLock(stateFile, func(state *MountState) error {
		loaded = *state
		return nil
	})
	require.NoError(t, err)
	require.Len(t, loaded.Mounts, 1)
	require.Equal(t, "alive", loaded.Mounts[0].BeamID)
}

func TestMountState_PruneStale_AllAlive(t *testing.T) {
	t.Parallel()

	state := MountState{
		Mounts: []MountEntry{
			{BeamID: "alive", MountPoint: "/mnt/alive", SshfsPID: os.Getpid()},
		},
	}

	warnings := PruneStale(&state)
	require.Empty(t, warnings)
	require.Len(t, state.Mounts, 1)
}

func TestMountState_PruneStale_Empty(t *testing.T) {
	t.Parallel()

	state := MountState{}
	warnings := PruneStale(&state)
	require.Empty(t, warnings)
	require.Empty(t, state.Mounts)
}

func TestMountState_LockFileCreated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "beams_mounts.json")

	err := WithStateLock(stateFile, func(state *MountState) error {
		// Lock file should exist while we hold the lock.
		_, err := os.Stat(stateFile + ".lock")
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)
}
