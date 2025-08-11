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

package uacc

import (
	"encoding/binary"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// UserAccountHandler handles user accounting across multiple backends.
type UserAccountHandler struct {
	utmp         *UtmpBackend
	wtmpdb       *WtmpdbBackend
	isPAMEnabled bool
}

// UaccConfig configures NewUserAccounting.
type UaccConfig struct {
	// IsPAMEnabled indicates if PAM is in use (affects wtmpdb login/out).
	IsPAMEnabled bool
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
func NewUserAccountHandler(cfg UaccConfig) (*UserAccountHandler, error) {
	uacc := &UserAccountHandler{
		isPAMEnabled: cfg.IsPAMEnabled,
	}
	if utmp, err := NewUtmpBackend(cfg.UtmpFile, cfg.WtmpFile, cfg.BtmpFile); err == nil {
		uacc.utmp = utmp
	}
	if wtmpdb, err := NewWtmpdbBackend(cfg.WtmpdbFile); err == nil {
		uacc.wtmpdb = wtmpdb
	}
	if uacc.utmp == nil && uacc.wtmpdb == nil {
		return nil, trace.BadParameter("no valid backends available")
	}
	return uacc, nil
}

// Session represents a login session. It must be closed when the session is finished.
type Session struct {
	uacc      *UserAccountHandler
	utmpKey   string
	wtmpdbKey *int64
}

// OpenSession opens a new login session. It will succeed if at least one backend succeeds.
func (uacc *UserAccountHandler) OpenSession(tty *os.File, username string, remote net.Addr) (*Session, error) {
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
	errors := make([]error, 0, 2)
	if uacc.utmp != nil {
		if err := uacc.utmp.Login(ttyName, username, remote, loginTime); err == nil {
			anySucceeded = true
			session.utmpKey = ttyName
		} else {
			errors = append(errors, err)
		}
	}
	if uacc.wtmpdb != nil && !uacc.isPAMEnabled {
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

// Close closes the login session. It will succeed if at least one backend succeeds.
func (session *Session) Close() error {
	logoutTime := time.Now()
	var anySucceeded bool
	errors := make([]error, 0, 2)
	if session.utmpKey != "" {
		if session.uacc.utmp == nil {
			return trace.BadParameter("utmp not supported")
		}
		if err := session.uacc.utmp.Logout(session.utmpKey, logoutTime); err == nil {
			anySucceeded = true
		} else {
			errors = append(errors, err)
		}
	}
	if session.wtmpdbKey != nil {
		if session.uacc.wtmpdb == nil {
			return trace.BadParameter("wtmpdb not supported")
		} else if session.uacc.isPAMEnabled {
			return trace.BadParameter("wtmpdb login/logout is handled by PAM")
		}
		if err := session.uacc.wtmpdb.Logout(*session.wtmpdbKey, logoutTime); err == nil {
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
		if err := uacc.utmp.FailedLogin(username, remote, time.Now()); err != nil {
			return trace.Wrap(err)
		}
	}
	// wtmpdb doesn't log failed logins
	return nil
}

// prepareAddr parses and transforms a net.Addr into a format usable by other uacc functions.
func prepareAddr(addr net.Addr) ([4]int32, error) {
	stringIP, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return [4]int32{}, trace.Wrap(err)
	}
	ip := net.ParseIP(stringIP)
	rawV6 := ip.To16()

	// this case can occur if the net.Addr isn't in an expected IP format, in that case, ignore it
	// we have to guard against this because the net.Addr internal format is implementation specific
	if rawV6 == nil {
		return [4]int32{}, nil
	}

	groupedV6 := [4]int32{}
	for i := range groupedV6 {
		// some bit magic to convert the byte array into 4 32 bit integers
		groupedV6[i] = int32(binary.LittleEndian.Uint32(rawV6[i*4 : (i+1)*4]))
	}
	return groupedV6, nil
}
