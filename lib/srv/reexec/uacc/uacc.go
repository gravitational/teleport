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

// Package uacc implements user accounting on Linux systems. There are two
// supported backends, utmp and wtmpdb. Operations in this package attempt to use
// all available backends and succeed if at least one backend succeeds.
//
// # utmp
//
// utmp is the classic Unix user accounting system. Current sessions are logged in
// the utmp file, session history is logged in the wtmp file, and failed logins are
// logged in the btmp file (or their *tmpx counterparts, the file format is
// identical on Linux). Teleport writes to the *tmp files via the libc API in
// <utmp.h>.
//
// # wtmpdb
//
// [wtmpdb] is the Y2038-safe successor to utmp. Session history is logged in the
// wtmp.db sqlite database. Teleport writes to wtmpdb with sqlite directly instead
// of using libwtmpdb.
//
// [wtmpdb]: https://github.com/thkukuk/wtmpdb
package uacc

import (
	"context"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// UserAccountHandler handles user accounting across multiple backends.
type UserAccountHandler struct {
	utmp   *UtmpBackend
	wtmpdb *WtmpdbBackend
}

// UaccConfig configures NewUserAccounting.
type UaccConfig struct {
	// WtmpdbFile is the path to an alternate wtmpdb database.
	WtmpdbFile string
	// UtmpFile is the path to an alternate utmp file.
	UtmpFile string
	// WtmpFile is the path to an alternate wtmp file.
	WtmpFile string
	// BtmpFile is the path to an alternate btmp file.
	BtmpFile string
}

// NewUserAccountHandler creates a new UserAccountHandler.
func NewUserAccountHandler(cfg UaccConfig) *UserAccountHandler {
	uacc := &UserAccountHandler{}
	ctx := context.Background()
	//nolint:staticcheck // SA4023 Always non-nil error on non-Linux hosts
	utmp, utmpErr := NewUtmpBackend(cfg.UtmpFile, cfg.WtmpFile, cfg.BtmpFile)
	//nolint:staticcheck // SA4023 Always non-nil error on non-Linux hosts
	if utmpErr == nil {
		uacc.utmp = utmp
		slog.DebugContext(ctx, "utmp user accounting is active")
	}
	wtmpdb, wtmpdbErr := NewWtmpdbBackend(cfg.WtmpdbFile)
	if wtmpdbErr == nil {
		uacc.wtmpdb = wtmpdb
		slog.DebugContext(ctx, "wtmpdb user accounting is active")
	}
	if uacc.utmp == nil && uacc.wtmpdb == nil {
		slog.DebugContext(
			ctx, "no user accounting backends are available, sessions will not be logged locally",
			"utmp_error", utmpErr, "wtmpdb_error", wtmpdbErr,
		)
	}
	return uacc
}

// Session represents a login session. It must be closed when the session is finished.
type Session struct {
	uacc      *UserAccountHandler
	utmpKey   string
	wtmpdbKey *int64
}

// OpenSession opens a new login session. It will attempt to open a session
// with all backends and will succeed if at least one backend succeeds.
func (uacc *UserAccountHandler) OpenSession(tty *os.File, username string, remote net.Addr) (*Session, error) {
	if uacc.utmp == nil && uacc.wtmpdb == nil {
		return &Session{}, nil
	}

	loginTime := time.Now()
	ttyFullName, err := os.Readlink(tty.Name())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ttyName := strings.TrimPrefix(ttyFullName, "/dev/")

	var anySucceeded bool
	session := &Session{
		uacc: uacc,
	}
	var errors []error
	if uacc.utmp != nil {
		//nolint:staticcheck // SA4023 Always non-nil error on non-Linux hosts
		if err := uacc.utmp.Login(ttyName, username, remote, loginTime); err == nil {
			anySucceeded = true
			session.utmpKey = ttyName
		} else {
			errors = append(errors, err)
		}
	}
	if uacc.wtmpdb != nil {
		key, err := uacc.wtmpdb.Login(ttyName, username, remote, loginTime)
		if err == nil {
			anySucceeded = true
			session.wtmpdbKey = &key
		} else {
			errors = append(errors, err)
		}
	}
	if !anySucceeded {
		return nil, trace.NewAggregate(errors...)
	}
	return session, nil
}

// Close closes the login session. It will attempt to close the session
// on all backends and will succeed if at least one backend succeeds.
func (session *Session) Close() error {
	if session.utmpKey == "" && session.wtmpdbKey == nil {
		return nil
	}

	logoutTime := time.Now()
	var anySucceeded bool
	var errors []error
	if session.utmpKey != "" {
		utmp := session.uacc.utmp
		if utmp == nil {
			return trace.BadParameter("utmp not supported")
		}
		//nolint:staticcheck // SA4023 Always non-nil error on non-Linux hosts
		if err := utmp.Logout(session.utmpKey, logoutTime); err == nil {
			anySucceeded = true
		} else {
			errors = append(errors, err)
		}
	}
	if session.wtmpdbKey != nil {
		wtmpdb := session.uacc.wtmpdb
		if wtmpdb == nil {
			return trace.BadParameter("wtmpdb not supported")
		}
		if err := wtmpdb.Logout(*session.wtmpdbKey, logoutTime); err == nil {
			anySucceeded = true
		} else {
			errors = append(errors, err)
		}
	}
	if !anySucceeded {
		return trace.NewAggregate(errors...)
	}
	return nil
}

// FailedLogin logs a failed login attempt.
func (uacc *UserAccountHandler) FailedLogin(username string, remote net.Addr) error {
	if uacc.utmp != nil {
		//nolint:staticcheck // SA4023 Always non-nil error on non-Linux hosts
		if err := uacc.utmp.FailedLogin(username, remote, time.Now()); err != nil {
			return trace.Wrap(err)
		}
	}
	// wtmpdb doesn't log failed logins.
	return nil
}
