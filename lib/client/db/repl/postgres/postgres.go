package postgres

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/term"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/asciitable"
)

const banner = `Teleport PostgreSQL interactive shell (v%s)
Connected to %q instance as %q user.
Type "help" or \? for help.
`

type REPL interface {
	Close() error
}

type repl struct {
	wg         sync.WaitGroup
	ctx        context.Context
	conn       *pgconn.PgConn
	clientConn io.ReadWriteCloser
	serverConn net.Conn
	route      clientproto.RouteToDatabase
	term       *term.Terminal
}

func New(ctx context.Context, clientConn io.ReadWriteCloser, serverConn net.Conn, route clientproto.RouteToDatabase) (*repl, error) {
	config, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%s@placeholder/%s", route.Username, route.Database))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.TLSConfig = nil

	config.DialFunc = func(_ context.Context, _, _ string) (net.Conn, error) {
		return serverConn, nil
	}
	config.LookupFunc = func(_ context.Context, _ string) ([]string, error) {
		return []string{"placeholder"}, nil
	}

	pgConn, err := pgconn.ConnectConfig(ctx, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r := &repl{
		ctx:        ctx,
		conn:       pgConn,
		clientConn: clientConn,
		serverConn: serverConn,
		route:      route,
		term:       term.NewTerminal(clientConn, ""),
	}

	r.wg.Add(1)
	go r.start()
	return r, nil
}

const lineBreak = "\r\n"

func (r *repl) Close() {
	r.conn.Close(context.TODO())
	r.clientConn.Close()
	r.serverConn.Close()
}

// TODO: add error forwarding.
func (r *repl) Wait() error {
	r.wg.Wait()
	return nil
}

func (r *repl) start() {
	defer r.Close()

	var acc strings.Builder
	var isReadingMultiline bool

	lead := fmt.Sprintf("%s=> ", r.route.Database)
	leadSpacing := strings.Repeat(" ", len(lead))

	if _, err := fmt.Fprintf(r.term, banner, teleport.Version, r.route.ServiceName, r.route.Username); err != nil {
		return
	}
	r.term.SetPrompt(lineBreak + lead)

	for {
		line, err := r.term.ReadLine()
		if err != nil {
			return
		}

		var reply string
		// TODO: cover edge cases
		switch {
		case strings.HasPrefix(line, "\\"):
			if isReadingMultiline {
				acc.WriteString(" " + lineBreak + line)
				continue
			}

			args := strings.Split(line, " ")
			switch strings.TrimPrefix(args[0], "\\") {
			case "q":
				return
			case "d":
				reply = formatResult(r.conn.Exec(r.ctx, describeQuery).ReadAll()) + lineBreak
			case "?":
				reply = helpCommand
			default:
				reply = "Unknown command. Try \\? or \"help\" to show the list of supported commands" + lineBreak
			}
		case strings.HasSuffix(line, ";"):
			var query string
			if isReadingMultiline {
				acc.WriteString(" " + lineBreak + line)
				query = acc.String()
				acc.Reset()
			} else {
				query = line
			}
			isReadingMultiline = false
			r.term.SetPrompt(lineBreak + lead)
			reply = formatResult(r.conn.Exec(r.ctx, query).ReadAll()) + lineBreak
		default:
			// multiline commands.
			if isReadingMultiline {
				acc.WriteString(" " + lineBreak)
			}
			isReadingMultiline = true
			acc.WriteString(line)
			r.term.SetPrompt(leadSpacing)
		}

		if len(reply) == 0 {
			continue
		}

		if _, err := r.term.Write([]byte(reply)); err != nil {
			return
		}
	}
}

func formatResult(results []*pgconn.Result, err error) string {
	if err != nil {
		return formatError(err)
	}

	// TODO support multiple queries results.
	// TODO check if multiple queries should be supported.
	res := results[0]

	if !res.CommandTag.Select() {
		return res.CommandTag.String()
	}

	// build columns
	var columns []string
	for _, fd := range res.FieldDescriptions {
		columns = append(columns, fd.Name)
	}

	table := asciitable.MakeTable(columns)
	for _, row := range res.Rows {
		rowData := make([]string, len(columns))
		for i, data := range row {
			rowData[i] = string(data)
		}

		table.AddRow(rowData)
	}

	var sb strings.Builder
	table.AsBuffer().WriteTo(&sb)
	fmt.Fprintf(&sb, "(%d rows affected)", len(res.Rows))
	return sb.String()
}

func formatError(err error) string {
	return "ERR " + err.Error()
}

const helpCommand = `General:
  \q          Terminate the session.
  \teleport   Show Teleport interactive shell information, such as execution limitations.

Informational:
  \d              List tables, views, and sequences.
  \d NAME         Describe table, view, sequence, or index.
  \dt [PATTERN]   List tables.

Connection/Session:
  \session   Display information about the current session, like user, roles, and database instance.
`

const describeQuery = `SELECT n.nspname as "Schema",
c.relname as "Name",
CASE c.relkind WHEN 'r' THEN 'table' WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized view' WHEN 'i' THEN 'index' WHEN 'S' THEN 'sequence' WHEN 's' THEN 'special' WHEN 't' THEN 'TOAST table' WHEN 'f' THEN 'foreign table' WHEN 'p' THEN 'partitioned table' WHEN 'I' THEN 'partitioned index' END as "Type",
pg_catalog.pg_get_userbyid(c.relowner) as "Owner"
FROM pg_catalog.pg_class c
LEFT JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_catalog.pg_am am ON am.oid = c.relam
WHERE c.relkind IN ('r','p','v','m','S','f','')
AND n.nspname <> 'pg_catalog'
AND n.nspname !~ '^pg_toast'
AND n.nspname <> 'information_schema'
AND pg_catalog.pg_table_is_visible(c.oid) ORDER BY 1,2;`
