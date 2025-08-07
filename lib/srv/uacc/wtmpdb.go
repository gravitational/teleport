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
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"

	"github.com/gravitational/teleport/lib/utils"
)

const wtmpdbLocation = "/var/lib/wtmpdb/wtmp.db"
const USER_PROCESS = 3

type wtmpdbBackend struct {
	db *sql.DB
}

func newWtmpdb(dbPath string) (*wtmpdbBackend, error) {
	if dbPath == "" {
		dbPath = wtmpdbLocation
	}
	if !utils.FileExists(dbPath) {
		return nil, trace.NotFound("no wtmpdb at %q", dbPath)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &wtmpdbBackend{db: db}, nil
}

func (w *wtmpdbBackend) Login(ttyName, username string, remote net.Addr, ts time.Time) (string, error) {
	stmt, err := w.db.Prepare("INSERT INTO wtmp(Type, User, Login, TTY, RemoteHost) VALUES(?,?,?,?,?)")
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer stmt.Close()
	addr := utils.FromAddr(remote)
	result, err := stmt.Exec(USER_PROCESS, username, ts.UnixMicro(), ttyName, addr.Host())
	if err != nil {
		if errors.Is(err, sqlite3.ErrReadonly) {
			return "", trace.AccessDenied("cannot write to wtmpdb file, is teleport running as root?")
		}
		return "", trace.Wrap(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return strconv.Itoa(int(id)), nil
}

func (w *wtmpdbBackend) Logout(id string, ts time.Time) error {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return trace.Wrap(err)
	}
	stmt, err := w.db.Prepare("UPDATE wtmp SET Logout = ? WHERE ID = ?")
	if err != nil {
		return trace.Wrap(err, int64(idInt))
	}
	defer stmt.Close()
	_, err = stmt.Exec(ts.UnixMicro(), idInt)
	return trace.Wrap(err)
}

func (w *wtmpdbBackend) FailedLogin(username string, remote net.Addr, ts time.Time) error {
	return trace.NotImplemented("wtmpdb backend does not support logging failed logins")
}

func (w *wtmpdbBackend) IsUserLoggedIn(username string) (bool, error) {
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
