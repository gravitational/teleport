package uacc

import (
	"database/sql"
	"os"
	"time"

	"github.com/gravitational/trace"
)

const dbLocation = "/var/lib/wtmpdb/wtmp.db"

type wtmpdb struct {
	db *sql.DB
}

func newWtmpdb(dbFile string) (*wtmpdb, error) {
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &wtmpdb{db: db}, nil
}

func (w *wtmpdb) Login(tty *os.File, username, hostname string, ts time.Time) error {
	ttyName, err := getTTYName(tty)
	if err != nil {
		return trace.Wrap(err)
	}
	stmt, err := w.db.Prepare("INSET INTO wtmp(Type, User, Login, TTY, RemoteHost) VALUES(?,?,?,?,?)")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = stmt.Exec("USER_PROCESS", username, ts.UnixMicro(), ttyName, hostname)
	return trace.Wrap(err)
}

func (w *wtmpdb) Logout(tty *os.File, ts time.Time) error {
	ttyName, err := getTTYName(tty)
	if err != nil {
		return trace.Wrap(err)
	}
	stmt, err := w.db.Prepare("UPDATE wtmp SET Logout = ? WHERE TTY = ? AND LOGOUT = 0")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = stmt.Exec(ts.UnixMicro(), ttyName)
	return trace.Wrap(err)
}
