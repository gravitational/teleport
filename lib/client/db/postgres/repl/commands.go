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
	"fmt"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
)

// processCommand receives a command call and return the reply and if the
// command terminates the session.
func (r *REPL) processCommand(line string) (string, bool) {
	cmdStr, args, _ := strings.Cut(strings.TrimPrefix(line, commandPrefix), " ")
	cmd, ok := r.commands[cmdStr]
	if !ok {
		return "Unknown command. Try \\? to show the list of supported commands." + lineBreak, false
	}

	return cmd.ExecFunc(r, args)
}

// commandType specify the command category. This is used to organize the
// commands, for example, when showing them in the help command.
type commandType string

const (
	// commandTypeGeneral represents a general-purpose command type.
	commandTypeGeneral commandType = "General"
	// commandTypeConnection represents a command type related to connection
	// operations.
	commandTypeConnection = "Connection"
)

// command represents a command that can be executed in the REPL.
type command struct {
	// Type specifies the type of the command.
	Type commandType
	// Description provides a user-friendly explanation of what the command
	// does.
	Description string
	// ExecFunc is the function to execute the command. The commands can either
	// return a reply (that will be sent back to the client) as a string. It can
	// terminate the REPL by returning bool on the second argument.
	ExecFunc func(r *REPL, args string) (reply string, exit bool)
}

func initCommands() map[string]*command {
	return map[string]*command{
		"q": {
			Type:        commandTypeGeneral,
			Description: "Terminates the session.",
			ExecFunc:    func(_ *REPL, _ string) (string, bool) { return "", true },
		},
		"teleport": {
			Type:        commandTypeGeneral,
			Description: "Show Teleport interactive shell information, such as execution limitations.",
			ExecFunc: func(_ *REPL, _ string) (string, bool) {
				// Formats limitiations in a dash list. Example:
				// - hello
				//   multi line
				// - another item
				var limitations strings.Builder
				for _, l := range descriptiveLimitations {
					limitations.WriteString("- " + strings.Join(strings.Split(l, "\n"), "\n  ") + lineBreak)
				}

				return fmt.Sprintf(
					"Teleport PostgreSQL interactive shell (v%s)\n\nLimitations: \n%s",
					teleport.Version,
					limitations.String(),
				), false
			},
		},
		"?": {
			Type:        commandTypeGeneral,
			Description: "Show the list of supported commands.",
			ExecFunc: func(r *REPL, _ string) (string, bool) {
				typesTable := make(map[commandType]*asciitable.Table)
				for cmdStr, cmd := range r.commands {
					if _, ok := typesTable[cmd.Type]; !ok {
						table := asciitable.MakeHeadlessTable(2)
						typesTable[cmd.Type] = &table
					}

					typesTable[cmd.Type].AddRow([]string{"\\" + cmdStr, cmd.Description})
				}

				var res strings.Builder
				for cmdType, output := range typesTable {
					res.WriteString(string(cmdType) + lineBreak)
					output.AsBuffer().WriteTo(&res)
					res.WriteString(lineBreak)
				}

				return res.String(), false
			},
		},
		"session": {
			Type:        commandTypeConnection,
			Description: "Display information about the current session, like user, and database instance.",
			ExecFunc: func(r *REPL, _ string) (string, bool) {
				return fmt.Sprintf("Connected to %q instance at %q database as %q user.", r.route.ServiceName, r.route.Database, r.route.Username), false
			},
		},
	}
}
