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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"golang.org/x/term"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
	"github.com/gravitational/teleport/lib/defaults"
)

type REPL struct {
	*term.Terminal

	connConfig *pgconn.Config
	client     io.ReadWriteCloser
	route      clientproto.RouteToDatabase
	commands   map[string]*command

	// teleportVersion is used in golden tests to fake the current Teleport
	// version and prevent test failures when the real version is incremented.
	teleportVersion string
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
	// disable fallbacks because our fake dialer returns the same connection
	// each time and pgconn closes a conn on error before using a fallback,
	// which obscures the actual error and instead shows:
	// "failed to write startup message (use of closed network connection)"
	config.Fallbacks = nil

	// Provide a lookup function to avoid having the hostname placeholder to
	// resolve into something else. Note that the returned value won't be used.
	config.LookupFunc = func(_ context.Context, _ string) ([]string, error) {
		return []string{hostnamePlaceholder}, nil
	}
	config.DialFunc = func(_ context.Context, _, _ string) (net.Conn, error) {
		return cfg.ServerConn, nil
	}

	return &REPL{
		Terminal:        term.NewTerminal(cfg.Client, ""),
		connConfig:      config,
		client:          cfg.Client,
		route:           cfg.Route,
		commands:        initCommands(),
		teleportVersion: teleport.Version,
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
	defer context.AfterFunc(ctx, func() {
		_ = r.client.Close()
	})()

	if err := r.presentBanner(); err != nil {
		return trace.Wrap(err)
	}

	var (
		multilineAcc     strings.Builder
		readingMultiline bool
	)

	lead := lineLeading(r.route)
	leadSpacing := strings.Repeat(" ", len(lead))

	for {
		if readingMultiline {
			r.Terminal.SetPrompt(leadSpacing)
		} else {
			r.Terminal.SetPrompt(lead)
		}
		line, err := r.Terminal.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return trace.Wrap(formatTermError(ctx, err))
		}

		// ReadLine should always return the line without trailing line breaks,
		// but we still require to remove trailing and leading spaces.
		line = strings.TrimSpace(line)
		if !readingMultiline && (len(line) == 0 || strings.HasPrefix(line, commentPrefix)) {
			continue
		}

		switch {
		case strings.HasPrefix(line, commandPrefix) && !readingMultiline:
			reply, exit := r.processCommand(line)
			if exit {
				return nil
			}
			if _, err := r.Terminal.Write([]byte(reply + lineBreak)); err != nil {
				return trace.Wrap(formatTermError(ctx, err))
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

			resReader := pgConn.Exec(ctx, query)
			if err := streamResults(resReader, newTabWriter(r.Terminal)); err != nil {
				return trace.Wrap(formatTermError(ctx, err))
			}
			if _, err := r.Terminal.Write([]byte(lineBreak + lineBreak)); err != nil {
				return trace.Wrap(formatTermError(ctx, err))
			}
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
		r.Terminal,
		`Teleport PostgreSQL interactive shell (v%s)
Connected to %q instance as %q user.
Type \? for help.

`,
		r.teleportVersion,
		r.route.GetServiceName(),
		r.route.GetUsername())
	return trace.Wrap(err)
}

func newTabWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 1, 0, 1, ' ', 0)
}

// bufferedWriter buffers all Write calls until Flush is called.
// Clients must call Flush when done writing.
type bufferedWriter interface {
	io.Writer
	Flush() error
}

func streamResults(mrr *pgconn.MultiResultReader, writer bufferedWriter) error {
	defer mrr.Close()
	defer writer.Flush()

	var resultCount int
	for mrr.NextResult() {
		resultCount++
		if resultCount > 1 {
			// add an extra line break to separate multiple results
			if _, err := writer.Write([]byte(lineBreak + lineBreak)); err != nil {
				return trace.Wrap(err)
			}
		}
		if err := streamResult(mrr.ResultReader(), writer); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := mrr.Close(); err != nil {
		errReply := errorReplyPrefix + err.Error()
		if _, err := writer.Write([]byte(errReply)); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func streamResult(rr *pgconn.ResultReader, writer bufferedWriter) error {
	const (
		columnSep       = "\t|\t"
		maxBufferedRows = 100
	)
	defer rr.Close()
	defer writer.Flush()

	if len(rr.FieldDescriptions()) == 0 {
		// ignore the error, it will bubble up to the multi result reader Close
		cmdTag, _ := rr.Close()
		if _, err := writer.Write([]byte(cmdTag.String())); err != nil {
			return trace.Wrap(err)
		}
		// zero field descriptions implies zero rows will be returned
		// https://www.postgresql.org/docs/current/protocol-flow.html
		return nil
	}

	columns := make([]string, 0, len(rr.FieldDescriptions()))
	dashes := make([]string, 0, cap(columns))
	for _, fd := range rr.FieldDescriptions() {
		columns = append(columns, string(fd.Name))
		dashes = append(dashes, strings.Repeat("-", max(len(fd.Name))))
	}

	headerLine := strings.Join(columns, columnSep) + lineBreak
	sepLine := strings.Join(dashes, columnSep) + lineBreak
	if _, err := writer.Write([]byte(headerLine + sepLine)); err != nil {
		return trace.Wrap(err)
	}

	var rowCount int
	for rr.NextRow() {
		rowCount++
		row := bytes.Join(rr.Values(), []byte(columnSep))
		if _, err := writer.Write(row); err != nil {
			return trace.Wrap(err)
		}
		if rowCount%maxBufferedRows == 0 {
			if err := writer.Flush(); err != nil {
				return trace.Wrap(err)
			}
		}
		if _, err := writer.Write([]byte(lineBreak)); err != nil {
			return trace.Wrap(err)
		}
	}

	if _, err := writer.Write([]byte(rowsText(rowCount))); err != nil {
		return trace.Wrap(err)
	}
	return nil
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
	lineBreak = "\n"
	// commandPrefix is the prefix that identifies a REPL command.
	commandPrefix = "\\"
	// executionRequestSuffix is the suffix that indicates the input must be
	// executed.
	executionRequestSuffix = ";"
	// errorReplyPrefix is the prefix presented when there is a execution error.
	errorReplyPrefix = "ERR "
	// commentPrefix is the prefix for a single line comment
	commentPrefix = "--"
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
	"All escaped delimiters (semicolons) must appear on the same line as the unescaped query delimiter.",
	"Commands cannot be used in the middle of a query.",
}
