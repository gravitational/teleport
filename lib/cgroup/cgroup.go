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

// #include <stdint.h>
// #include <stdlib.h>
// extern uint64_t cgroup_id(char *path);
import "C"

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"

	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentCgroup,
})

// Config holds configuration for the cgroup service.
type Config struct {
	// MountPath is where the cgroupv2 hierarchy is mounted.
	MountPath string
}

// CheckAndSetDefaults checks BPF configuration.
func (c *Config) CheckAndSetDefaults() error {
	if c.MountPath == "" {
		c.MountPath = defaults.CgroupPath
	}
	return nil
}

// Service manages cgroup orchestration.
type Service struct {
	*Config
}

// New creates a new cgroup service.
func New(config *Config) (*Service, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Service{
		Config: config,
	}

	// Mount the cgroup2 filesystem.
	err = s.mount()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Cleanup the Teleport cgroup2 hierarchy. This is called upon restart of
	// Teleport, so all existing sessions should be done.
	err = s.cleanupHierarchy()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}

// Close will unmount the cgroup filesystem.
func (s *Service) Close() error {
	err := s.unmount()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Create will create a cgroup for a given session.
func (s *Service) Create(sessionID string) error {
	err := os.Mkdir(path.Join(s.MountPath, teleportRoot, sessionID), fileMode)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Remove will remove the cgroup for a session. An existing processes will be
// moved to the root controller.
func (s *Service) Remove(sessionID string) error {
	// Read in all PIDs for the cgroup.
	pids, err := readPids(path.Join(s.MountPath, teleportRoot, sessionID, cgroupProcs))
	if err != nil {
		return trace.Wrap(err)
	}

	// Move all PIDs to the root controller. This has to be done before a cgroup
	// can be removed.
	err = writePids(path.Join(s.MountPath, cgroupProcs), pids)
	if err != nil {
		return trace.Wrap(err)
	}

	// The rmdir syscall is used to remove a cgroup.
	err = unix.Rmdir(path.Join(s.MountPath, teleportRoot, sessionID))
	if err != nil {
		return trace.Wrap(err)
	}

	log.Debugf("Removed cgroup for session: %v.", sessionID)

	return nil
}

// Place  place a process in the cgroup for that session.
func (s *Service) Place(sessionID string, pid int) error {
	// Open cgroup.procs file for the cgroup.
	filepath := path.Join(s.MountPath, teleportRoot, sessionID, cgroupProcs)
	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_WRONLY, fileMode)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	// Write PID and place process in cgroup.
	_, err = f.WriteString(strconv.Itoa(pid))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// readPids returns a slice of PIDs from a file. Used to get list of all PIDs
// within a cgroup.
func readPids(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	var pids []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		pids = append(pids, scanner.Text())
	}
	if scanner.Err() != nil {
		return nil, trace.Wrap(err)
	}

	return pids, nil
}

// writePids writes a slice of PIDS to a given file. Used to add processes to
// a cgroup.
func writePids(path string, pids []string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, fileMode)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	for _, pid := range pids {
		_, err := f.WriteString(pid + "\n")
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// cleanupHierarchy removes any cgroups for any exisiting sessions.
func (s *Service) cleanupHierarchy() error {
	var sessions []string

	// Recursively look within the Teleport hierarchy for cgroups for session.
	err := filepath.Walk(path.Join(s.MountPath, teleportRoot), func(path string, info os.FileInfo, err error) error {
		// Only pick up cgroup.procs files.
		if !pattern.MatchString(path) {
			return nil
		}

		// Extract the session ID. Skip over cgroup.procs files not for sessions.
		parts := strings.Split(path, string(filepath.Separator))
		if len(parts) != 5 {
			return nil
		}
		sessionID := uuid.Parse(parts[3])
		if sessionID == nil {
			return nil
		}

		// Append to the list of sessions within the cgroup hierarchy.
		sessions = append(sessions, sessionID.String())

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Remove all sessions that were found.
	for _, sessionID := range sessions {
		err := s.Remove(sessionID)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// mount mounts the cgroup2 filesystem.
func (s *Service) mount() error {
	// Make sure path to cgroup2 mount point exists.
	err := os.MkdirAll(s.MountPath, fileMode)
	if err != nil {
		return trace.Wrap(err)
	}

	// Check if the Teleport root cgroup exists, if it does the cgroup filesystem
	// is already mounted, return right away.
	_, err = os.Stat(path.Join(s.MountPath, teleportRoot))
	if err == nil {
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
	log.Debugf("Mounted cgroup filesystem to %v.", s.MountPath)

	// Create cgroup that will hold Teleport sessions.
	err = os.MkdirAll(path.Join(s.MountPath, teleportRoot), fileMode)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// unmount will unmount the cgroupv2 filesystem.
func (s *Service) unmount() error {
	// The exact args to the umount syscall come from a strace of umount(8):
	//
	//    umount2("/cgroup2", 0)                  = 0
	err := unix.Unmount(s.MountPath, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ID returns the cgroup ID for the given session.
func (s *Service) ID(sessionID string) (uint64, error) {
	path := path.Join(s.MountPath, teleportRoot, sessionID)

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	// Returns the cgroup ID of a given path.
	cgid := C.cgroup_id(cpath)
	if cgid == 0 {
		return 0, trace.BadParameter("cgroup resolution failed")
	}

	return uint64(cgid), nil
}

var (
	// pattern matches cgroup process files.
	pattern = regexp.MustCompile(`cgroup\.procs$`)
)

const (
	// fileMode is the mode files and directories are created in within the
	// cgroup filesystem.
	fileMode = 0555

	// teleportRoot is the name of the root cgroup that holds all other
	// Teleport cgroups.
	teleportRoot = "teleport"

	// cgroupProcs is the name of the file that contains all processes within
	// a cgroup.
	cgroupProcs = "cgroup.procs"
)
