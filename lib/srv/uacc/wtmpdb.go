package uacc

import (
	"database/sql"
	"errors"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/gravitational/trace"
)

const wtmpdbLocation = "/var/lib/wtmpdb/wtmp.db"

type wtmpdbBackend struct {
	db *sql.DB
}

func newWtmpdb(dbPath string) (*wtmpdbBackend, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &wtmpdbBackend{db: db}, nil
}

func init() {
	wtmpdb, err := newWtmpdb(wtmpdbLocation)
	if err == nil {
		registerBackend(wtmpdb)
	}
}

func (w *wtmpdbBackend) Name() string {
	return "wtmpdb"
}

func (w *wtmpdbBackend) Login(tty *os.File, username string, remote net.Addr, ts time.Time) (string, error) {
	ttyName, err := GetTTYName(tty)
	if err != nil {
		return "", trace.Wrap(err)
	}
	remoteHost, _, err := net.SplitHostPort(remote.String())
	if err != nil {
		return "", trace.Wrap(err)
	}
	stmt, err := w.db.Prepare("INSET INTO wtmp(Type, User, Login, TTY, RemoteHost) VALUES(?,?,?,?,?)")
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer stmt.Close()
	result, err := stmt.Exec("USER_PROCESS", username, ts.UnixMicro(), ttyName, remoteHost)
	if err != nil {
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
	stmt, err := w.db.Prepare("SELECT TTY FROM wtmp WHERE User = ? AND Logout = 0")
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer stmt.Close()
	var tty string
	if err := stmt.QueryRow(username).Scan(&tty); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	return tty != "", nil
}
