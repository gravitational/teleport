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
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
)

type commandManager struct {
	byName     map[string]*command
	byShortcut map[rune]*command
	// shortcutPrefix is the prefix for command shortcuts, e.g. \h for help.
	shortcutPrefix string
}

// command represents a command that can be executed in the REPL.
type command struct {
	// name is the command's full name.
	name string
	// shortcut is the command's escaped single char name: \x.
	shortcut rune
	// description provides a user-friendly explanation of what the command
	// does.
	description string
	// execFunc is the function to execute the command. The commands can either
	// return a reply (that will be sent back to the client) as a string. It can
	// terminate the REPL by returning bool on the second argument.
	execFunc func(r *REPL, args string) (reply string, exit bool)
}

func (c *command) checkAndSetDefaults() error {
	if len(c.name) == 0 {
		return trace.BadParameter("missing command name")
	}
	if c.shortcut == 0 {
		return trace.BadParameter("missing command short name")
	}
	if len(c.description) == 0 {
		return trace.BadParameter("missing command description")
	}
	if c.execFunc == nil {
		return trace.BadParameter("missing command exec func")
	}
	c.name = normalizeCommandName(c.name)
	return nil
}

func normalizeCommandName(name string) string {
	return strings.ToLower(name)
}

func newCommands() (*commandManager, error) {
	c := &commandManager{
		byName:         make(map[string]*command),
		byShortcut:     make(map[rune]*command),
		shortcutPrefix: `\`,
	}
	for _, cmd := range []*command{
		{
			name:        "delimiter",
			shortcut:    'd',
			description: "Set statement delimiter. With no argument the delimiter resets to the default.",
			execFunc: func(r *REPL, args string) (reply string, exit bool) {
				delim := getArg(args)
				if err := r.parser.lex.setDelimiter(delim); err != nil {
					return err.Error(), false
				}
				return "", false
			},
		},
		{
			name:        "help",
			shortcut:    'h',
			description: "Show the list of supported commands.",
			execFunc: func(_ *REPL, _ string) (string, bool) {
				return c.help(), false
			},
		},
		{
			name:        "quit",
			shortcut:    'q',
			description: "Terminates the session.",
			execFunc:    func(_ *REPL, _ string) (string, bool) { return "", true },
		},
		{
			name:        "status",
			shortcut:    's',
			description: "Get status information.",
			execFunc: func(r *REPL, _ string) (string, bool) {
				table := asciitable.MakeHeadlessTable(2)
				table.AddRow([]string{"Teleport database:", r.route.ServiceName})
				table.AddRow([]string{"Current database:", formatDatabaseName(r.route.Database)})
				table.AddRow([]string{"Current user:", r.route.Username})
				table.AddRow([]string{"Server version:", r.myConn.GetServerVersion()})
				table.AddRow([]string{"Using delimiter:", r.parser.lex.delimiter()})
				return table.String(), false
			},
		},
		{
			name:        "teleport",
			shortcut:    't',
			description: "Show Teleport interactive shell information, such as execution limitations.",
			execFunc: func(r *REPL, _ string) (string, bool) {
				var sb strings.Builder
				for _, l := range descriptiveLimitations {
					sb.WriteString("- " + strings.ReplaceAll(l, "\n", "\n  ") + lineBreak)
				}
				return fmt.Sprintf(
					"Teleport MySQL interactive shell (v%s)\n\nLimitations: \n%s",
					cmp.Or(r.teleportVersion, teleport.Version),
					sb.String(),
				), false
			},
		},
		{
			name:        "use",
			shortcut:    'u',
			description: "Use another database.",
			execFunc: func(r *REPL, args string) (string, bool) {
				args = strings.TrimSpace(args)
				if len(args) == 0 {
					return "USE must be followed by a database name", false
				}
				dbName, err := getDatabaseName(r.myConn, args)
				if err != nil {
					return err.Error(), false
				}
				if err := r.myConn.UseDB(dbName); err != nil {
					return err.Error(), false
				}
				r.route.Database = dbName
				return fmt.Sprintf(`Default database changed to %q`, dbName), false
			},
		},
	} {
		if err := cmd.checkAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := c.add(cmd); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return c, nil
}

func (c *commandManager) add(cmd *command) error {
	if _, ok := c.byName[cmd.name]; ok {
		return trace.BadParameter("overlapping command name: %s", cmd.name)
	}
	c.byName[cmd.name] = cmd
	if _, ok := c.byShortcut[cmd.shortcut]; ok {
		return trace.BadParameter("overlapping command shortcut: %c", cmd.shortcut)
	}
	c.byShortcut[cmd.shortcut] = cmd
	return nil
}

// findCommand tries to find a command at the start of a line. If a command name
// or short name matches, then it returns the command and the rest of the line.
// If no command is found and the line does not start with the short name prefix
// then the command and error returned will both be nil.
func (c *commandManager) findCommand(line string) (*command, string, error) {
	line = strings.TrimSpace(line)
	if before, after, found := strings.Cut(line, c.shortcutPrefix); found && len(before) == 0 {
		shortcut, width := utf8.DecodeRuneInString(after)
		if width > 0 && shortcut != utf8.RuneError {
			if cmd, ok := c.byShortcut[shortcut]; ok {
				return cmd, after[1:], nil
			}
		}
		return nil, "", trace.Errorf("Unknown command.\n%v", c.help())
	}

	name, after, _ := strings.Cut(line, " ")
	if cmd, ok := c.byName[normalizeCommandName(name)]; ok {
		return cmd, after, nil
	}
	return nil, "", nil
}

func (c *commandManager) help() string {
	var res strings.Builder
	cmds := slices.Collect(maps.Values(c.byName))
	slices.SortStableFunc(cmds, func(a, b *command) int {
		return strings.Compare(a.name, b.name)
	})
	table := asciitable.MakeHeadlessTable(3)
	for _, cmd := range cmds {
		short := fmt.Sprintf("%s%c", c.shortcutPrefix, cmd.shortcut)
		row := []string{cmd.name, short, cmd.description}
		table.AddRow(row)
	}
	table.WriteTo(&res)
	res.WriteString(lineBreak)
	return res.String()
}

// getArg trims spaces from the line and returns the first space separated arg.
func getArg(line string) string {
	line = strings.TrimSpace(line)
	before, _, _ := strings.Cut(line, " ")
	return before
}

// getDatabaseName extracts a database name from args.
// Since USE is both a command (COM_INIT_DB) and a statement, as a client
// we have to parse the database argument for COM_INIT_DB.
// To handle quotation, we send the args to the server as a statement and then
// ask the server what the current database is.
func getDatabaseName(conn mysql.Executer, args string) (string, error) {
	result, err := conn.Execute("USE " + args)
	if err != nil {
		return "", trace.Wrap(err)
	}
	result.Close()
	return getCurrentDatabase(conn)
}

func getCurrentDatabase(conn mysql.Executer) (string, error) {
	result, err := conn.Execute("SELECT DATABASE() LIMIT 1")
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer result.Close()
	if result.Resultset == nil || len(result.Resultset.Values) == 0 || len(result.Resultset.Values[0]) == 0 {
		return "", trace.NotFound("missing result")
	}
	val := result.Resultset.Values[0][0]
	if val.Type == mysql.FieldValueTypeString {
		return string(val.AsString()), nil
	}
	return "", nil
}

func formatDatabaseName(name string) string {
	return cmp.Or(name, "(none)")
}

// descriptiveLimitations defines a user-friendly text containing the REPL
// limitations.
var descriptiveLimitations = []string{
	`Query cancellation is not supported. Once a query is sent, its execution
can only be canceled by killing it from another session.
You can identify the query process ID with "SHOW PROCESSLIST" and then kill the
query with "KILL QUERY <pid>".
`,
	// This limitation is due to our terminal emulator not fully supporting this
	// shortcut's custom handler. Instead, it will close the terminal, leading
	// to terminating the session. To avoid having users accidentally
	// terminating their sessions, we're turning this off until we have a better
	// solution and propose the behavior for it.
	//
	// This shortcut filtered out by the WebUI key handler.
	"Pressing CTRL-C will have no effect in this shell.",
}
