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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// mountCgroup mounts the cgroup2 filesystem.
func mountCgroup(s *Service) error {
	// Make sure path to cgroup2 mount point exists.
	err := os.MkdirAll(s.MountPath, fileMode)
	if err != nil {
		return trace.Wrap(err)
	}

	// Check if the Teleport root cgroup exists, if it does the cgroup filesystem
	// is already mounted, return right away.
	files, err := os.ReadDir(s.MountPath)
	if err == nil && len(files) > 0 {
		// Create cgroup that will hold Teleport sessions.
		err = os.MkdirAll(s.teleportRoot, fileMode)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	// Mount the cgroup2 filesystem. Even if the cgroup filesystem is already
	// mounted, it is safe to re-mount it at another location, both will have
	// the exact same view of the hierarchy. From "man cgroups":
	//
	//   It is not possible to mount the same controller against multiple
	//   cgroup hierarchies.  For example, it is not possible to mount both
	//   the cpu and cpuacct controllers against one hierarchy, and to mount
	//   the cpu controller alone against another hierarchy.  It is possible
	//   to create multiple mount points with exactly the same set of
	//   comounted controllers.  However, in this case all that results is
	//   multiple mount points providing a view of the same hierarchy.
	//
	// The exact args to the mount syscall come strace of mount(8). From the
	// docs: https://www.kernel.org/doc/Documentation/cgroup-v2.txt:
	//
	//    Unlike v1, cgroup v2 has only single hierarchy.  The cgroup v2
	//    hierarchy can be mounted with the following mount command:
	//
	//       # mount -t cgroup2 none $MOUNT_POINT
	//
	// The output of the strace looks like the following:
	//
	//    mount("none", "/cgroup3", "cgroup2", MS_MGC_VAL, NULL) = 0
	//
	// Where MS_MGC_VAL can be dropped. From mount(2) because we only support
	// kernels 4.18 and above for this feature.
	//
	//   The mountflags argument may have the magic number 0xC0ED (MS_MGC_VAL)
	//   in the top 16 bits.  (All of the other flags discussed in DESCRIPTION
	//   occupy the low order 16 bits of mountflags.)  Specifying MS_MGC_VAL
	//   was required in kernel versions prior to 2.4, but since Linux 2.4 is
	//   no longer required and is ignored if specified.
	err = unix.Mount("none", s.MountPath, "cgroup2", 0, "")
	if err != nil {
		return trace.Wrap(err)
	}
	logger.DebugContext(context.Background(), "Mounted cgroup filesystem.", "mount_path", s.MountPath)

	// Create cgroup that will hold Teleport sessions.
	err = os.MkdirAll(s.teleportRoot, fileMode)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// createCgroup will create a cgroup for a given session.
func createCgroup(s *Service, sessionID string) error {
	err := os.Mkdir(filepath.Join(s.teleportRoot, sessionID), fileMode)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
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
	defer service.Close(true)

	require.NoError(t, mountCgroup(service))

	// Create fake session ID and cgroup.
	sessionID := uuid.New().String()
	err = createCgroup(service, sessionID)
	require.NoError(t, err)

	// Cleanup hierarchy to remove all cgroups.
	err = service.cleanupHierarchy()
	require.NoError(t, err)

	// Make sure the cgroup no longer exists.
	cgroupPath := filepath.Join(service.teleportRoot, sessionID)
	require.NoDirExists(t, cgroupPath)

	require.NoError(t, service.unmount())
}

// TestNoopCleanup tests that attempting to cleanup a hierarchy that does
// not exist does not cause an error.
func TestNoopCleanup(t *testing.T) {
	// This test must be run as root. Only root can create cgroups.
	if !isRoot() {
		t.Skip("Tests for package cgroup can only be run as root.")
	}

	t.Parallel()

	// Start cgroup service.
	service, err := New(&Config{
		MountPath: t.TempDir(),
	})
	require.NoError(t, err)
	defer service.Close(true)

	// Cleanup hierarchy to remove all cgroups.
	err = service.cleanupHierarchy()
	require.NoError(t, err)

	require.NoError(t, service.unmount())
}

// isRoot returns a boolean if the test is being run as root or not. Tests
// for this package must be run as root.
func isRoot() bool {
	return os.Geteuid() == 0
}
