//go:build linux
// +build linux

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cgroup

import (
	"os"
	"path"
	"testing"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
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
	cgroupPath := path.Join(service.teleportRoot, sessionID)
	require.DirExists(t, cgroupPath)

	// Remove cgroup.
	err = service.Remove(sessionID)
	require.NoError(t, err)

	// Make sure cgroup is gone.
	require.NoDirExists(t, cgroupPath)

	// Close the cgroup service, this should unmound the cgroup filesystem.
	err = service.Close()
	require.NoError(t, err)

	// Make sure the cgroup filesystem has been unmounted.
	require.NoDirExists(t, service.teleportRoot)
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
	defer service.Close()

	// Create fake session ID and cgroup.
	sessionID := uuid.New().String()
	err = service.Create(sessionID)
	require.NoError(t, err)

	// Cleanup hierarchy to remove all cgroups.
	err = service.cleanupHierarchy()
	require.NoError(t, err)

	// Make sure the cgroup no longer exists.
	cgroupPath := path.Join(service.teleportRoot, sessionID)
	require.NoDirExists(t, cgroupPath)
}

// isRoot returns a boolean if the test is being run as root or not. Tests
// for this package must be run as root.
func isRoot() bool {
	return os.Geteuid() == 0
}
