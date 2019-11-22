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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/pborman/uuid"
	"gopkg.in/check.v1"
)

type Suite struct{}

var _ = fmt.Printf
var _ = check.Suite(&Suite{})

func TestControlGroups(t *testing.T) { check.TestingT(t) }

func (s *Suite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}
func (s *Suite) TearDownSuite(c *check.C) {}
func (s *Suite) SetUpTest(c *check.C)     {}
func (s *Suite) TearDownTest(c *check.C)  {}

// TestCreate tests creating and removing cgroups as well as shutting down
// the service and unmounting the cgroup hierarchy.
func (s *Suite) TestCreate(c *check.C) {
	// This test must be run as root. Only root can create cgroups.
	if !isRoot() {
		c.Skip("Tests for package cgroup can only be run as root.")
	}

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	dir, err := ioutil.TempDir("", "cgroup-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir)

	// Start cgroup service.
	service, err := New(&Config{
		MountPath: dir,
	})
	c.Assert(err, check.IsNil)

	// Create fake session ID and cgroup.
	sessionID := uuid.New()
	err = service.Create(sessionID)
	c.Assert(err, check.IsNil)

	// Make sure that it exists.
	cgroupPath := path.Join(dir, teleportRoot, sessionID)
	_, err = os.Stat(cgroupPath)
	if os.IsNotExist(err) {
		c.Fatalf("Could not find cgroup file %v: %v.", cgroupPath, err)
	}

	// Remove cgroup.
	err = service.Remove(sessionID)
	c.Assert(err, check.IsNil)

	// Make sure cgroup is gone.
	_, err = os.Stat(cgroupPath)
	if !os.IsNotExist(err) {
		c.Fatalf("Failed to remove cgroup at %v: %v.", cgroupPath, err)
	}

	// Close the cgroup service, this should unmound the cgroup filesystem.
	err = service.Close()
	c.Assert(err, check.IsNil)

	// Make sure the cgroup filesystem has been unmounted.
	_, err = os.Stat(cgroupPath)
	if !os.IsNotExist(err) {
		c.Fatalf("Failed to unmound cgroup filesystem from %v: %v.", dir, err)
	}
}

// TestCleanup tests the ability for Teleport to remove and cleanup all
// cgroups which is performed upon startup.
func (s *Suite) TestCleanup(c *check.C) {
	// This test must be run as root. Only root can create cgroups.
	if !isRoot() {
		c.Skip("Tests for package cgroup can only be run as root.")
	}

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	dir, err := ioutil.TempDir("", "cgroup-test")
	c.Assert(err, check.IsNil)
	if err != nil {
	}
	defer os.RemoveAll(dir)

	// Start cgroup service.
	service, err := New(&Config{
		MountPath: dir,
	})
	defer service.Close()
	c.Assert(err, check.IsNil)

	// Create fake session ID and cgroup.
	sessionID := uuid.New()
	err = service.Create(sessionID)
	c.Assert(err, check.IsNil)

	// Cleanup hierarchy to remove all cgroups.
	err = service.cleanupHierarchy()
	c.Assert(err, check.IsNil)

	// Make sure the cgroup no longer exists.
	cgroupPath := path.Join(dir, teleportRoot, sessionID)
	_, err = os.Stat(cgroupPath)
	if os.IsNotExist(err) {
		c.Fatalf("Could not find cgroup file %v: %v.", cgroupPath, err)
	}
}

// isRoot returns a boolean if the test is being run as root or not. Tests
// for this package must be run as root.
func isRoot() bool {
	if os.Geteuid() != 0 {
		return false
	}
	return true
}
