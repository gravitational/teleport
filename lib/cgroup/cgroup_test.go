//go:build linux
// +build linux

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package cgroup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

// TestRootCreate tests creating and removing cgroups as well as shutting down
// the service and unmounting the cgroup hierarchy.
func TestRootCreate(t *testing.T) {
	// This test must be run as root. Only root can create cgroups.
	if !isRoot() {
		t.Skip("Tests for package cgroup can only be run as root.")
	}

	t.Parallel()

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	dir := t.TempDir()

	// Start cgroup service.
	service, err := New(&Config{
		MountPath: dir,
	})
	require.NoError(t, err)

	// Create fake session ID and cgroup.
	sessionID := uuid.New().String()
	err = service.Create(sessionID)
	require.NoError(t, err)

	// Make sure that it exists.
	cgroupPath := filepath.Join(service.teleportRoot, sessionID)
	require.DirExists(t, cgroupPath)

	// Remove cgroup.
	err = service.Remove(sessionID)
	require.NoError(t, err)

	// Make sure cgroup is gone.
	require.NoDirExists(t, cgroupPath)

	// Close the cgroup service, this should unmound the cgroup filesystem.
	const skipUnmount = false
	err = service.Close(skipUnmount)
	require.NoError(t, err)

	// Make sure the cgroup filesystem has been unmounted.
	require.NoDirExists(t, service.teleportRoot)
}

// TestRootCreateCustomRootPath given a service configured with a custom root
// path, cgroups must be placed on the correct path.
func TestRootCreateCustomRootPath(t *testing.T) {
	// This test must be run as root. Only root can create cgroups.
	if !isRoot() {
		t.Skip("Tests for package cgroup can only be run as root.")
	}

	t.Parallel()

	for _, rootPath := range []string{
		"custom",
		"/custom",
		"nested/custom",
		"/deep/nested/custom",
	} {
		rootPath := rootPath
		t.Run(rootPath, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			service, err := New(&Config{
				MountPath: dir,
				RootPath:  rootPath,
			})
			require.NoError(t, err)
			defer service.Close(false)

			sessionID := uuid.New().String()
			err = service.Create(sessionID)
			require.NoError(t, err)

			cgroupPath := filepath.Join(service.teleportRoot, sessionID)
			require.DirExists(t, cgroupPath)
			require.Contains(t, cgroupPath, rootPath)

			err = service.Remove(sessionID)
			require.NoError(t, err)
			require.NoDirExists(t, cgroupPath)

			// Teardown
			err = service.Close(false)
			require.NoError(t, err)
			require.NoDirExists(t, service.teleportRoot)
		})
	}
}

// TestRootCleanup tests the ability for Teleport to remove and cleanup all
// cgroups which is performed upon startup.
func TestRootCleanup(t *testing.T) {
	// This test must be run as root. Only root can create cgroups.
	if !isRoot() {
		t.Skip("Tests for package cgroup can only be run as root.")
	}

	t.Parallel()

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	dir := t.TempDir()

	// Start cgroup service.
	service, err := New(&Config{
		MountPath: dir,
	})
	require.NoError(t, err)

	// Create fake session ID and cgroup.
	sessionID := uuid.New().String()
	err = service.Create(sessionID)
	require.NoError(t, err)

	// Cleanup hierarchy to remove all cgroups.
	err = service.cleanupHierarchy()
	require.NoError(t, err)

	// Make sure the cgroup no longer exists.
	cgroupPath := filepath.Join(service.teleportRoot, sessionID)
	require.NoDirExists(t, cgroupPath)

	err = service.unmount()
	require.NoError(t, err)
}

// TestRootSkipUnmount checks that closing the service with skipUnmount set to
// true works correctly; i.e. it cleans up the cgroups we're responsible for but
// doesn't unmount the cgroup2 file system.
func TestRootSkipUnmount(t *testing.T) {
	// This test must be run as root. Only root can create cgroups.
	if !isRoot() {
		t.Skip("Tests for package cgroup can only be run as root.")
	}

	t.Parallel()

	// Start a cgroup service with a temporary directory as the mount path.
	service, err := New(&Config{
		MountPath: t.TempDir(),
	})
	require.NoError(t, err)

	sessionID := uuid.NewString()
	sessionPath := filepath.Join(service.teleportRoot, sessionID)
	require.NoError(t, service.Create(sessionID))

	require.DirExists(t, sessionPath)

	const skipUnmount = true
	require.NoError(t, service.Close(skipUnmount))

	// Check whether cgroup mount path is indeed mounted as cgroup
	isCgroupMounted, err := isCgroupDir(service.MountPath)
	require.NoError(t, err)
	require.True(t, isCgroupMounted)

	require.DirExists(t, service.MountPath)
	require.NoDirExists(t, filepath.Join(service.teleportRoot, sessionID))

	require.NoError(t, service.unmount())
	
	// Check whether cgroup mount path is no longer mounted as cgroup
	isCgroupMounted, err = isCgroupDir(service.MountPath)
	require.NoError(t, err)
	require.False(t, isCgroupMounted)
}

// isRoot returns a boolean if the test is being run as root or not. Tests
// for this package must be run as root.
func isRoot() bool {
	return os.Geteuid() == 0
}

// https://elixir.bootlin.com/linux/v6.16.6/source/include/uapi/linux/magic.h#L71
const CGROUP2_SUPER_MAGIC = 0x63677270

func isCgroupDir(path string) (bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return false, err
	}

	return st.Type == CGROUP2_SUPER_MAGIC, nil
}
