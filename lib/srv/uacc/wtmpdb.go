package uacc

import (
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const wtmpdbLocation = "/var/lib/wtmpdb/wtmp.db"
const USER_PROCESS = 3

type wtmpdbBackend struct {
	db *sql.DB
}

func newWtmpdb(dbPath string) (*wtmpdbBackend, error) {
	if !utils.FileExists(dbPath) {
		return nil, trace.NotFound("no wtmpdb at %q", dbPath)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &wtmpdbBackend{db: db}, nil
}

func (w *wtmpdbBackend) Name() string {
	return "wtmpdb"
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
		fmt.Println(err)
		return false, nil
	}
	fmt.Println(count)
	return count != 0, nil
}
