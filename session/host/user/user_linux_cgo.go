//go:build linux && cgo

// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package user

/*
#define _XOPEN_SOURCE 500
#include <pwd.h>
*/
import "C"

import (
	osuser "os/user"
	"strconv"
	"sync"

	"github.com/gravitational/trace"
)

var packageMU sync.Mutex

// Lookup wraps [os/user.Lookup] with process-wide serialization.
func Lookup(username string) (*osuser.User, error) {
	packageMU.Lock()
	defer packageMU.Unlock()
	return osuser.Lookup(username)
}

// LookupId wraps [os/user.LookupId] with process-wide serialization.
func LookupId(id string) (*osuser.User, error) {
	packageMU.Lock()
	defer packageMU.Unlock()
	return osuser.LookupId(id)
}

// LookupGroup wraps [os/user.LookupGroup] with process-wide serialization.
func LookupGroup(name string) (*osuser.Group, error) {
	packageMU.Lock()
	defer packageMU.Unlock()
	return osuser.LookupGroup(name)
}

// LookupGroupId wraps [os/user.LookupGroupId] with process-wide serialization.
func LookupGroupId(id string) (*osuser.Group, error) {
	packageMU.Lock()
	defer packageMU.Unlock()
	return osuser.LookupGroupId(id)
}

// Current wraps [os/user.Current] with process-wide serialization.
func Current() (*osuser.User, error) {
	packageMU.Lock()
	defer packageMU.Unlock()
	return osuser.Current()
}

// LookGroupIdsup wraps [os/user.User.GroupIds] with process-wide serialization.
func GroupIds(u *osuser.User) ([]string, error) {
	if u == nil {
		return nil, trace.BadParameter("user cannot be nil")
	}
	packageMU.Lock()
	defer packageMU.Unlock()
	return u.GroupIds()
}

// GetHostUsers returns the list of all users on the host from the user
// directory (depending on system configuration this can be /etc/passwd,
// LDAP...).
func GetHostUsers() ([]osuser.User, error) {
	// A lock should be acquired when using MT-Unsafe race:pwent functions (i.e.
	// setpwent/getpwent/endpwent). Since `getpwent` can follow the same path
	// into libnss we take the package lock.
	packageMU.Lock()
	defer packageMU.Unlock()

	C.setpwent()
	defer C.endpwent()

	var results []osuser.User
	for {
		result, err := C.getpwent()
		// cgo error convention, check the return value before errno
		if result != nil {
			results = append(results, passwdC2Go(result))
			continue
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return results, nil
	}
}

// passwdC2Go converts `passwd` struct from C to golang native struct
func passwdC2Go(passwdC *C.struct_passwd) osuser.User {
	name := C.GoString(passwdC.pw_name)
	return osuser.User{
		Name:     name,
		Username: name,
		Uid:      strconv.FormatUint(uint64(passwdC.pw_uid), 10),
		Gid:      strconv.FormatUint(uint64(passwdC.pw_gid), 10),
		HomeDir:  C.GoString(passwdC.pw_dir),
	}
}
