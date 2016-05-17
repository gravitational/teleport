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
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"syscall"
	"unsafe"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
)

var osxUserShellRegexp = regexp.MustCompile("UserShell: (/[^ ]+)\n")

// GetLoginShell determines the login shell for a given username
func GetLoginShell(username string) (string, error) {
	user, err := user.Lookup(username)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// func to determine user shell on OSX:
	forMac := func() (string, error) {
		dir := "Local/Default/Users/" + username
		out, err := exec.Command("dscl", "localhost", "-read", dir, "UserShell").Output()
		if err != nil {
			log.Warn(err)
			return "", trace.Errorf("cannot determine shell for %s", username)
		}
		m := osxUserShellRegexp.FindStringSubmatch(string(out))
		shell := m[1]
		if shell == "" {
			return "", trace.Errorf("dscl output parsing error getting shell for %s", username)
		}
		return shell, nil
	}
	// func to determine user shell on other unixes (linux)
	forUnix := func() (string, error) {
		// didn't find it in /etc/passwd? try
		shell, err := lookupPosixShell(user.Username)
		if err != nil {
			log.Error(err)
			return "", trace.Errorf("cannot determine shell for %s", username)
		}
		return shell, nil
	}
	if runtime.GOOS == "darwin" {
		return forMac()
	}
	return forUnix()
}

// lookupPosixShell determines the login shell for a given username using posix system call
func lookupPosixShell(username string) (string, error) {
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
	if rv != 0 {
		return "", trace.Errorf("lookupPosixShell: lookup username %s: %s", username, syscall.Errno(rv))
	}
	if result == nil {
		return "", trace.Errorf("lookupPosixShell: unknown username %s", username)
	}
	return C.GoString(pwd.pw_shell), nil
}
