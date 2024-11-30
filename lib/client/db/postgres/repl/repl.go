// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package repl

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"golang.org/x/term"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/asciitable"
)

type REPL struct {
	ctx        context.Context
	cancelFunc context.CancelCauseFunc
	conn       *pgconn.PgConn
	client     io.ReadWriter
	serverConn net.Conn
	route      clientproto.RouteToDatabase
	term       *term.Terminal
	commands   map[string]*command
}

func Start(ctx context.Context, client io.ReadWriter, serverConn net.Conn, route clientproto.RouteToDatabase) (*REPL, error) {
	config, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%s@%s/%s", route.Username, hostnamePlaceholder, route.Database))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.TLSConfig = nil

	// Provide a lookup function to avoid having the hostname placeholder to
	// resolve into something else. Note that the returned value won't be used.
	config.LookupFunc = func(_ context.Context, _ string) ([]string, error) {
		return []string{hostnamePlaceholder}, nil
	}
	config.DialFunc = func(_ context.Context, _, _ string) (net.Conn, error) {
		return serverConn, nil
	}

	pgConn, err := pgconn.ConnectConfig(ctx, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	replCtx, cancelFunc := context.WithCancelCause(ctx)
	r := &REPL{
		ctx:        replCtx,
		cancelFunc: cancelFunc,
		conn:       pgConn,
		client:     client,
		serverConn: serverConn,
		route:      route,
		term:       term.NewTerminal(client, ""),
		commands:   initCommands(),
	}

	go r.start()
	return r, nil
}

func (r *REPL) Close() {
	r.close(nil)
}

func (r *REPL) close(err error) {
	r.cancelFunc(err)
	r.conn.Close(r.ctx)
	r.serverConn.Close()
}

func (r *REPL) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}

func (r *REPL) start() {
	if err := r.presentBanner(); err != nil {
		r.close(err)
		return
	}

	// After loop is done, we always close the REPL to ensure the connections
	// are cleaned.
	r.close(r.loop())
}

// loop implements the main REPL loop.
func (r *REPL) loop() error {
	var (
		multilineAcc     strings.Builder
		readingMultiline bool
	)

	lead := lineLeading(r.route)
	leadSpacing := strings.Repeat(" ", len(lead))
	r.term.SetPrompt(lineBreak + lead)

	for {
		line, err := r.term.ReadLine()
		if err != nil {
			return trace.Wrap(err)
		}
		// ReadLine should always return the line without trailing line breaks,
		// but we still require to remove trailing and leading spaces.
		line = strings.TrimSpace(line)

		var reply string
		switch {
		case strings.HasPrefix(line, commandPrefix) && !readingMultiline:
			var exit bool
			reply, exit = r.processCommand(line)
			if exit {
				return nil
			}
		case strings.HasSuffix(line, executionRequestSuffix):
			var query string
			if readingMultiline {
				multilineAcc.WriteString(lineBreak + line)
				query = multilineAcc.String()
			} else {
				query = line
			}

			// Reset multiline state.
			multilineAcc.Reset()
			readingMultiline = false
			r.term.SetPrompt(lineBreak + lead)

			reply = formatResult(r.conn.Exec(r.ctx, query).ReadAll()) + lineBreak
		default:
			// If there wasn't a specific execution, we assume the input is
			// multi-line. In this case, we need to accumulate the contents.

			// If this isn't the first line, add the line break as the
			// ReadLine function removes it.
			if readingMultiline {
				multilineAcc.WriteString(lineBreak)
			}

			readingMultiline = true
			multilineAcc.WriteString(line)
			r.term.SetPrompt(leadSpacing)
		}

		if reply == "" {
			continue
		}

		if _, err := r.term.Write([]byte(reply)); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (r *REPL) presentBanner() error {
	_, err := fmt.Fprintf(
		r.term,
		`Teleport PostgreSQL interactive shell (v%s)
Connected to %q instance as %q user.
Type \? for help.`,
		teleport.Version,
		r.route.GetServiceName(),
		r.route.GetUsername())
	return trace.Wrap(err)
}

// formatResult formats a pgconn.Exec result.
func formatResult(results []*pgconn.Result, err error) string {
	if err != nil {
		return errorReplyPrefix + err.Error()
	}

	var sb strings.Builder
	for _, res := range results {
		if !res.CommandTag.Select() {
			return res.CommandTag.String()
		}

		// build columns
		var columns []string
		for _, fd := range res.FieldDescriptions {
			columns = append(columns, string(fd.Name))
		}

		table := asciitable.MakeTable(columns)
		for _, row := range res.Rows {
			rowData := make([]string, len(columns))
			for i, data := range row {
				// The PostgreSQL package is responsible for transforming the
				// row data into a readable format.
				rowData[i] = string(data)
			}

			table.AddRow(rowData)
		}

		table.AsBuffer().WriteTo(&sb)
		sb.WriteString(rowsText(len(res.Rows)))
	}

	return sb.String()
}

func lineLeading(route clientproto.RouteToDatabase) string {
	return fmt.Sprintf("%s=> ", route.Database)
}

func rowsText(count int) string {
	rowTxt := "row"
	if count > 1 {
		rowTxt = "rows"
	}

	return fmt.Sprintf("(%d %s)", count, rowTxt)
}

const (
	// hostnamePlaceholder is the hostname used when connecting to the database.
	// The pgconn functions require a hostname, however, since we already have
	// the connection, we just need to provide a name to suppress this
	// requirement.
	hostnamePlaceholder = "repl"
	// lineBreak represents a line break on the REPL.
	lineBreak = "\r\n"
	// commandPrefix is the prefix that identifies a REPL command.
	commandPrefix = "\\"
	// executionRequestSuffix is the suffix that indicates the input must be
	// executed.
	executionRequestSuffix = ";"
	// errorReplyPrefix is the prefix presented when there is a execution error.
	errorReplyPrefix = "ERR "
)
