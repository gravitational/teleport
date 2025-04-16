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
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"golang.org/x/term"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/asciitable"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
	"github.com/gravitational/teleport/lib/defaults"
)

type REPL struct {
	connConfig *pgconn.Config
	client     io.ReadWriteCloser
	serverConn net.Conn
	route      clientproto.RouteToDatabase
	term       *term.Terminal
	commands   map[string]*command
}

func New(_ context.Context, cfg *dbrepl.NewREPLConfig) (dbrepl.REPLInstance, error) {
	config, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%s", hostnamePlaceholder))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.User = cfg.Route.Username
	config.Database = cfg.Route.Database
	config.ConnectTimeout = defaults.DatabaseConnectTimeout
	config.RuntimeParams = map[string]string{
		applicationNameParamName: applicationNameParamValue,
	}
	config.TLSConfig = nil

	// Provide a lookup function to avoid having the hostname placeholder to
	// resolve into something else. Note that the returned value won't be used.
	config.LookupFunc = func(_ context.Context, _ string) ([]string, error) {
		return []string{hostnamePlaceholder}, nil
	}
	config.DialFunc = func(_ context.Context, _, _ string) (net.Conn, error) {
		return cfg.ServerConn, nil
	}

	return &REPL{
		connConfig: config,
		client:     cfg.Client,
		serverConn: cfg.ServerConn,
		route:      cfg.Route,
		term:       term.NewTerminal(cfg.Client, ""),
		commands:   initCommands(),
	}, nil
}

// Run starts and run the PostgreSQL REPL session. The provided context is used
// to interrupt the execution and clean up resources.
func (r *REPL) Run(ctx context.Context) error {
	pgConn, err := pgconn.ConnectConfig(ctx, r.connConfig)
	if err != nil {
		return trace.ConnectionProblem(err, "Unable to connect to database: %v", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pgConn.Close(closeCtx)
	}()

	// term.Terminal blocks reads/writes without respecting the context. The
	// only thing that unblocks it is closing the underlaying connection (in
	// our case r.client). On this goroutine we only watch for context
	// cancelation and close the connection. This will unblocks all terminal
	// reads/writes.
	ctxCancelCh := make(chan struct{})
	defer close(ctxCancelCh)
	go func() {
		select {
		case <-ctx.Done():
			_ = r.client.Close()
		case <-ctxCancelCh:
		}
	}()

	if err := r.presentBanner(); err != nil {
		return trace.Wrap(err)
	}

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
			return trace.Wrap(formatTermError(ctx, err))
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

			reply = formatResult(pgConn.Exec(ctx, query).ReadAll()) + lineBreak
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
			return trace.Wrap(formatTermError(ctx, err))
		}
	}
}

// formatTermError changes the term.Terminal error to match caller expectations.
func formatTermError(ctx context.Context, err error) error {
	// When context is canceled it will immediately lead read/write errors due
	// to the closed connection. For this cases we return the context error.
	if ctx.Err() != nil && (errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed)) {
		return ctx.Err()
	}

	return err
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

	var (
		sb         strings.Builder
		resultsLen = len(results)
	)
	for i, res := range results {
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

		// Add line breaks to separate results. Except the last result, which
		// will have line breaks added later in the reply.
		if i != resultsLen-1 {
			sb.WriteString(lineBreak + lineBreak)
		}
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

const (
	// applicationNameParamName defines the application name parameter name.
	//
	// https://www.postgresql.org/docs/17/libpq-connect.html#LIBPQ-CONNECT-APPLICATION-NAME
	applicationNameParamName = "application_name"
	// applicationNameParamValue defines the application name parameter value.
	applicationNameParamValue = "teleport-repl"
)

// descriptiveLimitations defines a user-friendly text containing the REPL
// limitations.
var descriptiveLimitations = []string{
	`Query cancellation is not supported. Once a query is sent, its execution
cannot be canceled. Note that Teleport sends a terminate message to the database
when the database session terminates. This flow doesn't guarantee that any
running queries will be canceled.
See https://www.postgresql.org/docs/17/protocol-flow.html#PROTOCOL-FLOW-TERMINATION for more details on the termination flow.`,
	// This limitation is due to our terminal emulator not fully supporting this
	// shortcut's custom handler. Instead, it will close the terminal, leading
	// to terminating the session. To avoid having users accidentally
	// terminating their sessions, we're turning this off until we have a better
	// solution and propose the behavior for it.
	//
	// This shortcut filtered out by the WebUI key handler.
	"Pressing CTRL-C will have no effect in this shell.",
}
