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

package uacc

/*
#cgo LDFLAGS: -ldl

#include <dlfcn.h>
#include <stdint.h>
#include <stdlib.h>

static int64_t (*teleport_wtmpdb_login_handle)(
	const char *db_path,
	int type,
	const char *user,
	uint64_t usec_login,
	const char *tty,
	const char *rhost,
	const char *service,
	char **error
) = NULL;

#cgo noescape teleport_wtmpdb_login
#cgo nocallback teleport_wtmpdb_login

static int64_t teleport_wtmpdb_login(
	const char *db_path,
	int type,
	const char *user,
	uint64_t usec_login,
	const char *tty,
	const char *rhost,
	const char *service,
	char **error
) {
	return teleport_wtmpdb_login_handle(db_path, type, user, usec_login, tty, rhost, service, error);
}

static int (*teleport_wtmpdb_logout_handle)(
	const char *db_path,
	int64_t id,
	uint64_t usec_logout,
	char **error
) = NULL;

#cgo noescape teleport_wtmpdb_logout
#cgo nocallback teleport_wtmpdb_logout

static int teleport_wtmpdb_logout(const char *db_path, int64_t id, uint64_t usec_logout, char **error) {
	return teleport_wtmpdb_logout_handle(db_path, id, usec_logout, error);
}

#cgo noescape teleport_load_libwtmpdb
#cgo nocallback teleport_load_libwtmpdb

static int teleport_load_libwtmpdb() {
	void *handle = dlopen("libwtmpdb.so.0", RTLD_LAZY);
	if (!handle) {
		return 1;
	}
	teleport_wtmpdb_login_handle = dlsym(handle, "wtmpdb_login");
	if (!teleport_wtmpdb_login_handle) {
		return 1;
	}
	teleport_wtmpdb_logout_handle = dlsym(handle, "wtmpdb_logout");
	if (!teleport_wtmpdb_logout_handle) {
		return 1;
	}
	return 0;
}
*/
import "C"

import (
	"net"
	"sync"
	"time"
	"unsafe"

	"github.com/gravitational/trace"
)

var wtmpdbLoadOnce sync.Once
var wtmpdbLoaded bool

func wtmpdbBackendAvailable() error {
	wtmpdbLoadOnce.Do(func() {
		if C.teleport_load_libwtmpdb() == 0 {
			wtmpdbLoaded = true
		}
	})
	// TODO(espadolini): distinguish between a failure to dlopen (maybe it's not
	// installed) and a failure to dlsym (something is very weird)
	if !wtmpdbLoaded {
		return trace.NotFound("libwtmpdb is not available")
	}
	return nil
}

type wtmpdbBackend struct {
	dbPath string
}

func newWtmpdbBackend(dbPath string) (*WtmpdbBackend, error) {
	if err := wtmpdbBackendAvailable(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &WtmpdbBackend{wtmpdbBackend: wtmpdbBackend{
		dbPath: dbPath,
	}}, nil
}

func (w *wtmpdbBackend) login(ttyName, username string, remote net.Addr, ts time.Time) (int64, error) {
	if err := wtmpdbBackendAvailable(); err != nil {
		return 0, trace.Wrap(err)
	}
	var dbPath *C.char
	if w.dbPath != "" {
		dbPath = constCString(w.dbPath)
	}
	var error *C.char
	id := C.teleport_wtmpdb_login(
		dbPath,
		userProcess,
		constCString(username),
		C.uint64_t(ts.UnixMicro()),
		constCString(ttyName),
		constCString(hostFromAddr(remote)),
		nil,
		&error,
	)
	if id < 0 || error != nil {
		errorStr := C.GoString(error)
		C.free(unsafe.Pointer(error))
		return 0, trace.Errorf("wtmpdb_login: (%d) %+q", id, errorStr)
	}
	return int64(id), nil
}

func (w *wtmpdbBackend) logout(id int64, ts time.Time) error {
	if err := wtmpdbBackendAvailable(); err != nil {
		return trace.Wrap(err)
	}
	var dbPath *C.char
	if w.dbPath != "" {
		dbPath = constCString(w.dbPath)
	}
	var error *C.char
	r := C.teleport_wtmpdb_logout(
		dbPath,
		C.int64_t(id),
		C.uint64_t(ts.UnixMicro()),
		&error,
	)
	if r != 0 || error != nil {
		errorStr := C.GoString(error)
		C.free(unsafe.Pointer(error))
		return trace.Errorf("wtmpdb_logout: (%d) %+q", r, errorStr)
	}
	return nil
}

func constCString(s string) *C.char {
	if s == "" || s[len(s)-1] != 0 {
		s += "\x00"
	}
	return (*C.char)(unsafe.Pointer(unsafe.StringData(s)))
}
