//go:build !windows
// +build !windows

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

package shell

/*
#cgo solaris CFLAGS: -D_POSIX_PTHREAD_SEMANTICS
#include <unistd.h>
#include <sys/types.h>
#include <pwd.h>
#include <stdlib.h>

static int mygetpwnam_r(const char *name, struct passwd *pwd,
	char *buf, size_t buflen, struct passwd **result) {
	return getpwnam_r(name, pwd, buf, buflen, result);
}
*/
import "C"

import (
	"context"
	"log/slog"
	"os/user"
	"strings"
	"syscall"
	"unsafe"

	"github.com/gravitational/trace"
)

// getLoginShell determines the login shell for a given username
func getLoginShell(username string) (string, error) {
	// See if the username is valid.
	_, err := user.Lookup(username)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Based on stdlib user/lookup_unix.go packages which does not return
	// user shell: https://golang.org/src/os/user/lookup_unix.go
	var pwd C.struct_passwd
	var result *C.struct_passwd

	bufSize := C.sysconf(C._SC_GETPW_R_SIZE_MAX)
	if bufSize == -1 {
		bufSize = 1024
	}
	if bufSize <= 0 || bufSize > 1<<20 {
		return "", trace.BadParameter("unreasonable _SC_GETPW_R_SIZE_MAX of %d", bufSize)
	}
	buf := C.malloc(C.size_t(bufSize))
	defer C.free(buf)
	var rv C.int
	nameC := C.CString(username)
	defer C.free(unsafe.Pointer(nameC))
	rv = C.mygetpwnam_r(nameC,
		&pwd,
		(*C.char)(buf),
		C.size_t(bufSize),
		&result)
	if rv != 0 || result == nil {
		slog.ErrorContext(context.Background(), "failed looking up username", "username", username, "error", syscall.Errno(rv).Error())
		return "", trace.BadParameter("cannot determine shell for %s", username)
	}

	// If no shell was found, return trace.NotFound to allow the caller to set
	// the default shell.
	shellCmd := strings.TrimSpace(C.GoString(pwd.pw_shell))
	if len(shellCmd) == 0 {
		return "", trace.NotFound("no shell specified for %v", username)
	}

	return shellCmd, nil
}
