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
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentBPF,
})

// Config holds configuration for the cgroup service.
type Config struct {
	// MountPath is where the cgroupv2 hierarchy is mounted.
	MountPath string
}

// CheckAndSetDefaults checks BPF configuration.
func (c *Config) CheckAndSetDefaults() error {
	if c.MountPath == "" {
		c.MountPath = defaults.CgroupMountPath
	}
	return nil
}

type Service struct {
	*Config
}

func New(config *Config) (*Service, error) {
	var err error

	s := &Service{
		Config: config,
	}

	if !isMounted() {
		err = mount()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Teleport has restarted, all sessions have been killed, remove all cgroups.
	err = s.removeAll()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}

func (s *Service) Close() error {
	return unmount()
}

func (s *Service) Create(sessionID string) error {
	path := "/cgroup2/teleport/" + sessionID

	err := os.Mkdir(path, 0555)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Service) Remove(sessionID string) error {
	pids, err := readPids("/cgroup2/teleport/" + sessionID + "/cgroup.procs")
	if err != nil {
		return trace.Wrap(err)
	}

	err = writePids("/cgroup2/cgroup.procs", pids)
	if err != nil {
		return trace.Wrap(err)
	}

	err = exec.Command("/bin/rmdir", "/cgroup2/teleport/"+sessionID).Run()
	if err != nil {
		return trace.Wrap(err)
	}

	log.Debugf("Removed cgroup for session: %v.", sessionID)

	return nil
}

// TODO(russjones): Check if multiple processes write to the same cgroup file
// atomically.
// Place will place a process in the cgroup for that session.
func (s *Service) Place(sessionID string, pid int) error {
	f, err := os.OpenFile("/cgroup2/teleport/"+sessionID+"/cgroup.procs", os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	_, err = f.WriteString(strconv.Itoa(pid))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

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

func writePids(path string, pids []string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0555)
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

// TODO(russjones): Does this make sense in the situation where you restart?
// What if it's a graceful restart?
func (s *Service) removeAll() error {
	var sessions []string

	err := filepath.Walk("/cgroup2", func(path string, info os.FileInfo, err error) error {
		if !cgroupPattern.MatchString(path) {
			return nil
		}

		parts := strings.Split(path, "/")
		if len(parts) != 5 {
			return trace.BadParameter("invalid cgroup: %v", path)
		}
		sessions = append(sessions, parts[3])

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for _, sessionID := range sessions {
		err := s.Remove(sessionID)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// isMounted checks if the cgroup filesystem has already been mounted.
// TODO(russjones): Parse /proc/mounts line by line and check if the
// filesystem cgroup2 is not mounted, not that it doesn't occur anywhere in
// the file.
func isMounted() bool {
	buf, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}

	return bytes.Contains(buf, []byte("cgroup2"))
}

// mount will mount the cgroupv2 filesystem.
func mount() error {
	log.Printf("--> Attempting to mount.\n")
	// TODO: Log debug error.
	os.Mkdir("/cgroup2", 0555)

	// TODO(russjones): Replace with:
	// return unix.Mount("", "/cgroup2", "cgroup2", 0, "ro")
	err := exec.Command("/usr/bin/mount", "-t", "cgroup2", "none", "/cgroup2").Run()
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.Mkdir("/cgroup2/teleport", 0555)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// unmount will unmount the cgroupv2 filesystem.
func unmount() error {
	return unix.Unmount("/cgroup2", 0)
}

// ID returns the cgroup ID for the given session.
func ID(sessionID string) (uint64, error) {
	path := "/cgroup2/teleport/" + sessionID

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	cgid := C.cgroup_id(cpath)
	if cgid == 0 {
		return 0, trace.BadParameter("cgroup resolution failed")
	}

	return uint64(cgid), nil
}

var cgroupPattern = regexp.MustCompile(`^/cgroup2/teleport/[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}/cgroup.procs`)
