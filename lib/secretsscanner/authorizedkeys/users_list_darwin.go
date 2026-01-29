/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package authorizedkeys

/*
#include <pwd.h>
*/
import "C"

import (
	"os/user"
	"runtime"
	"strconv"

	"github.com/gravitational/trace"
)

// getHostUsers returns the list of all users on the host from the user
// directory (depending on system configuration this can be /etc/passwd,
// LDAP...).
func getHostUsers() ([]user.User, error) {
	// on darwin the setpwent/getpwent/endpwent functions use thread-local
	// storage so there's no need for a global lock but we must call the whole
	// sequence from the same thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	C.setpwent()
	defer C.endpwent()

	var results []user.User
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
func passwdC2Go(passwdC *C.struct_passwd) user.User {
	name := C.GoString(passwdC.pw_name)
	return user.User{
		Name:     name,
		Username: name,
		Uid:      strconv.FormatUint(uint64(passwdC.pw_uid), 10),
		Gid:      strconv.FormatUint(uint64(passwdC.pw_gid), 10),
		HomeDir:  C.GoString(passwdC.pw_dir),
	}
}
