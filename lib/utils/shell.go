/*
Copyright 2015 Gravitational, Inc.

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

package utils

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
	"os/user"
	"strings"
	"syscall"
	"unsafe"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

const (
	DefaultShell = "/bin/sh"
)

// GetLoginShell determines the login shell for a given username
func GetLoginShell(username string) (string, error) {
	// see if the username is valid
	_, err := user.Lookup(username)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// based on stdlib user/lookup_unix.go packages which does not return user shell
	// https://golang.org/src/os/user/lookup_unix.go
	var pwd C.struct_passwd
	var result *C.struct_passwd

	bufSize := C.sysconf(C._SC_GETPW_R_SIZE_MAX)
	if bufSize == -1 {
		bufSize = 1024
	}
	if bufSize <= 0 || bufSize > 1<<20 {
		return "", trace.Errorf("lookupPosixShell: unreasonable _SC_GETPW_R_SIZE_MAX of %d", bufSize)
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
		log.Errorf("lookupPosixShell: lookup username %s: %s", username, syscall.Errno(rv))
		return "", trace.Errorf("cannot determine shell for %s", username)
	}
	shellCmd := strings.TrimSpace(C.GoString(pwd.pw_shell))
	if len(shellCmd) == 0 {
		log.Warnf("no shell specified for %s. using default=%s", username, DefaultShell)
		shellCmd = DefaultShell
	}
	return shellCmd, nil
}
