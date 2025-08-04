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
	"encoding/json"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

type userKey struct {
	Utmp   string
	Wtmpdb string
}

type UserAccounting struct {
	utmp         *utmpBackend
	wtmpdb       *wtmpdbBackend
	isPAMEnabled bool
}

type UaccConfig struct {
	IsPAMEnabled bool
	WtmpdbFile   string
	Utmp         string
	Wtmp         string
	Btmp         string
}

func NewUserAccounting(cfg UaccConfig) (*UserAccounting, error) {
	uacc := &UserAccounting{
		isPAMEnabled: cfg.IsPAMEnabled,
	}
	if utmp, err := newUtmpBackend(cfg.Utmp, cfg.Wtmp, cfg.Btmp); err == nil {
		uacc.utmp = utmp
	}
	if wtmpdb, err := newWtmpdb(cfg.WtmpdbFile); err == nil {
		uacc.wtmpdb = wtmpdb
	}
	if uacc.utmp == nil && uacc.wtmpdb == nil {
		return nil, trace.BadParameter("no valid backends available")
	}
	return uacc, nil
}

func (uacc *UserAccounting) Login(tty *os.File, username string, remote net.Addr, ts time.Time) ([]byte, error) {
	ttyName, err := GetTTYName(tty)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var backendKeys userKey
	var anySucceeded bool
	errors := make([]error, 0, 2)
	if uacc.utmp != nil {
		utmpKey, err := uacc.utmp.Login(ttyName, username, remote, ts)
		if err == nil {
			anySucceeded = true
			backendKeys.Utmp = utmpKey
		} else {
			errors = append(errors, err)
		}
	}
	if uacc.wtmpdb != nil && !uacc.isPAMEnabled {
		wtmpdbKey, err := uacc.wtmpdb.Login(ttyName, username, remote, ts)
		if err == nil {
			anySucceeded = true
			backendKeys.Wtmpdb = wtmpdbKey
		} else {
			errors = append(errors, err)
		}
	}
	if !anySucceeded {
		return nil, trace.NewAggregate(errors...)
	}
	key, err := json.Marshal(backendKeys)
	return key, trace.Wrap(err)
}

func (uacc *UserAccounting) Logout(key []byte, ts time.Time) error {
	var backendKeys userKey
	if err := json.Unmarshal(key, &backendKeys); err != nil {
		return trace.Wrap(err)
	}
	var anySucceeded bool
	errors := make([]error, 0, 2)
	if backendKeys.Utmp != "" {
		if uacc.utmp == nil {
			return trace.BadParameter("utmp not supported")
		}
		if err := uacc.utmp.Logout(backendKeys.Utmp, ts); err == nil {
			anySucceeded = true
		} else {
			errors = append(errors, err)
		}
	}
	if backendKeys.Wtmpdb != "" {
		if uacc.wtmpdb == nil {
			return trace.BadParameter("wtmpdb not supported")
		} else if uacc.isPAMEnabled {
			return trace.BadParameter("wtmpdb login/logout is handled by PAM")
		}
		if err := uacc.wtmpdb.Logout(backendKeys.Wtmpdb, ts); err == nil {
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

func (uacc *UserAccounting) FailedLogin(username string, remote net.Addr, ts time.Time) error {
	if uacc.utmp != nil {
		if err := uacc.utmp.FailedLogin(username, remote, ts); err != nil {
			return trace.Wrap(err)
		}
	}
	// wtmpdb doesn't log failed logins
	return nil
}

func (uacc *UserAccounting) IsUserLoggedIn(username string) (bool, error) {
	errors := make([]error, 0, 2)
	if uacc.utmp != nil {
		loggedIn, err := uacc.utmp.IsUserLoggedIn(username)
		if err != nil {
			errors = append(errors, err)
		} else if loggedIn {
			return true, nil
		}
	}
	if uacc.wtmpdb != nil {
		loggedIn, err := uacc.wtmpdb.IsUserLoggedIn(username)
		if err != nil {
			errors = append(errors, err)
		} else if loggedIn {
			return true, nil
		}
	}
	return false, trace.NewAggregate(errors...)
}

// PrepareAddr parses and transforms a net.Addr into a format usable by other uacc functions.
func PrepareAddr(addr net.Addr) ([4]int32, error) {
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

func GetTTYName(tty *os.File) (string, error) {
	ttyFullName, err := os.Readlink(tty.Name())
	if err != nil {
		return "", trace.Wrap(err)
	}
	return strings.TrimPrefix(ttyFullName, "/dev/"), nil
}
