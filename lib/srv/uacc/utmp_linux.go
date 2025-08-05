//go:build linux

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

/*
Package uacc concerns itself with updating the user account database and log on nodes
that a client connects to with an interactive session.
*/
package uacc

// #include <stdlib.h>
// #include "uacc.h"
import "C"

import (
	"net"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// Due to thread safety design in glibc we must serialize all access to the accounting database.
var accountDb sync.Mutex

// Max hostname length as defined by glibc.
const hostMaxLen = 255

// Max username length as defined by glibc.
const userMaxLen = 32

const uaccPathErrMaxLength = 4096

// Sometimes the _UTMP_PATH and _WTMP_PATH macros from glibc are bad, this seems to depend on distro.
// I asked around on IRC, no one really knows why. I suspect it's another
// archaic remnant of old Unix days and that a cleanup is long overdue.
//
// In the meantime, we just try to resolve from these paths instead.

const (
	defaultUtmpFilePath = "/var/run/utmp"
	defaultWtmpFilePath = "/var/log/wtmp"
	// wtmpAltFilePath exists only because on some system the path is different.
	// It's being used when the wtmp path is not provided and the wtmpFilePath doesn't exist.
	wtmpAltFilePath     = "/var/run/wtmp"
	defaultBtmpFilePath = "/var/log/btmp"

	defaultUtmpxFilePath = "/var/run/utmpx"
	defaultWtmpxFilePath = "/var/log/wtmpx"
	wtmpxAltFilePath     = "/var/run/wtmpx"
	defaultBtmpxFilePath = "/var/log/btmpx"
)

type UtmpBackend struct {
	utmpPath string
	wtmpPath string
	btmpPath string
}

func getTargetFile(candidates ...string) (string, error) {
	for _, candidate := range candidates {
		if utils.FileExists(candidate) {
			return candidate, nil
		}
	}
	return "", trace.BadParameter("no target files exist")
}

func NewUtmpBackend(utmpFile, wtmpFile, btmpFile string) (*UtmpBackend, error) {
	utmp, err := getTargetFile(utmpFile, defaultUtmpxFilePath, defaultUtmpFilePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wtmp, err := getTargetFile(wtmpFile, defaultWtmpxFilePath, wtmpxAltFilePath, defaultWtmpFilePath, wtmpAltFilePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	btmp, err := getTargetFile(btmpFile, defaultBtmpxFilePath, defaultBtmpFilePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &UtmpBackend{
		utmpPath: utmp,
		wtmpPath: wtmp,
		btmpPath: btmp,
	}, nil
}

func (w *UtmpBackend) Login(ttyName, username string, remote net.Addr, ts time.Time) (string, error) {
	// String parameter validation.
	if len(username) > userMaxLen {
		return "", trace.BadParameter("username length exceeds OS limits")
	}
	addr := utils.FromAddr(remote)
	if len(addr.Host()) > hostMaxLen {
		return "", trace.BadParameter("hostname length exceeds OS limits")
	}
	if len(ttyName) > (int)(C.max_len_tty_name()-1) {
		return "", trace.BadParameter("tty name length exceeds OS limits")
	}

	// Convert Go strings into C strings that we can pass over ffi.
	cUtmpPath := C.CString(w.utmpPath)
	defer C.free(unsafe.Pointer(cUtmpPath))

	cWtmpPath := C.CString(w.wtmpPath)
	defer C.free(unsafe.Pointer(cWtmpPath))

	cUsername := C.CString(username)
	defer C.free(unsafe.Pointer(cUsername))
	cHostname := C.CString(addr.Host())
	defer C.free(unsafe.Pointer(cHostname))
	cTtyName := C.CString(strings.TrimPrefix(ttyName, "/dev/"))
	defer C.free(unsafe.Pointer(cTtyName))
	cIDName := C.CString(strings.TrimPrefix(ttyName, "/dev/pts/"))
	defer C.free(unsafe.Pointer(cIDName))

	// Convert IPv6 array into C integer format.
	remoteIP, err := PrepareAddr(remote)
	if err != nil {
		return "", trace.Wrap(err)
	}
	cIP := convertIPToC(remoteIP)
	secondsElapsed, microsFraction := cTimestamp(ts)

	accountDb.Lock()
	defer accountDb.Unlock()
	var uaccPathErr [uaccPathErrMaxLength]C.char
	status, errno := C.uacc_add_utmp_entry(cUtmpPath, cWtmpPath, cUsername, cHostname, &cIP[0], cTtyName, cIDName, secondsElapsed, microsFraction, &uaccPathErr[0])

	switch status {
	case C.UACC_UTMP_MISSING_PERMISSIONS:
		return "", trace.AccessDenied("missing permissions to write to utmp/wtmp")
	case C.UACC_UTMP_WRITE_ERROR:
		return "", trace.AccessDenied("failed to add entry to utmp database")
	case C.UACC_UTMP_FAILED_OPEN:
		return "", trace.AccessDenied("failed to open user account database, code: %d", errno)
	case C.UACC_UTMP_FAILED_TO_SELECT_FILE:
		return "", trace.BadParameter("failed to select file")
	case C.UACC_UTMP_PATH_DOES_NOT_EXIST:
		return "", trace.NotFound("user accounting files are missing from the system, running in a container?")
	default:
		return ttyName, decodeUnknownError(int(status), uaccPathErr)
	}
}

func (w *UtmpBackend) Logout(ttyName string, ts time.Time) error {
	// String parameter validation.
	if len(ttyName) > (int)(C.max_len_tty_name()-1) {
		return trace.BadParameter("tty name length exceeds OS limits")
	}

	// Convert Go strings into C strings that we can pass over ffi.
	cUtmpPath := C.CString(w.utmpPath)
	defer C.free(unsafe.Pointer(cUtmpPath))
	cWtmpPath := C.CString(w.wtmpPath)
	defer C.free(unsafe.Pointer(cWtmpPath))

	cTtyName := C.CString(strings.TrimPrefix(ttyName, "/dev/"))
	defer C.free(unsafe.Pointer(cTtyName))
	secondsElapsed, microsFraction := cTimestamp(ts)

	accountDb.Lock()
	defer accountDb.Unlock()
	var uaccPathErr [uaccPathErrMaxLength]C.char
	status, errno := C.uacc_mark_utmp_entry_dead(cUtmpPath, cWtmpPath, cTtyName, secondsElapsed, microsFraction, &uaccPathErr[0])

	switch status {
	case C.UACC_UTMP_MISSING_PERMISSIONS:
		return trace.AccessDenied("missing permissions to write to utmp/wtmp")
	case C.UACC_UTMP_WRITE_ERROR:
		return trace.AccessDenied("failed to add entry to utmp database")
	case C.UACC_UTMP_READ_ERROR:
		return trace.AccessDenied("failed to read and search utmp database")
	case C.UACC_UTMP_FAILED_OPEN:
		return trace.AccessDenied("failed to open user account database, code: %d", errno)
	case C.UACC_UTMP_FAILED_TO_SELECT_FILE:
		return trace.BadParameter("failed to select file")
	case C.UACC_UTMP_PATH_DOES_NOT_EXIST:
		return trace.NotFound("user accounting files are missing from the system, running in a container?")
	default:
		return decodeUnknownError(int(status), uaccPathErr)
	}
}

func (w *UtmpBackend) FailedLogin(username string, remote net.Addr, ts time.Time) error {
	// String parameter validation.
	if len(username) > userMaxLen {
		return trace.BadParameter("username length exceeds OS limits")
	}
	addr := utils.FromAddr(remote)
	if len(addr.Host()) > hostMaxLen {
		return trace.BadParameter("hostname length exceeds OS limits")
	}

	// Convert Go strings into C strings that we can pass over ffi.
	cBtmpPath := C.CString(w.btmpPath)
	defer C.free(unsafe.Pointer(cBtmpPath))
	cUsername := C.CString(username)
	defer C.free(unsafe.Pointer(cUsername))
	cHostname := C.CString(addr.Host())
	defer C.free(unsafe.Pointer(cHostname))

	// Convert IPv6 array into C integer format.
	remoteIP, err := PrepareAddr(remote)
	if err != nil {
		return trace.Wrap(err)
	}
	cIP := convertIPToC(remoteIP)

	secondsElapsed, microsFraction := cTimestamp(ts)

	accountDb.Lock()
	defer accountDb.Unlock()
	var uaccPathErr [uaccPathErrMaxLength]C.char
	status, errno := C.uacc_add_btmp_entry(cBtmpPath, cUsername, cHostname, &cIP[0], secondsElapsed, microsFraction, &uaccPathErr[0])
	switch status {
	case C.UACC_UTMP_MISSING_PERMISSIONS:
		return trace.AccessDenied("missing permissions to write to btmp")
	case C.UACC_UTMP_WRITE_ERROR:
		return trace.AccessDenied("failed to add entry to btmp")
	case C.UACC_UTMP_FAILED_OPEN:
		return trace.AccessDenied("failed to open user account database, code: %d", errno)
	case C.UACC_UTMP_FAILED_TO_SELECT_FILE:
		return trace.BadParameter("failed to select file")
	case C.UACC_UTMP_PATH_DOES_NOT_EXIST:
		return trace.NotFound("user accounting files are missing from the system, running in a container?")
	default:
		return decodeUnknownError(int(status), uaccPathErr)
	}
}

func (w *UtmpBackend) IsUserLoggedIn(username string) (bool, error) {
	return w.IsUserInFile(w.utmpPath, username)
}

func (w *UtmpBackend) IsUserInFile(utmp string, username string) (bool, error) {
	if len(username) > userMaxLen {
		return false, trace.BadParameter("username length exceeds OS limits")
	}

	// Convert Go strings into C strings that we can pass over ffi.
	var cUtmpPath *C.char
	if len(utmp) > 0 {
		cUtmpPath = C.CString(utmp)
		defer C.free(unsafe.Pointer(cUtmpPath))
	}
	cUsername := C.CString(username)
	defer C.free(unsafe.Pointer(cUsername))

	accountDb.Lock()
	defer accountDb.Unlock()
	var uaccPathErr [uaccPathErrMaxLength]C.char
	status, errno := C.uacc_has_entry_with_user(cUtmpPath, cUsername, &uaccPathErr[0])

	switch status {
	case C.UACC_UTMP_FAILED_OPEN:
		return false, trace.AccessDenied("failed to open user account database, code: %d", errno)
	case C.UACC_UTMP_ENTRY_DOES_NOT_EXIST:
		return false, nil
	case C.UACC_UTMP_FAILED_TO_SELECT_FILE:
		return false, trace.BadParameter("failed to select file")
	case C.UACC_UTMP_PATH_DOES_NOT_EXIST:
		return false, trace.NotFound("user accounting files are missing from the system, running in a container?")
	default:
		return status == 0, decodeUnknownError(int(status), uaccPathErr)
	}
}

func convertIPToC(remote [4]int32) [4]C.int32_t {
	var cIP [4]C.int32_t
	for i := 0; i < 4; i++ {
		cIP[i] = (C.int32_t)(remote[i])
	}
	return cIP
}

func cTimestamp(ts time.Time) (C.int32_t, C.int32_t) {
	secondsElapsed := (C.int32_t)(ts.Unix())
	microsFraction := (C.int32_t)((ts.UnixNano() % int64(time.Second)) / int64(time.Microsecond))
	return secondsElapsed, microsFraction
}

func decodeUnknownError(status int, rawUaccPathErr [uaccPathErrMaxLength]C.char) error {
	if status == 0 {
		return nil
	}

	uaccPathErrBytes := make([]byte, 0, uaccPathErrMaxLength)
	for _, char := range rawUaccPathErr {
		uaccPathErrBytes = append(uaccPathErrBytes, (byte)(char))
	}
	uaccPathErr := string(uaccPathErrBytes)

	if uaccPathErr != "" {
		return trace.Errorf("unknown error with code %d and data %v", status, uaccPathErr)
	}

	return trace.Errorf("unknown error with code %d", status)
}
