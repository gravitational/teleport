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
	"database/sql"
	"errors"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/mattn/go-sqlite3"

	"github.com/gravitational/teleport/lib/utils"
)

// defaultWtmpdbPath is the default location of the wtmpdb db file.
const defaultWtmpdbPath = "/var/lib/wtmpdb/wtmp.db"

// userProcess is the wtmpdb entry type for user sessions.
const userProcess = 3

// WtmpdbBackend handles user accounting on systems with wtmpdb.
type WtmpdbBackend struct {
	db *sql.DB
}

// NewWtmpdbBackend creates a new wtmpdb backend.
func NewWtmpdbBackend(dbPath string) (*WtmpdbBackend, error) {
	if dbPath == "" {
		dbPath = defaultWtmpdbPath
	}
	if !utils.FileExists(dbPath) {
		return nil, trace.NotFound("no wtmpdb at %q", dbPath)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &WtmpdbBackend{db: db}, nil
}

// Login creates a new session entry for the given user.
func (w *WtmpdbBackend) Login(ttyName, username string, remote net.Addr, ts time.Time) (int64, error) {
	// Schema: https://github.com/thkukuk/wtmpdb?tab=readme-ov-file#database
	stmt, err := w.db.Prepare("INSERT INTO wtmp(Type, User, Login, TTY, RemoteHost) VALUES(?,?,?,?,?)")
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer stmt.Close()
	addr := utils.FromAddr(remote)
	result, err := stmt.Exec(userProcess, username, ts.UnixMicro(), ttyName, addr.Host())
	if err != nil {
		if errors.Is(err, sqlite3.ErrReadonly) {
			return 0, trace.AccessDenied("cannot write to wtmpdb file, is Teleport running as root?")
		}
		return 0, trace.Wrap(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return id, nil
}

// Logout marks the user corresponding to the given id (returned from Login) as logged out.
func (w *WtmpdbBackend) Logout(id int64, ts time.Time) error {
	stmt, err := w.db.Prepare("UPDATE wtmp SET Logout = ? WHERE ID = ?")
	if err != nil {
		return trace.Wrap(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(ts.UnixMicro(), id)
	return trace.Wrap(err)
}

// IsUserLoggedIn checks if the given user has an active session.
func (w *WtmpdbBackend) IsUserLoggedIn(username string) (bool, error) {
	stmt, err := w.db.Prepare("SELECT COUNT(1) FROM wtmp WHERE User = ? AND Logout IS NULL")
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer stmt.Close()
	var count int
	if err := stmt.QueryRow(username).Scan(&count); err != nil {
		return false, nil
	}
	return count != 0, nil
}
