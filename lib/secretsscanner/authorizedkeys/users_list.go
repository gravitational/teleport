//go:build !windows

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
#cgo solaris CFLAGS: -D_POSIX_PTHREAD_SEMANTICS
#include <pwd.h>
*/
import "C"

import (
	"os/user"
	"strconv"
)

// getHostUsers returns a list of all users on the host
// from local /etc/passwd file, LDAP, or other user databases.
func getHostUsers() (results []user.User, _ error) {
	C.setpwent()
	var result *C.struct_passwd
	for {
		result = C.getpwent()
		if result == nil {
			break
		}
		results = append(results, passwdC2Go(result))
	}

	C.endpwent()

	return results, nil
}

// passwdC2Go converts `passwd` struct from C to golang native struct
func passwdC2Go(passwdC *C.struct_passwd) user.User {
	return user.User{
		Name:     C.GoString(passwdC.pw_name),
		Username: C.GoString(passwdC.pw_name),
		Uid:      strconv.FormatUint(uint64(passwdC.pw_uid), 10),
		Gid:      strconv.FormatUint(uint64(passwdC.pw_gid), 10),
		HomeDir:  C.GoString(passwdC.pw_dir),
	}
}
