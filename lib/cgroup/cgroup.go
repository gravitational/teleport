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
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/gravitational/trace"
)

type Service struct {
}

// TODO(russjones): Add support to cleanup unused cgroups.
func New() (*Service, error) {
	if !isMounted() {
		err := mount()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &Service{}, nil
}

func (s *Service) Close() error {
	return unmount()
}

func (s *Service) Create(sessionID string) error {
	err := os.Mkdir("/cgroup2/teleport-session-"+sessionID, 0555)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// TODO(russjones): Check if multiple processes write to the same cgroup file
// atomically.
// Place will place a process in the cgroup for that session.
func (s *Service) Place(pid int, sessionID string) error {
	f, err := os.OpenFile("/cgroup2/teleport-session-"+sessionID+"/cgroup.procs", os.O_APPEND|os.O_WRONLY, 0755)
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

// Unplace will remove a process from all Teleport related cgroups.
func (s *Service) Unplace(pid int) error {
	f, err := os.OpenFile("/cgroup2/cgroup.procs", os.O_APPEND|os.O_WRONLY, 0755)
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
	return exec.Command("/usr/bin/mount", "-t", "cgroup2", "none", "/cgroup2").Run()

}

// unmount will unmount the cgroupv2 filesystem.
func unmount() error {
	return unix.Unmount("/cgroup2", 0)
}

// ID returns the cgroup ID for the given cgroup at the given path.
func ID(path string) (uint64, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	cgid := C.cgroup_id(cpath)
	if cgid == 0 {
		return 0, trace.BadParameter("cgroup resolution failed")
	}

	return uint64(cgid), nil
}
