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
	"os/user"
	"runtime"
	"strconv"
	"syscall"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTBot,
})

// SymlinksMode is an enum type listing various symlink behavior modes.
type SymlinksMode string

const (
	// SymlinksInsecure does allow resolving symlink paths and does not issue
	// any symlink-related warnings.
	SymlinksInsecure SymlinksMode = "insecure"

	// SymlinksTrySecure attempts to write files securely and avoid symlink
	// attacks, but falls back with a warning if the necessary OS / kernel
	// support is missing.
	SymlinksTrySecure SymlinksMode = "try-secure"

	// SymlinksSecure attempts to write files securely and fails with an error
	// if the operation fails. This should be the default on systems were we
	// expect it to be supported.
	SymlinksSecure SymlinksMode = "secure"
)

// ACLMode is an enum type listing various ACL behavior modes.
type ACLMode string

const (
	// ACLOff disables ACLs
	ACLOff ACLMode = "off"

	// ACLTry attempts to use ACLs but falls back to no ACLs with a warning if
	// unavailable.
	ACLTry ACLMode = "try"

	// ACLOn enables ACL support and fails if ACLs are unavailable.
	ACLOn ACLMode = "on"
)

const (
	// DefaultMode is the preferred permissions mode for bot files.
	DefaultMode fs.FileMode = 0600

	// DefaultModeACL is the preferred permissions mode for bot files when ACLs
	// are in use. Our preferred ACL mask overwrites the group bits and so
	// appears to be 0670 when the true permissions are owner r/w + bot user
	// r/w.
	DefaultModeACL fs.FileMode = 0670

	// DefaultDirMode is the preferred permissions mode for bot directories.
	// Directories need the execute bit set for most operations on their
	// contents to succeed.
	DefaultDirMode fs.FileMode = 0700
)

// openStandard attempts to open the given path for writing with O_CREATE set.
func openStandard(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, DefaultMode)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return file, nil
}

// createStandard creates an empty file or directory at the given path without
// attempting to prevent symlink attacks.
func createStandard(path string, isDir bool) error {
	if isDir {
		if err := os.Mkdir(path, DefaultMode); err != nil {
			return trace.Wrap(err)
		}
	} else {
		f, err := openStandard(path)
		if err != nil {
			return trace.Wrap(err)
		}

		f.Close()
	}

	return nil
}

// IsOwnedBy checks that the file at the given path is owned by the given user.
func IsOwnedBy(fileInfo fs.FileInfo, user *user.User) (bool, error) {
	if runtime.GOOS == constants.WindowsOS {
		// no-op on windows
		return true, nil
	}

	info, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return false, trace.NotImplemented("Cannot verify file ownership on this platform")
	}

	// Our files are 0600, so don't bother checking gid.
	return strconv.Itoa(int(info.Uid)) == user.Uid, nil
}
