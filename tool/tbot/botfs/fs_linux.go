//go:build linux
// +build linux

/*
Copyright 2022 Gravitational, Inc.

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

package botfs

import (
	"io/fs"
	"os"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/joshlf/go-acl"
	"golang.org/x/sys/unix"
)

const DefaultMode fs.FileMode = 0600

const Openat2MinKernel = "5.6.0"

// CreateSecure attempts to create the given file or directory without
// evaluating symlinks. This is only supported on recent Linux kernel versions
// (5.6+). The resulting file permissions are unspecified; Chmod should be
// called afterward.
func CreateSecure(path string, isDir bool) error {
	if isDir {
		// We can't specify RESOLVE_NO_SYMLINKS for mkdir. This isn't the end
		// of the world, though: if an attacker attempts a symlink attack we'll
		// just open the correct file for read/write later (and error when it
		// doesn't exist).
		if err := os.Mkdir(path, DefaultMode); err != nil {
			return trace.Wrap(err)
		}
	} else {
		how := unix.OpenHow{
			// Equivalent to 0600
			Mode:    unix.O_RDONLY | unix.S_IRUSR | unix.S_IWUSR,
			Flags:   unix.O_CREAT,
			Resolve: unix.RESOLVE_NO_SYMLINKS,
		}

		// TODO: how do we want to handle limited support for Openat2? need a
		// fallback impl + some UX to enable "paranoid mode"
		fd, err := unix.Openat2(unix.AT_FDCWD, path, &how)
		_ = unix.Close(fd)
		if err == unix.ENOSYS {
			return trace.Errorf("CreateSecure() failed (kernel may be too old, requires Linux 5.6+)")
		} else if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func VerifyACL(path string, botUser string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return trace.Wrap(err)
	}

	current, err := acl.Get(path)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("current acl for %s: %+v", path, current)

	if stat.IsDir() {

	} else {

	}

	return nil
}

func ConfigureACL(path string, botUser string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return trace.Wrap(err)
	}

	if stat.IsDir() {

	} else {

	}

	return nil
}

// HasACLSupport determines if this binary / system supports ACLs.
func HasACLSupport() (bool, error) {
	// TODO: consider checking for FS support here, for now this just assumes
	// linux is always supported.
	return true, nil
}

// IsCreateSecureSupported determines if `CreateSecure()` should be supported
// on this OS / kernel version. Note that it just checks the kernel
func IsCreateSecureSupported() (bool, error) {
	minKernel := semver.New(Openat2MinKernel)
	version, err := utils.KernelVersion()
	if err != nil {
		return false, trace.Wrap(err)
	}
	if version.LessThan(*minKernel) {
		return false, nil
	}

	return true, nil
}
