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

package botfs

import (
	"context"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

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
	// if the operation fails. This should be the default on systems where we
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

	// ACLRequired enables ACL support and fails if ACLs are unavailable.
	ACLRequired ACLMode = "required"

	// DefaultMode is the preferred permissions mode for bot files.
	DefaultMode fs.FileMode = 0600

	// DefaultDirMode is the preferred permissions mode for bot directories.
	// Directories need the execute bit set for most operations on their
	// contents to succeed.
	DefaultDirMode fs.FileMode = 0700
)

// OpenFlags is a bitmask containing flags passed to `open()`
type OpenFlags int

const (
	// ReadFlags contains `open()` flags to be used when opening files for
	// reading. The file will be created if it does not exist, and reads should
	// return an empty byte array.
	ReadFlags = iota

	// WriteFlags is the mode with which files should be opened specifically
	// for writing.
	WriteFlags
)

// Flags returns opening flags for this OpenFlags variant.
func (f OpenFlags) Flags() int {
	switch f {
	case WriteFlags:
		return os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	default:
		return os.O_RDONLY
	}
}

// ACLOptions contains parameters needed to configure ACLs
type ACLOptions struct {
	// BotUser is the bot user that should have write access to this entry
	BotUser *user.User

	// ReaderUser is the user that should have read access to the file. This
	// may be nil if the reader user is not known.
	ReaderUser *user.User
}

// ACLSelector is a target for an ACL entry, pointing at e.g. a single user or
// group. Only one field may be specified in a given selector and this should be
// validated with `CheckAndSetDefaults()`.
type ACLSelector struct {
	// User is a user specifier. If numeric, it is treated as a UID. Only one
	// field may be specified per ACLSelector.
	User string `yaml:"user,omitempty"`

	// Group is a group specifier. If numeric, it is treated as a UID. Only one
	// field may be specified per ACLSelector.
	Group string `yaml:"group,omitempty"`
}

func (s *ACLSelector) CheckAndSetDefaults() error {
	if s.User != "" && s.Group != "" {
		return trace.BadParameter("reader: only one of 'user' and 'group' may be set, not both")
	}

	if s.User == "" && s.Group == "" {
		return trace.BadParameter("reader: one of 'user' or 'group' must be set")
	}

	return nil
}

// openStandard attempts to open the given path. The file may be writable
// depending on the provided `OpenFlags` value.
func openStandard(path string, flags OpenFlags) (*os.File, error) {
	file, err := os.OpenFile(path, flags.Flags(), DefaultMode)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return file, nil
}

// createStandard creates an empty file or directory at the given path without
// attempting to prevent symlink attacks.
func createStandard(path string, isDir bool) error {
	if isDir {
		if err := os.Mkdir(path, DefaultDirMode); err != nil {
			return trace.ConvertSystemError(err)
		}

		return nil
	}

	f, err := openStandard(path, WriteFlags)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := f.Close(); err != nil {
		log.WarnContext(
			context.TODO(),
			"Failed to close file",
			"path", path,
			"error", err,
		)
	}

	return nil
}

// TestACL attempts to create a temporary file in the given parent directory and
// apply an ACL to it. Note that `readers` should be representative of runtime
// reader configuration as `ConfigureACL()` may attempt to resolve named users,
// and may fail if resolution fails.
func TestACL(directory string, readers []*ACLSelector) error {
	// Note: we need to create the test file in the dest dir to ensure we
	// actually test the target filesystem.
	id, err := uuid.NewRandom()
	if err != nil {
		return trace.Wrap(err)
	}

	testFile := filepath.Join(directory, id.String())
	if err := Create(testFile, false, SymlinksInsecure); err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		err := os.Remove(testFile)
		if err != nil {
			log.DebugContext(
				context.TODO(),
				"Failed to delete ACL test file",
				"path", testFile,
			)
		}
	}()

	// Configure a dummy ACL that redundantly includes the user as a reader.
	//nolint:staticcheck // staticcheck doesn't like nop implementations in fs_other.go
	if err := ConfigureACL(testFile, readers); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
