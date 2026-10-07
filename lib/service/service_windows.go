/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

func writeFile(filename string, data []byte) (err error) {
	return WriteOwnerRestrictedFile(filename, data)
}

// WriteOwnerRestrictedFile writes data to filename through a temporary file
// in the same directory that is created with an owner-only DACL, see
// CreateOwnerRestrictedFile. The temporary file is then renamed over filename.
func WriteOwnerRestrictedFile(filename string, data []byte) (err error) {
	f, err := createOwnerRestrictedTemp(filename)
	if err != nil {
		return trace.Wrap(err)
	}
	tmpName := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = f.Write(data); err != nil {
		return trace.ConvertSystemError(err)
	}
	// Flush the contents before the move, so that a crash can't leave filename
	// pointing at a file whose data never reached the disk.
	if err = f.Sync(); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err = f.Close(); err != nil {
		return trace.ConvertSystemError(err)
	}

	// there's no way to do atomic rename in Windows, we do best effort here.
	// Renaming within the same directory keeps the DACL the file was created
	// with, and since it is protected no ACE is inherited from the parent
	// directory either.
	err = os.Rename(tmpName, filename)
	return trace.ConvertSystemError(err)
}

// createOwnerRestrictedTemp creates a new owner-restricted file with a random
// name next to filename, retrying on name collisions like os.CreateTemp does.
func createOwnerRestrictedTemp(filename string) (*os.File, error) {
	const maxAttempts = 10000
	dir, base := filepath.Dir(filename), filepath.Base(filename)
	for range maxAttempts {
		var suffix [8]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		f, err := CreateOwnerRestrictedFile(filepath.Join(dir, base+".tmp"+hex.EncodeToString(suffix[:])))
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return f, nil
	}
	return nil, trace.LimitExceeded("failed to find a free temporary file name next to %s", filename)
}

// CreateOwnerRestrictedFile creates the file at path for writing and fails
// (with an error satisfying os.IsExist) if anything already exists there.
//
// The file is created in a single CreateFile call that both requires the path
// to be new (CREATE_NEW) and hands the kernel the restrictive DACL through
// SECURITY_ATTRIBUTES, so there is never a moment where the file exists with
// the ACL inherited from the parent directory. Windows ignores the permission
// bits passed to os.OpenFile (they only control the read-only attribute) and a
// create-then-restrict sequence leaves a window where another local account
// with inherited read access on the directory can open a handle that stays
// valid after the DACL is tightened. The file is also opened without sharing,
// so no other handle can be opened on it until it is closed.
//
// The DACL only grants access to the current user and to the accounts that are
// effectively root on Windows: LocalSystem and the built-in Administrators
// group (on Linux 0o600 doesn't keep root out either). The DACL is protected,
// so inheritable ACEs of the parent directory (which commonly grant read access
// to all users) are not applied.
func CreateOwnerRestrictedFile(path string) (*os.File, error) {
	sd, err := ownerRestrictedSecurityDescriptor()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, trace.Wrap(err, "converting path %q", path)
	}
	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}
	h, err := windows.CreateFile(
		name,
		windows.GENERIC_WRITE,
		0, // no sharing while the handle is open
		sa,
		windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		// Keep the syscall error so that os.IsExist works for callers.
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}

func ownerRestrictedSecurityDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, trace.Wrap(err, "getting the SID of the current user")
	}

	// https://learn.microsoft.com/en-us/windows/win32/secauthz/security-descriptor-definition-language
	// D:P  a protected discretionary ACL, inheritance from the parent is blocked
	// FA   FILE_ALL_ACCESS
	// SY   LocalSystem
	// BA   built-in Administrators
	sddl := fmt.Sprintf("D:P(A;;FA;;;%s)(A;;FA;;;SY)(A;;FA;;;BA)", user.User.Sid.String())
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, trace.Wrap(err, "parsing security descriptor %q", sddl)
	}
	return sd, nil
}
