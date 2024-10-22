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

// #include <stdint.h>
// #include <stdlib.h>
// extern uint64_t cgroup_id(char *path);
import "C"

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentCgroup)

// Config holds configuration for the cgroup service.
type Config struct {
	// MountPath is where the cgroupv2 hierarchy is mounted.
	MountPath string
	// RootPath directory where the Teleport managed cgroups are going to be
	// placed.
	RootPath string
}

// CheckAndSetDefaults checks BPF configuration.
func (c *Config) CheckAndSetDefaults() error {
	if c.MountPath == "" {
		c.MountPath = defaults.CgroupPath
	}
	if c.RootPath == "" {
		c.RootPath = teleportRoot
	}
	return nil
}

// Service manages cgroup orchestration.
type Service struct {
	*Config

	// teleportRoot is the root cgroup that holds all Teleport sessions. Used
	// to remove all cgroups upon shutdown.
	teleportRoot string
}

// New creates a new cgroup service.
func New(config *Config) (*Service, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Service{
		Config:       config,
		teleportRoot: filepath.Join(config.MountPath, config.RootPath, uuid.New().String()),
	}

	// Mount the cgroup2 filesystem.
	err = s.mount()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.DebugContext(context.TODO(), "Teleport session hierarchy mounted.", "hierarchy_root", s.teleportRoot)
	return s, nil
}

// Close will clean up the session cgroups and unmount the cgroup2 filesystem,
// unless otherwise requested.
func (s *Service) Close(skipUnmount bool) error {
	err := s.cleanupHierarchy()
	if err != nil {
		return trace.Wrap(err)
	}

	if skipUnmount {
		logger.DebugContext(context.TODO(), "Cleaned up Teleport session hierarchy.", "hierarchy_root", s.teleportRoot)
		return nil
	}

	err = s.unmount()
	if err != nil {
		return trace.Wrap(err)
	}

	logger.DebugContext(context.TODO(), "Cleaned up and unmounted Teleport session hierarchy.", "hierarchy_root", s.teleportRoot)
	return nil
}

// Create will create a cgroup for a given session.
func (s *Service) Create(sessionID string) error {
	err := os.Mkdir(filepath.Join(s.teleportRoot, sessionID), fileMode)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Remove will remove the cgroup for a session. An existing processes will be
// moved to the root controller.
func (s *Service) Remove(sessionID string) error {
	// Read in all PIDs for the cgroup.
	pids, err := readPids(filepath.Join(s.teleportRoot, sessionID, cgroupProcs))
	if err != nil {
		return trace.Wrap(err)
	}

	// Move all PIDs to the root controller. This has to be done before a cgroup
	// can be removed.
	err = writePids(filepath.Join(s.MountPath, cgroupProcs), pids)
	if err != nil {
		return trace.Wrap(err)
	}

	// The rmdir syscall is used to remove a cgroup.
	err = unix.Rmdir(filepath.Join(s.teleportRoot, sessionID))
	if err != nil {
		return trace.Wrap(err)
	}

	logger.DebugContext(context.TODO(), "Removed cgroup for session.", "session_id", sessionID)
	return nil
}

// Place places a process in the cgroup for that session.
func (s *Service) Place(sessionID string, pid int) error {
	// Open cgroup.procs file for the cgroup.
	filepath := filepath.Join(s.teleportRoot, sessionID, cgroupProcs)
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

	return trace.Wrap(f.Sync())
}

// readPids returns a slice of PIDs from a file. Used to get list of all PIDs
// within a cgroup.
func readPids(path string) ([]string, error) {
	f, err := utils.OpenFileNoUnsafeLinks(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	var pids []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		pids = append(pids, scanner.Text())
	}
	err = scanner.Err()
	if err != nil {
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

	return trace.Wrap(f.Sync())
}

// cleanupHierarchy removes any cgroups for any exisiting sessions.
func (s *Service) cleanupHierarchy() error {
	var sessions []string

	// Recursively look within the Teleport hierarchy for cgroups for session.
	err := filepath.Walk(filepath.Join(s.teleportRoot), func(path string, info os.FileInfo, _ error) error {
		// Only pick up cgroup.procs files.
		if !pattern.MatchString(path) {
			return nil
		}

		// Trim the path at which the cgroup hierarchy is mounted. This will
		// remove the UUID used in the mount path for this cgroup hierarchy.
		cleanpath := strings.TrimPrefix(path, filepath.Clean(s.teleportRoot))

		// Extract the session ID from the remaining parts of the path that
		// should look like ["" UUID cgroup.procs].
		parts := strings.Split(cleanpath, string(os.PathSeparator))
		if len(parts) != 3 {
			return nil
		}
		sessionID, err := uuid.Parse(parts[1])
		if err != nil {
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
	logger.DebugContext(context.TODO(), "Mounted cgroup filesystem.", "mount_path", s.MountPath)

	// Create cgroup that will hold Teleport sessions.
	err = os.MkdirAll(s.teleportRoot, fileMode)
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

type fileHandle struct {
	CgroupID uint64
}

// ID returns the cgroup ID for the given session.
func (s *Service) ID(sessionID string) (uint64, error) {
	var fh fileHandle
	path := filepath.Join(s.teleportRoot, sessionID)

	// Call the "name_to_handle_at" syscall directly (unix.NameToHandleAt is a
	// thin wrapper around the syscall) instead of calling the glibc wrapper.
	// This has to be done to support older versions of glibc (like the one
	// CentOS 6 ships with) which don't have the "name_to_handle_at" wrapper.
	//
	// Note that unix.NameToHandleAt is slightly more than a thin wrapper, it
	// calls "name_to_handle_at" in a loop to get the correct size of the
	// returned "f_handle" value. See the below link for more details.
	//
	// https://github.com/torvalds/linux/commit/f269099a7e7a0c6732c4a817d0e99e92216414d9
	handle, _, err := unix.NameToHandleAt(unix.AT_FDCWD, path, 0)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Read in bytes of "f_handle" which should be 8 bytes encoded little-endian.
	//
	// At the moment, all supported platforms (Linux and either AMD64 or ARM)
	// are little-endian, so this is not an issue for now. If we ever need to
	// support a big-endian platform, this file will have to be split into platform
	// specific versions. See the following thread for more details:
	// https://groups.google.com/forum/#!topic/golang-nuts/3GEzwKfRRQw.
	err = binary.Read(bytes.NewBuffer(handle.Bytes()), binary.LittleEndian, &fh)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return fh.CgroupID, nil
}

var (
	// pattern matches cgroup process files.
	pattern = regexp.MustCompile(`cgroup\.procs$`)
)

const (
	// fileMode is the mode files and directories are created in within the
	// cgroup filesystem.
	fileMode = 0555

	// teleportRoot is the prefix of the root cgroup that holds all other
	// Teleport cgroups.
	teleportRoot = "teleport"

	// cgroupProcs is the name of the file that contains all processes within
	// a cgroup.
	cgroupProcs = "cgroup.procs"
)
