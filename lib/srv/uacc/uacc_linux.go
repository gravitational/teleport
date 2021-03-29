// +build linux

/*
Copyright 2021 Gravitational, Inc.

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

/*
Package uacc concerns itself with updating the user account database and log on nodes
that a client connects to with an interactive session.
*/
package uacc

// #include <stdlib.h>
// #include "uacc.h"
import "C"

import (
	"os"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gravitational/trace"
)

// Due to thread safety design in glibc we must serialize all access to the accounting database.
var accountDb sync.Mutex

// Max length of username and hostname as defined by glibc.
const nameMaxLen = 255

// Open writes a new entry to the utmp database with a tag of `USER_PROCESS`.
// This should be called when an interactive session is started.
//
// `username`: Name of the user the interactive session is running under.
// `hostname`: Name of the system the user is logged into.
// `remoteAddrV6`: IPv6 address of the remote host.
// `ttyName`: Name of the TTY including the `/dev/` prefix.
func Open(utmpPath, wtmpPath string, username, hostname string, remote [4]int32, tty *os.File) error {
	ttyName, err := os.Readlink(tty.Name())
	if err != nil {
		return trace.Errorf("failed to resolve soft proc tty link: %v", err)
	}

	// String parameter validation.
	if len(username) > nameMaxLen {
		return trace.BadParameter("username length exceeds OS limits")
	}
	if len(hostname) > nameMaxLen {
		return trace.BadParameter("hostname length exceeds OS limits")
	}
	if len(ttyName) > (int)(C.max_len_tty_name()-1) {
		return trace.BadParameter("tty name length exceeds OS limits")
	}

	// Convert Go strings into C strings that we can pass over ffi.
	var cUtmpPath *C.char = nil
	var cWtmpPath *C.char = nil
	if len(utmpPath) > 0 {
		cUtmpPath = C.CString(utmpPath)
		defer C.free(unsafe.Pointer(cUtmpPath))
	}
	if len(wtmpPath) > 0 {
		cWtmpPath = C.CString(wtmpPath)
		defer C.free(unsafe.Pointer(cWtmpPath))
	}
	cUsername := C.CString(username)
	defer C.free(unsafe.Pointer(cUsername))
	cHostname := C.CString(hostname)
	defer C.free(unsafe.Pointer(cHostname))
	cTtyName := C.CString(strings.TrimPrefix(ttyName, "/dev/"))
	defer C.free(unsafe.Pointer(cTtyName))
	cIDName := C.CString(strings.TrimPrefix(ttyName, "/dev/pts/"))
	defer C.free(unsafe.Pointer(cIDName))

	// Convert IPv6 array into C integer format.
	cIP := [4]C.int{0, 0, 0, 0}
	for i := 0; i < 4; i++ {
		cIP[i] = (C.int)(remote[i])
	}

	timestamp := time.Now()
	secondsElapsed := (C.int32_t)(timestamp.Unix())
	microsFraction := (C.int32_t)((timestamp.UnixNano() % int64(time.Second)) / int64(time.Microsecond))

	accountDb.Lock()
	status := C.uacc_add_utmp_entry(cUtmpPath, cWtmpPath, cUsername, cHostname, &cIP[0], cTtyName, cIDName, secondsElapsed, microsFraction)
	accountDb.Unlock()

	switch status {
	case C.UACC_UTMP_MISSING_PERMISSIONS:
		return trace.AccessDenied("missing permissions to write to utmp/wtmp")
	case C.UACC_UTMP_WRITE_ERROR:
		return trace.AccessDenied("failed to add entry to utmp database")
	case C.UACC_UTMP_FAILED_OPEN:
		code := C.get_errno()
		return trace.AccessDenied("failed to open user account database, code: %d", code)
	case C.UACC_UTMP_FAILED_TO_SELECT_FILE:
		return trace.BadParameter("failed to select file")
	default:
		if status != 0 {
			return trace.Errorf("unknown error with code %d", status)
		}

		return nil
	}
}

// Close marks an entry in the utmp database as DEAD_PROCESS.
// This should be called when an interactive session exits.
//
// The `ttyName` parameter must be the name of the TTY including the `/dev/` prefix.
func Close(utmpPath, wtmpPath string, tty *os.File) error {
	ttyName, err := os.Readlink(tty.Name())
	if err != nil {
		return trace.Errorf("failed to resolve soft proc tty link: %v", err)
	}

	// String parameter validation.
	if len(ttyName) > (int)(C.max_len_tty_name()-1) {
		return trace.BadParameter("tty name length exceeds OS limits")
	}

	// Convert Go strings into C strings that we can pass over ffi.
	var cUtmpPath *C.char = nil
	var cWtmpPath *C.char = nil
	if len(utmpPath) > 0 {
		cUtmpPath = C.CString(utmpPath)
		defer C.free(unsafe.Pointer(cUtmpPath))
	}
	if len(wtmpPath) > 0 {
		cWtmpPath = C.CString(wtmpPath)
		defer C.free(unsafe.Pointer(cWtmpPath))
	}
	cTtyName := C.CString(strings.TrimPrefix(ttyName, "/dev/"))
	defer C.free(unsafe.Pointer(cTtyName))

	timestamp := time.Now()
	secondsElapsed := (C.int32_t)(timestamp.Unix())
	microsFraction := (C.int32_t)((timestamp.UnixNano() % int64(time.Second)) / int64(time.Microsecond))

	accountDb.Lock()
	status := C.uacc_mark_utmp_entry_dead(cUtmpPath, cWtmpPath, cTtyName, secondsElapsed, microsFraction)
	accountDb.Unlock()

	switch status {
	case C.UACC_UTMP_MISSING_PERMISSIONS:
		return trace.AccessDenied("missing permissions to write to utmp/wtmp")
	case C.UACC_UTMP_WRITE_ERROR:
		return trace.AccessDenied("failed to add entry to utmp database")
	case C.UACC_UTMP_READ_ERROR:
		return trace.AccessDenied("failed to read and search utmp database")
	case C.UACC_UTMP_FAILED_OPEN:
		code := C.get_errno()
		return trace.AccessDenied("failed to open user account database, code: %d", code)
	case C.UACC_UTMP_FAILED_TO_SELECT_FILE:
		return trace.BadParameter("failed to select file")
	default:
		if status != 0 {
			return trace.Errorf("unknown error with code %d", status)
		}

		return nil
	}
}

// UserWithPtyInDatabase checks the user accounting database for the existence of an USER_PROCESS entry with the given username.
func UserWithPtyInDatabase(utmpPath string, username string) error {
	if len(username) > nameMaxLen {
		return trace.BadParameter("username length exceeds OS limits")
	}

	// Convert Go strings into C strings that we can pass over ffi.
	var cUtmpPath *C.char = nil
	if len(utmpPath) > 0 {
		cUtmpPath = C.CString(utmpPath)
		defer C.free(unsafe.Pointer(cUtmpPath))
	}
	cUsername := C.CString(username)
	defer C.free(unsafe.Pointer(cUsername))

	accountDb.Lock()
	status := C.uacc_has_entry_with_user(cUtmpPath, cUsername)
	accountDb.Unlock()

	switch status {
	case C.UACC_UTMP_FAILED_OPEN:
		code := C.get_errno()
		return trace.AccessDenied("failed to open user account database, code: %d", code)
	case C.UACC_UTMP_ENTRY_DOES_NOT_EXIST:
		return trace.NotFound("user not found")
	case C.UACC_UTMP_FAILED_TO_SELECT_FILE:
		return trace.BadParameter("failed to select file")
	default:
		if status != 0 {
			return trace.Errorf("unknown error with code %d", status)
		}

		return nil
	}
}
