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

package repl

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"net"
	"strings"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
	"golang.org/x/term"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
)

// REPL implements [dbrepl.REPLInstance] for MySQL.
type REPL struct {
	*term.Terminal

	client     io.ReadWriteCloser
	serverConn net.Conn
	route      clientproto.RouteToDatabase
	parser     *parser
	myConn     mysqlConn

	// teleportVersion is used in golden tests to fake the current Teleport
	// version and prevent test failures when the real version is incremented.
	teleportVersion string

	// testPassword is normally blank, only used in tests where the REPL connects
	// directly to a MySQL instance without Teleport proxying.
	testPassword string
	// disableQueryTimings is used in golden tests to disable query timings for
	// test consistency.
	disableQueryTimings bool
}

type mysqlConn interface {
	Execute(command string, args ...any) (*mysql.Result, error)
	ExecuteSelectStreaming(command string, result *mysql.Result, perRowCallback client.SelectPerRowCallback, perResultCallback client.SelectPerResultCallback) error
	UseDB(dbName string) error
	GetServerVersion() string
}

// New implements [dbrepl.REPLNewFunc].
func New(_ context.Context, cfg *dbrepl.NewREPLConfig) (dbrepl.REPLInstance, error) {
	parser, err := newParser()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &REPL{
		Terminal:   term.NewTerminal(cfg.Client, ""),
		client:     cfg.Client,
		serverConn: cfg.ServerConn,
		route:      cfg.Route,
		parser:     parser,
	}, nil
}

// Run starts and run the PostgreSQL REPL session. The provided context is used
// to interrupt the execution and clean up resources.
func (r *REPL) Run(ctx context.Context) error {
	dialer := func(context.Context, string, string) (net.Conn, error) {
		return r.serverConn, nil
	}
	const hostnamePlaceholder = "repl"
	myConn, err := client.ConnectWithDialer(ctx, "tcp", hostnamePlaceholder,
		r.route.GetUsername(),
		r.testPassword,
		r.route.GetDatabase(),
		dialer,
		withClientCapabilities(
			mysql.CLIENT_MULTI_RESULTS,
			mysql.CLIENT_MULTI_STATEMENTS,
		),
		func(c *client.Conn) error {
			c.SetAttributes(map[string]string{clientNameParamName: clientNameParamValue})
			return nil
		},
	)
	if err != nil {
		return trace.ConnectionProblem(err, "Unable to connect to database: %v", err)
	}
	r.myConn = myConn
	defer func() {
		_ = myConn.Close()
	}()

	// term.Terminal blocks reads/writes without respecting the context. The
	// only thing that unblocks it is closing the underlaying connection (in
	// our case r.client). On this goroutine we only watch for context
	// cancelation and close the connection. This will unblocks all terminal
	// reads/writes.
	stop := context.AfterFunc(ctx, func() {
		_ = r.client.Close()
	})
	defer stop()

	if err := r.presentBanner(); err != nil {
		return trace.Wrap(err)
	}

	for {
		line, err := r.readLine()
		if err != nil {
			return trace.Wrap(rewriteTermError(ctx, err))
		}
		for reply, exit := range r.eval(line) {
			if exit {
				return nil
			}
			if err := r.print(reply); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

func withClientCapabilities(caps ...uint32) client.Option {
	return func(conn *client.Conn) error {
		for _, cap := range caps {
			conn.SetCapability(cap)
		}
		return nil
	}
}

func (r *REPL) presentBanner() error {
	_, err := fmt.Fprintf(
		r.Terminal,
		`Teleport MySQL interactive shell (v%s)
Connected to instance %q as user %q.
Type 'help' or '\h' for help.

`,
		cmp.Or(r.teleportVersion, teleport.Version),
		r.route.GetServiceName(),
		r.route.GetUsername())
	return trace.Wrap(err)
}

func (r *REPL) readLine() (string, error) {
	r.Terminal.SetPrompt(r.getPrompt())
	return r.Terminal.ReadLine()
}

func (r *REPL) getPrompt() string {
	dbName := formatDatabaseName(r.route.Database)
	var suffix string
	switch {
	case r.parser.lex.isInComment():
		suffix = "/*"
	case r.parser.lex.isInString():
		suffix = r.parser.lex.inStringKind()
	case r.parser.lex.isInQuery():
		suffix = "-"
	default:
		suffix = dbName
	}
	padding := len(dbName)
	// pad the suffix with leading spaces until it is len(dbName), for example:
	// default: "mysql>"
	// comment: "   */>"
	// string:  "    '>"
	// query:   "    ->"
	return fmt.Sprintf("%*s> ", padding, suffix)
}

func (r *REPL) eval(line string) iter.Seq2[string, bool] {
	return func(yield func(string, bool) bool) {
		for evaluator := range r.parser.parse(line) {
			if !yield(evaluator.eval(r)) {
				return
			}
		}
	}
}

// print writes a reply to the REPL client.
func (r *REPL) print(reply string) error {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil
	}
	_, err := r.Terminal.Write([]byte(lineBreak + reply + lineBreak + lineBreak))
	return trace.Wrap(err)
}

// rewriteTermError changes the term.Terminal error to match caller expectations.
func rewriteTermError(ctx context.Context, err error) error {
	// When context is canceled it will immediately lead read/write errors due
	// to the closed connection. For this cases we return the context error.
	if ctx.Err() != nil && (errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed)) {
		return ctx.Err()
	}
	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}

// summarizeResult summarizes a query result.
func summarizeResult(result *mysql.Result, numRows int, elapsed *time.Duration) string {
	var out string
	if result.Resultset != nil && len(result.Resultset.Fields) > 0 {
		out = summarizeResultset(numRows)
	} else {
		out = fmt.Sprintf("%v warnings, %v rows affected", result.Warnings, result.AffectedRows)
	}
	if elapsed != nil {
		return fmt.Sprintf("%s (%.2f sec)", out, elapsed.Seconds())
	}
	return out
}

// summarizeResultset summarizes a query result that contains rows.
func summarizeResultset(numRows int) string {
	switch numRows {
	case 0:
		return "Empty set"
	case 1:
		return "1 row in set"
	default:
		return fmt.Sprintf("%d rows in set", numRows)
	}
}

const (
	// lineBreak represents a line break on the REPL.
	lineBreak = "\n"
	// errorReplyPrefix is the prefix presented when there is a execution error.
	errorReplyPrefix = "ERR "
	// clientNameParamName defines the application name parameter name.
	//
	// https://dev.mysql.com/doc/refman/8.0/en/performance-schema-connection-attribute-tables.html
	clientNameParamName = "_client_name"
	// clientNameParamValue defines the application name parameter value.
	clientNameParamValue = "teleport-repl"
)
