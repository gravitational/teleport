//go:build !darwin

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
#cgo CFLAGS: -D_POSIX_PTHREAD_SEMANTICS -D__USE_MISC
#include <unistd.h>
#include <sys/types.h>
#include <pwd.h>
#include <stdlib.h>
*/
import "C"

import (
	"os/user"

	"github.com/gravitational/trace"
)

// getHostUsers returns a list of all users on the host
// from local /etc/passwd file, LDAP, or other user databases.
func getHostUsers() (results []user.User, _ error) {

	bufSize := C.sysconf(C._SC_GETPW_R_SIZE_MAX)
	if bufSize == -1 {
		bufSize = 16384
	}
	if bufSize <= 0 || bufSize > 1<<20 {
		return nil, trace.BadParameter("unreasonable _SC_GETPW_R_SIZE_MAX of %d", bufSize)
	}
	buf := C.malloc(C.size_t(bufSize))
	defer C.free(buf)

	C.setpwent()

	var pwdBuf C.struct_passwd
	for {
		var result *C.struct_passwd
		rv := C.getpwent_r(&pwdBuf, (*C.char)(buf), C.size_t(bufSize), &result)
		if rv != 0 || result == nil {
			break
		}
		results = append(results, passwdC2Go(&pwdBuf))
	}

	C.endpwent()

	return results, nil
}
