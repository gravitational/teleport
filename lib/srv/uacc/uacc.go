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
	"context"
	"encoding/binary"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
)

type userAccountingBackend interface {
	Name() string
	Login(ttyName, username, hostname string, remote net.Addr, ts time.Time) (string, error)
	Logout(id string, ts time.Time) error
	FailedLogin(username, hostname string, remote net.Addr, ts time.Time) error
	IsUserLoggedIn(username string) (bool, error)
}

var backends []userAccountingBackend

func registerBackend(backend userAccountingBackend) {
	backends = append(backends, backend)
	slog.DebugContext(context.Background(), "registered user accounting backend", "backend", backend.Name())
}

func tryBackendOp(f func(bk userAccountingBackend) error) error {
	errors := make([]error, 0, len(backends))
	var success bool
	for _, bk := range backends {
		err := f(bk)
		if err == nil {
			success = true
		} else {
			errors = append(errors, trace.Wrap(err, "backend %q failed", bk.Name()))
		}
	}
	if success {
		return nil
	}
	return trace.NewAggregate(errors...)
}

type UserAccounting struct {
	keys map[string]map[string]string
}

func NewUserAccounting() (*UserAccounting, error) {
	if len(backends) == 0 {
		return nil, trace.NotImplemented("no user accounting backends available")
	}
	return &UserAccounting{
		keys: make(map[string]map[string]string),
	}, nil
}

func (uacc *UserAccounting) Login(ttyName, username, hostname string, remote net.Addr, ts time.Time) (string, error) {
	keysPerBackend := make(map[string]string, len(backends))
	err := tryBackendOp(func(bk userAccountingBackend) error {
		key, err := bk.Login(ttyName, username, hostname, remote, ts)
		if err != nil {
			return trace.Wrap(err)
		}
		keysPerBackend[bk.Name()] = key
		return nil
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	key := uuid.NewString()
	uacc.keys[key] = keysPerBackend
	return key, nil
}

func (uacc *UserAccounting) Logout(key string, ts time.Time) error {
	keysPerBackend, ok := uacc.keys[key]
	if !ok {
		return trace.NotFound("no local user for key %q", key)
	}
	err := tryBackendOp(func(bk userAccountingBackend) error {
		key, ok := keysPerBackend[bk.Name()]
		if !ok {
			return trace.NotFound("no local user for backend %q", bk.Name())
		}
		return trace.Wrap(bk.Logout(key, ts))
	})
	if err != nil {
		return trace.Wrap(err)
	}
	delete(uacc.keys, key)
	return nil
}

func FailedLogin(username, hostname string, remote net.Addr, ts time.Time) error {
	return trace.Wrap(tryBackendOp(func(bk userAccountingBackend) error {
		return bk.FailedLogin(username, hostname, remote, ts)
	}))
}

func IsUserLoggedIn(username string) (bool, error) {
	var isUserLoggedIn bool
	// TODO: too clever?
	err := tryBackendOp(func(bk userAccountingBackend) error {
		loggedIn, err := bk.IsUserLoggedIn(username)
		isUserLoggedIn = isUserLoggedIn || loggedIn
		return err
	})
	return isUserLoggedIn, trace.Wrap(err)
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

func getTTYName(tty *os.File) (string, error) {
	ttyFullName, err := os.Readlink(tty.Name())
	if err != nil {
		return "", trace.Wrap(err)
	}
	return strings.TrimPrefix(ttyFullName, "/dev/"), nil
}
