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
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"net"
	"strings"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
	"golang.org/x/term"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/asciitable"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
)

// REPL implements [dbrepl.REPLInstance] for MySQL.
type REPL struct {
	client     io.ReadWriteCloser
	serverConn net.Conn
	route      clientproto.RouteToDatabase
	term       *term.Terminal
	commands   *commandManager
	myConn     mysqlConn
	lex        lexer

	// testPassword is normally blank, only used in tests where the REPL connects
	// directly to a MySQL instance without Teleport proxying.
	testPassword string
}

type mysqlConn interface {
	Execute(command string, args ...any) (*mysql.Result, error)
	UseDB(dbName string) error
	GetServerVersion() string
	GetConnectionID() uint32
}

// New implements [dbrepl.REPLNewFunc].
func New(_ context.Context, cfg *dbrepl.NewREPLConfig) (dbrepl.REPLInstance, error) {
	cmds, err := newCommands()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &REPL{
		client:     cfg.Client,
		serverConn: cfg.ServerConn,
		route:      cfg.Route,
		term:       term.NewTerminal(cfg.Client, ""),
		commands:   cmds,
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
		r.term,
		`Teleport MySQL interactive shell (v%s)
Connected to instance %q as user %q.
Type 'help' or '\h' for help.

`,
		teleport.Version,
		r.route.GetServiceName(),
		r.route.GetUsername())
	return trace.Wrap(err)
}

func (r *REPL) readLine() (string, error) {
	r.term.SetPrompt(r.getPrompt())
	return r.term.ReadLine()
}

func (r *REPL) getPrompt() string {
	dbName := formatDatabaseName(r.route.Database)
	var suffix string
	switch {
	case r.lex.isInComment():
		suffix = "/*"
	case r.lex.isInString():
		suffix = r.lex.inStringKind()
	case r.lex.isInQuery():
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
	r.lex.setLine(line)
	return func(yield func(string, bool) bool) {
		for !r.lex.isEmpty() {
			switch {
			case r.lex.isInComment():
				r.lex.discardMultiLineComment()
			case r.lex.isInString():
				r.lex.acceptString()
			case !r.lex.isInQuery():
				if !yield(r.parseCommand()) {
					return
				}
			default:
				if !yield(r.parseQuery()) {
					return
				}
			}
		}
		if r.lex.isInQuery() {
			r.lex.writeString(lineBreak)
		}
	}
}

// print writes a reply to the REPL client.
func (r *REPL) print(reply string) error {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil
	}
	_, err := r.term.Write([]byte(lineBreak + reply + lineBreak + lineBreak))
	return trace.Wrap(err)
}

// parseCommand looks for a command and its args on the current line, executing
// the command and returning a client reply and whether the REPL should exit.
// If the line contains the current delimiter and the command is not the special
// DELIMITER command, then it must be parsed as a potential query instead.
func (r *REPL) parseCommand() (string, bool) {
	r.lex.discardWhitespace()
	line := r.lex.peekString()
	cmd, args, err := r.commands.findCommand(line)
	switch {
	case err != nil:
		r.lex.discardRemaining()
		return err.Error(), false
	case cmd != nil:
		if cmd.name != "delimiter" && strings.Contains(line, r.lex.delimiter()) {
			return r.parseQuery()
		}
		r.lex.discardRemaining()
		return cmd.execFunc(r, args)
	default:
		return r.parseQuery()
	}
}

// parseQuery looks for a query in the input, executing the query and returning
// a client reply and whether the REPL should exit.
func (r *REPL) parseQuery() (string, bool) {
	if r.lex.advanceByDelimiter() {
		query := r.lex.getQuery()
		if !r.lex.isMultilineQuery() || strings.HasPrefix(query, r.commands.shortcutPrefix) {
			cmd, args, err := r.commands.findCommand(query)
			switch {
			case err != nil:
				return err.Error(), false
			case cmd != nil:
				return cmd.execFunc(r, args)
			}
		}
		return r.runStatement(query), false
	}

	tok := r.lex.scan()
	switch tok.kind {
	case tokenSingleComment:
		// intentionally skip writing out comments to our query buffer
		r.lex.discardSingleLineComment()
	case tokenOpenComment:
		r.lex.setOpenComment()
		r.lex.discardMultiLineComment()
	case tokenBackslash:
		r.lex.writeString(tok.text)
		r.lex.acceptEscapedRune()
	case tokenSingleQuote, tokenDoubleQuote, tokenBacktick:
		r.lex.writeString(tok.text)
		r.lex.setOpenString(tok)
		r.lex.acceptString()
	default:
		r.lex.writeString(tok.text)
	}
	return "", false
}

// runStatement executes a single statement and formats the response as a reply
// to the REPL client.
func (r *REPL) runStatement(statement string) string {
	result, err := r.myConn.Execute(statement)
	if err != nil {
		return errorReplyPrefix + err.Error()
	}
	defer result.Close()
	reply := formatResult(result)
	return reply
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

// formatResult formats a query result.
func formatResult(result *mysql.Result) string {
	if result.Resultset != nil {
		return lineBreak + formatResultset(result.Resultset)
	}
	return lineBreak + fmt.Sprintf("%v warnings, %v rows affected", result.Warnings, result.AffectedRows)
}

// formatResultset formats a query result that contains rows.
func formatResultset(set *mysql.Resultset) string {
	if set == nil || len(set.Values) == 0 {
		return "Empty set"
	}
	var sb strings.Builder
	columns := make([]string, 0, len(set.Fields))
	for _, f := range set.Fields {
		columns = append(columns, string(f.Name))
	}
	table := asciitable.MakeTable(columns)
	for _, rowValues := range set.Values {
		row := make([]string, 0, len(rowValues))
		for _, val := range rowValues {
			row = append(row, val.String())
		}
		table.AddRow(row)
	}
	table.WriteTo(&sb)
	sb.WriteString(lineBreak)

	if numRows := len(set.Values); numRows == 1 {
		sb.WriteString("(1 row in set)")
	} else {
		// pluralize 0 or multiple rows
		fmt.Fprintf(&sb, "(%d rows in set)", numRows)
	}
	return sb.String()
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
