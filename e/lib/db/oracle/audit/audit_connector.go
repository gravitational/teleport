package audit

import (
	"context"
	"crypto/tls"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"
	go_ora "github.com/sijms/go-ora/v2"
	"github.com/sijms/go-ora/v2/network"
)

const (
	// queryAuditLog allows to query DBA_AUDIT_TRAIL audit view and get use session audit logs entries.
	// To limit height memory consumption audit log entries will be fetched in batches using OFFSET/FETCH NEXT ROWS
	// Oracle mechanism.
	// https://docs.oracle.com/en/database/oracle/oracle-database/19/refrn/DBA_AUDIT_TRAIL.html
	queryAuditLog = `
SELECT entryid, sql_text, sql_bind
FROM SYS.DBA_AUDIT_TRAIL
WHERE sql_text IS NOT NULL
  AND sessionid = :1
  AND entryid > :2
ORDER BY entryid OFFSET :3 ROWS FETCH NEXT :4 ROWS ONLY`

	// queryAudSID allows to obtain audSID the unique audit session identifier that is used
	// to fetch per session audits logs. That mapping is done based on SID (SessionID) obtained from
	// the client-server handshake.
	queryAudSID = `SELECT AUDSID FROM SYS.V_$SESSION WHERE sid = :1`
)

type oracleConnector interface {
	init(serviceName, sessionID, addr string, conf *tls.Config, kerberosFun KerberosAuthFunc) (string, error)
	getAudSid(sid string) (string, error)
	fetchAuditLogs(audSID string, entryID string) ([]QueryEntry, error)
	close() error
}

// QueryEntry is the query entry result fetched from Oracle audit table.
type QueryEntry struct {
	// Text is the text of the SQL query.
	Text string
	// Bind is the bind query argument.
	Bind string
	// EntryID is the unique audit entry ID per audit session.
	EntryID string
}

type kerberosAuth struct {
	kerberosFun KerberosAuthFunc
}

func (k kerberosAuth) Authenticate(server, service string) ([]byte, error) {
	return k.kerberosFun(server, service)
}

func getOracleConnector(addr, serviceName string, tlsConfig *tls.Config, kerberosFun KerberosAuthFunc) (driver.Connector, error) {
	authType := "TCPS"
	if kerberosFun != nil {
		authType = "KERBEROS"
	}

	dsn := fmt.Sprintf(`oracle://%s/%s?SSL=enabled&AUTH TYPE=%s`, addr, serviceName, authType)
	conn := go_ora.NewConnector(dsn)

	oc, ok := conn.(*go_ora.OracleConnector)
	if !ok {
		return nil, trace.BadParameter("expected *go_ora.OracleConnector, got %T", conn)
	}
	oc.WithTLSConfig(tlsConfig)

	if kerberosFun != nil {
		oc.WithKerberosAuth(kerberosAuth{kerberosFun: kerberosFun})
	}

	return conn, nil
}

type oracleDB struct {
	db *sql.DB
	// username as reported by database; for reporting in error messages
	username string
}

var _ oracleConnector = (*oracleDB)(nil)

func (o *oracleDB) init(serviceName, sessionID, addr string, conf *tls.Config, kerberosFun KerberosAuthFunc) (string, error) {
	if serviceName == "" {
		return "", trace.BadParameter("empty serviceName")
	}
	if sessionID == "" {
		return "", trace.BadParameter("empty sessionID")
	}

	conn, err := getOracleConnector(addr, serviceName, conf, kerberosFun)
	if err != nil {
		return "", trace.Wrap(err)
	}

	dbConn := sql.OpenDB(conn)
	err = dbConn.Ping()
	if err != nil {
		return "", trace.Wrap(err)
	}
	o.db = dbConn

	// report username as seen by the database.
	row := dbConn.QueryRow("SELECT USER FROM DUAL")
	var user string
	err = row.Scan(&user)
	o.username = user
	if err != nil {
		slog.DebugContext(context.Background(), "failed to query database user", "err", err)
		o.username = "<UNKNOWN>"
	}

	audSID, err := o.getAudSid(sessionID)
	if err != nil {
		_ = dbConn.Close()
		return "", trace.Wrap(err, "failed to get aud-sid")
	}

	return audSID, nil
}

func (o *oracleDB) close() error {
	if o == nil {
		return nil
	}
	if o.db != nil {
		return o.db.Close()
	}
	return nil
}

// Oracle returns somewhat confusing "ORA-00942: table or view does not exist" when user is missing SELECT permissions.
const oracleErrorCodeNoSuchTable = 942

// getAudSid uses the Oracle system table and maps the Oracle sessionID obtained from the handshake to the
// unique audit ID identified allowing to distinguish a user session and fetch audit entries for a particular
// user session.
func (o *oracleDB) getAudSid(sid string) (string, error) {
	r, err := o.db.Query(queryAudSID, sid)
	if err != nil {
		var oracleErr *network.OracleError
		if errors.As(err, &oracleErr) {
			if oracleErr.ErrCode == oracleErrorCodeNoSuchTable {
				return "", trace.Wrap(err, "audit user %s is missing SELECT permissions to the SYS.V_$SESSION view", o.username)
			}
		}
		return "", trace.Wrap(err)
	}
	defer r.Close()

	var audSID string
	for r.Next() {
		err := r.Scan(&audSID)
		if err != nil {
			return "", trace.Wrap(err)
		}
		if audSID != "" {
			return audSID, nil
		}
	}
	return "", trace.Wrap(r.Err(), "failed to get auditSID")
}

// fetchAuditLogs fetches the query from the Oracle 'dba_audit_trail' table.
func (o *oracleDB) fetchAuditLogs(audSID string, lastEntryID string) ([]QueryEntry, error) {
	if lastEntryID == "" {
		// entryID starts from 1 in dba_audit_trail;
		lastEntryID = "1"
	}

	// rowLimit limits number of log entries rows returned from Oracle server.
	const rowLimit = 1000

	var (
		entryID string
		sqlText sql.NullString
		sqlBind sql.NullString
	)

	var out []QueryEntry
	for offset := 0; ; offset += rowLimit {
		r, err := o.db.Query(queryAuditLog, audSID, lastEntryID, offset, rowLimit)
		if err != nil {
			var oracleErr *network.OracleError
			if errors.As(err, &oracleErr) {
				if oracleErr.ErrCode == oracleErrorCodeNoSuchTable {
					return nil, trace.Wrap(err, "audit user %s is missing SELECT permissions to the SYS.DBA_AUDIT_TRAIL view", o.username)
				}
			}
			return nil, trace.Wrap(err)
		}
		rowsCount := 0
		for r.Next() {
			err = r.Scan(&entryID, &sqlText, &sqlBind)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			rowsCount += 1
			out = append(out, QueryEntry{
				Text:    sqlText.String,
				Bind:    sqlBind.String,
				EntryID: entryID,
			})
		}
		if r.Err() != nil {
			return nil, trace.Wrap(r.Err())
		}
		if rowsCount == 0 {
			return out, nil
		}
	}
}
