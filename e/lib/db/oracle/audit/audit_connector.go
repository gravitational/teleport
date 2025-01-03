package audit

import (
	"crypto/tls"
	"database/sql"
	"fmt"

	"github.com/gravitational/trace"
	go_ora "github.com/sijms/go-ora/v2"
)

const (
	// queryAuditLogFmt allows to query dba_audit_trail audit table and get use session audit logs entries.
	// To limit height memory consumption audit log entries will be fetched in batches using OFFSET/FETCH NEXT ROWS
	// Oracle mechanism.
	// https://docs.oracle.com/en/database/oracle/oracle-database/19/refrn/DBA_AUDIT_TRAIL.html
	queryAuditLogFmt = `
SELECT entryid, sql_text, sql_bind
FROM dba_audit_trail
WHERE sql_text IS NOT NULL
  AND sessionid = '%s'
  AND entryid > %s
ORDER BY entryid OFFSET %d ROWS FETCH NEXT %d ROWS ONLY`

	// queryAudSIDFmt allows to obtain audSID the unique audit session identifier that is used
	// to fetch per session audits logs. That mapping is done based on SID (SessionID) obtained from
	// the client-server handshake.
	queryAudSIDFmt = `
SELECT audsid
FROM v$session
WHERE sid = '%s'`
)

type oracleConnector interface {
	init(serviceName, sessionID, addr string, conf *tls.Config) (string, error)
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

func databaseConn(addr, serviceName string, tlsConfig *tls.Config) (*sql.DB, error) {
	var driver go_ora.OracleDriver
	dsn := fmt.Sprintf(`oracle://%s/%s?SSL=enabled&AUTH TYPE=TCPS`, addr, serviceName)
	conn, err := driver.OpenConnector(dsn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	oc, ok := conn.(*go_ora.OracleConnector)
	if !ok {
		return nil, trace.BadParameter("expected *go_ora.OracleConnector, got %T", conn)
	}
	oc.WithTLSConfig(tlsConfig)

	dbConn := sql.OpenDB(conn)

	if err := dbConn.Ping(); err != nil {
		return nil, trace.Wrap(err)
	}

	return dbConn, nil
}

type oracleDB struct {
	db     *sql.DB
	audSID string
}

func (o *oracleDB) init(serviceName, sessionID, addr string, conf *tls.Config) (string, error) {
	if serviceName == "" {
		return "", trace.BadParameter("empty serviceName")
	}
	if sessionID == "" {
		return "", trace.BadParameter("empty sessionID")
	}
	db, err := databaseConn(addr, serviceName, conf)
	if err != nil {
		return "", trace.Wrap(err)
	}
	o.db = db
	audSID, err := o.getAudSid(sessionID)
	if err != nil {
		return "", trace.NewAggregate(err, db.Close())
	}
	o.audSID = audSID
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

// getAudSid uses the Oracle system table and maps the Oracle sessionID obtained from the handshake to the
// unique audit ID identified allowing to distinguish a user session and fetch audit entries for a particular
// user session.
func (o *oracleDB) getAudSid(sid string) (string, error) {
	r, err := o.db.Query(fmt.Sprintf(queryAudSIDFmt, sid))
	if err != nil {
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
	return "", trace.NewAggregate(trace.BadParameter("failed to get auditSID"), r.Err())

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
		query := fmt.Sprintf(queryAuditLogFmt, audSID, lastEntryID, offset, rowLimit)
		r, err := o.db.Query(query)
		if err != nil {
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
