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
	"path/filepath"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/joshlf/go-acl"
	"golang.org/x/sys/unix"
)

// Openat2MinKernel is the kernel release that adds support for the openat2()
// syscall.
const Openat2MinKernel = "5.6.0"

// modeACLReadExecute is the permissions mode needed for read on directories.
const modeACLReadExecute fs.FileMode = 05

// modeACLReadWrite is the lower 3 bytes of a UNIX file mode for permission
// bits, i.e. just one r/w/x.
const modeACLReadWrite fs.FileMode = 06

// modeACLReadWriteExecute is the permissions mode needed for full rwx on
// directories.
const modeACLReadWriteExecute fs.FileMode = 07

// modeACLNone is the UNIX file mode for no permissions, used for group and
// world read/write.
const modeACLNone fs.FileMode = 0

// openSecure opens the given path for writing (with O_CREAT, mode 0600)
// with the RESOLVE_NO_SYMLINKS flag set.
func openSecure(path string) (*os.File, error) {
	how := unix.OpenHow{
		// Equivalent to 0600. Unfortunately it's not worth reusing our
		// default file mode constant here.
		Mode:    unix.O_RDONLY | unix.S_IRUSR | unix.S_IWUSR,
		Flags:   unix.O_CREAT,
		Resolve: unix.RESOLVE_NO_SYMLINKS,
	}

	fd, err := unix.Openat2(unix.AT_FDCWD, path, &how)
	if err != nil {
		// note: returning the original error here for comparison purposes
		return nil, err
	}

	// os.File.Close() appears to close wrapped files sanely, so rely on that
	// rather than relying on callers to use unix.Close(fd)
	return os.NewFile(uintptr(fd), filepath.Base(path)), nil
}

// createStandard creates an empty file or directory at the given path while
// attempting to prevent symlink attacks.
func createSecure(path string, isDir bool) error {
	if isDir {
		// We can't specify RESOLVE_NO_SYMLINKS for mkdir. This isn't the end
		// of the world, though: if an attacker attempts a symlink attack we'll
		// just open the correct file for read/write later (and error when it
		// doesn't exist).
		if err := os.Mkdir(path, DefaultMode); err != nil {
			return trace.Wrap(err)
		}
	} else {
		f, err := openSecure(path)
		if err == unix.ENOSYS {
			// bubble up the original error for comparison
			return err
		} else if err != nil {
			return trace.Wrap(err)
		}

		// No writing to do, just close it.
		f.Close()
	}

	return nil
}

// Create attempts to create the given file or directory with the given
// symlinks mode.
func Create(path string, isDir bool, symlinksMode SymlinksMode) error {
	// Implementation note: paranoid file _creation_ is only really useful for
	// providing an early warning if openat2() / ACLs are unsupported on the
	// host system, as it will catch compatibility issues during `tbot init`.
	// Read() and Write() with Symlinks(Try)Secure are the codepaths that
	// actually prevents symlink attacks.

	switch symlinksMode {
	case SymlinksSecure:
		if err := createSecure(path, isDir); err != nil {
			if err == unix.ENOSYS {
				return trace.Errorf("createSecure(%q) failed due to missing "+
					"syscall; `symlinks: insecure` may be required for this "+
					"system", path)
			}

			return trace.Wrap(err)
		}
	case SymlinksTrySecure:
		err := createSecure(path, isDir)
		if err == nil {
			// All good, move on.
			return nil
		}

		if err != unix.ENOSYS {
			// Something else went wrong, fail.
			return trace.Wrap(err)
		}

		// TODO: this will be very noisy on older systems. Maybe flip a global
		// or something to only log once?
		log.Warnf("Failed to create %q securely due to missing syscall; "+
			"falling back to regular file creation. Set `symlinks: "+
			"insecure` on this destination to disable this warning.", path)

		return trace.Wrap(createStandard(path, isDir))
	case SymlinksInsecure:
		return trace.Wrap(createStandard(path, isDir))
	}

	return nil
}

// Write stores the given data to the file at the given path.
func Write(path string, data []byte, symlinksMode SymlinksMode) error {
	var file *os.File
	var err error

	switch symlinksMode {
	case SymlinksSecure:
		file, err = openSecure(path)
		if err == unix.ENOSYS {
			return trace.Errorf("openSecure(%q) failed due to missing "+
				"syscall; `symlinks: insecure` may be required for this "+
				"system", path)
		} else if err != nil {
			return trace.Wrap(err)
		}
	case SymlinksTrySecure:
		file, err = openSecure(path)
		if err == unix.ENOSYS {
			log.Warnf("Failed to write to %q securely due to missing "+
				"syscall; falling back to regular file write. Set "+
				"`symlinks: insecure` on this destination to disable this "+
				"warning.", path)
			file, err = openStandard(path)
			if err != nil {
				return trace.Wrap(err)
			}
		} else if err != nil {
			return trace.Wrap(err)
		}
	case SymlinksInsecure:
		file, err = openStandard(path)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// desiredBotPerms determines the desired bot permissions for an artifact at
// the given path. Directories require read/exec, files require read/write.
func desiredPerms(path string) (userMode fs.FileMode, botMode fs.FileMode, err error) {
	stat, err := os.Stat(path)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}

	botMode = modeACLReadWrite
	userMode = modeACLReadWrite
	if stat.IsDir() {
		userMode = modeACLReadWriteExecute
		botMode = modeACLReadExecute
	}

	return
}

// VerifyACL verifies whether the ACL of the given file allows writes from the
// bot user. Errors may optionally be used as more informational warnings;
// ConfigureACL can be used to correct them, assuming the user has permissions.
func VerifyACL(path string, botUserID string) error {
	current, err := acl.Get(path)
	if err != nil {
		return trace.Wrap(err)
	}

	userMode, botMode, err := desiredPerms(path)
	if err != nil {
		return trace.Wrap(err)
	}

	userTagCount := 0
	errors := []error{}

	for _, entry := range current {
		switch entry.Tag {
		case acl.TagUserObj:
			if entry.Perms != userMode {
				errors = append(errors, trace.BadParameter("user entry has improper permissions %d", entry.Perms))
			}
		case acl.TagGroupObj:
			if entry.Perms != modeACLNone {
				errors = append(errors, trace.BadParameter("group entry has improper permissions %d", entry.Perms))
			}
		case acl.TagOther:
			if entry.Perms != modeACLNone {
				errors = append(errors, trace.BadParameter("other entry has improper permissions %d", entry.Perms))
			}
		case acl.TagMask:
			if entry.Perms != modeACLReadWrite {
				errors = append(errors, trace.BadParameter("mask entry has improper permissions %d", entry.Perms))
			}
		case acl.TagGroup:
			// Group tags currently not allowed.
			errors = append(errors, trace.BadParameter("unexpected group entry found"))
		case acl.TagUser:
			userTagCount++

			if entry.Qualifier != botUserID {
				errors = append(errors, trace.BadParameter("invalid qualifier %q for user entry", entry.Qualifier))
			}

			if entry.Perms != botMode {
				errors = append(errors, trace.BadParameter("invalid permissions %q for bot user entry", entry.Perms))
			}
		}
	}

	if userTagCount == 0 {
		errors = append(errors, trace.BadParameter("no user tag for bot user %q found", botUserID))
	} else if userTagCount > 1 {
		errors = append(errors, trace.BadParameter("unexpected number of user tags"))
	}

	return trace.NewAggregate(errors...)
}

// ConfigureACL configures ACLs of the given file to allow writes from the bot
// user.
func ConfigureACL(path string, botUserID string) error {
	// We fully specify the ACL here to ensure the correct permissions are
	// always set (rather than just appending an "allow" for the bot user).
	// Note: These need to be sorted by tag value.
	userMode, botMode, err := desiredPerms(path)
	if err != nil {
		return trace.Wrap(err)
	}

	desiredACL := acl.ACL{
		{
			// Note: Mask does not apply to user object.
			Tag:   acl.TagUserObj,
			Perms: userMode,
		},
		{
			// Entry allows the bot to write.
			Tag:       acl.TagUser,
			Qualifier: botUserID,
			Perms:     botMode,
		},
		{
			Tag:   acl.TagGroupObj,
			Perms: modeACLNone,
		},
		{
			// Mask is the maximum permissions the ACL can grant. This should
			// match the desired bot permissions.
			Tag:   acl.TagMask,
			Perms: botMode,
		},
		{
			Tag:   acl.TagOther,
			Perms: modeACLNone,
		},
	}

	log.Debugf("Configuring ACL for user %q on path %q: %v", botUserID, path, desiredACL)
	return trace.Wrap(acl.Set(path, desiredACL))
}

// HasACLSupport determines if this binary / system supports ACLs.
func HasACLSupport() (bool, error) {
	// We just assume Linux _can_ support ACLs here, and will test for support
	// at runtime.
	return true, nil
}

// HasSecureWriteSupport determines if `CreateSecure()` should be supported
// on this OS / kernel version. Note that it just checks the kernel version,
// so this should be treated as a fallible hint.
func HasSecureWriteSupport() (bool, error) {
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
