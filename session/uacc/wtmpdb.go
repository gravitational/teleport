// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

import (
	"net"
	"time"
)

// userProcess is the wtmpdb entry type for user sessions.
const userProcess = 3

// WtmpdbBackend handles user accounting on systems with wtmpdb.
type WtmpdbBackend struct {
	wtmpdbBackend
}

func WtmpdbBackendAvailable() error {
	return wtmpdbBackendAvailable()
}

// NewWtmpdbBackend creates a new wtmpdb backend.
func NewWtmpdbBackend(dbPath string) (*WtmpdbBackend, error) {
	return newWtmpdbBackend(dbPath)
}

// Login creates a new session entry for the given user.
func (w *WtmpdbBackend) Login(ttyName, username string, remote net.Addr, ts time.Time) (int64, error) {
	return w.login(ttyName, username, remote, ts)
}

// Logout marks the user corresponding to the given id (returned from Login) as logged out.
func (w *WtmpdbBackend) Logout(id int64, ts time.Time) error {
	return w.logout(id, ts)
}
