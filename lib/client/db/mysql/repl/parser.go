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
	"iter"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
)

func newParser() (*parser, error) {
	commands, err := newCommands()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &parser{commands: commands}, nil
}

// parser parses commands and queries given one or more lines of input.
type parser struct {
	commands *commandManager
	lex      lexer
}

// parse returns an iterator over the input line yielding evaluators for each
// command or query found.
func (p *parser) parse(line string) iter.Seq[evaluator] {
	p.lex.setLine(line)
	return func(yield func(evaluator) bool) {
		defer func() {
			if p.lex.isEmpty() && p.lex.isInQuery() {
				// must be a multiline query, so insert a linebreak in the query buf
				p.lex.writeString(lineBreak)
			}
		}()

		for !p.lex.isEmpty() {
			if p.lex.isInComment() {
				// dont preserve comments
				p.lex.discardMultiLineComment()
				continue
			}

			if !p.lex.isInQuery() {
				// not in a comment or query: is the rest of the line a command?
				if cmd := p.tryParseCommand(); cmd != nil {
					if !yield(cmd) {
						return
					}
					continue
				}
			}

			if p.lex.isInString() {
				p.lex.acceptString()
				continue
			}

			// not in comment nor string: do we have a command/query delimiter?
			if cmdOrQuery := p.tryParseQuery(); cmdOrQuery != nil {
				if !yield(cmdOrQuery) {
					return
				}
				continue
			}

			// not in comment nor string and no delimiter found: scan text
			tok := p.lex.scan()
			switch tok.kind {
			case tokenSingleComment:
				// dont preserve comments
				p.lex.discardSingleLineComment()
			case tokenOpenComment:
				p.lex.setOpenComment()
				p.lex.discardMultiLineComment()
			case tokenBackslash:
				p.lex.writeString(tok.text)
				p.lex.acceptEscapedRune()
			case tokenSingleQuote, tokenDoubleQuote, tokenBacktick:
				p.lex.writeString(tok.text)
				p.lex.setOpenString(tok)
				p.lex.acceptString()
			default:
				p.lex.writeString(tok.text)
			}
		}
	}
}

// tryParseCommand tries to parse a command and its args on the current line.
// If the line contains the current delimiter and the command is not the special
// DELIMITER command, then give up and let the query parser handle it.
func (p *parser) tryParseCommand() evaluator {
	p.lex.discardWhitespace()
	line := p.lex.peekString()
	cmd, args, err := p.commands.findCommand(line)
	switch {
	case err != nil:
		if strings.Contains(line, p.lex.delimiter()) {
			return nil
		}
		p.lex.discardRemaining()
		return errorEvaluator{err: err}
	case cmd != nil:
		if cmd.name != "delimiter" && strings.Contains(line, p.lex.delimiter()) {
			return nil
		}
		p.lex.discardRemaining()
		return commandEvaluator{cmd: cmd, args: args}
	default:
		return nil
	}
}

// tryParseQuery looks for a delimiter in the input. If it finds one and the
// query buffer is all from one line, then it first looks for a command in the
// query buffer. If there is no command or the query buffer is multiline, then
// it returns a query evaluator.
func (p *parser) tryParseQuery() evaluator {
	if p.lex.advanceByDelimiter() {
		query := p.lex.getQuery()
		if !p.lex.isMultilineQuery() || strings.HasPrefix(query, p.commands.shortcutPrefix) {
			cmd, args, err := p.commands.findCommand(query)
			switch {
			case err != nil:
				// don't discard the rest of the line, since we found the
				// command in the query buffer, unlike the command parser which
				// looks ahead in the current line for a command
				return errorEvaluator{err: err}
			case cmd != nil:
				return commandEvaluator{cmd: cmd, args: args}
			}
		}
		return queryEvaluator(query)
	}
	return nil
}

type evaluator interface {
	eval(r *REPL) (reply string, exit bool)
}

// queryEvaluator executes a single query and formats the response as a reply
// to the REPL client.
type queryEvaluator string

func (query queryEvaluator) eval(r *REPL) (string, bool) {
	const maxBufferedRows = 100
	start := time.Now()
	var (
		res      mysql.Result
		rowCount int
	)
	defer res.Close()
	writer := tabwriter.NewWriter(r.Terminal, 5, 0, 1, ' ', 0)
	err := r.myConn.ExecuteSelectStreaming(string(query), &res,
		func(row []mysql.FieldValue) error {
			for _, item := range row {
				if _, err := writer.Write([]byte(item.String() + "\t")); err != nil {
					return trace.Wrap(err)
				}
			}
			if _, err := writer.Write([]byte(lineBreak)); err != nil {
				return trace.Wrap(err)
			}
			rowCount++
			if rowCount%maxBufferedRows == 0 {
				return trace.Wrap(writer.Flush())
			}
			return nil
		},
		func(result *mysql.Result) error {
			headers := make([]string, 0, len(result.Resultset.Fields))
			dashes := make([]string, 0, cap(headers))
			for _, f := range result.Resultset.Fields {
				headers = append(headers, string(f.Name))
				dashes = append(dashes, strings.Repeat("-", len(f.Name)))
			}
			headerLine := lineBreak + strings.Join(headers, "\t")
			sepLine := lineBreak + strings.Join(dashes, "\t")
			if _, err := writer.Write([]byte(headerLine + sepLine + lineBreak)); err != nil {
				return trace.Wrap(err)
			}
			return nil
		},
	)
	if err != nil {
		return errorReplyPrefix + err.Error(), false
	}
	if err := writer.Flush(); err != nil {
		return errorReplyPrefix + err.Error(), false
	}

	if r.disableQueryTimings {
		return summarizeResult(&res, rowCount, nil), false
	}

	elapsed := time.Since(start)
	return summarizeResult(&res, rowCount, &elapsed), false
}

// commandEvaluator executes a command.
type commandEvaluator struct {
	cmd  *command
	args string
}

func (e commandEvaluator) eval(r *REPL) (string, bool) {
	return e.cmd.execFunc(r, e.args)
}

type errorEvaluator struct {
	err error
}

// errorEvaluator formats an error as a reply to the REPL client.
func (e errorEvaluator) eval(_ *REPL) (string, bool) {
	return e.err.Error(), false
}
