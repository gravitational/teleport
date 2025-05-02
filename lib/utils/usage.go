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

//go:build !docs

package utils

import (
	"context"
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
)

// UpdateAppUsageTemplate updates usage template for kingpin applications by
// pre-parsing the arguments then applying any changes to the usage template if
// necessary.
func UpdateAppUsageTemplate(app *kingpin.Application, args []string) {
	app.UsageTemplate(createUsageTemplate(
		withCommandPrintfWidth(app, args),
	))
}

// withCommandPrintfWidth returns a usage template option that
// updates command printf width if longer than default.
func withCommandPrintfWidth(app *kingpin.Application, args []string) func(*usageTemplateOptions) {
	return func(opt *usageTemplateOptions) {
		var commands []*kingpin.CmdModel

		// When selected command is "help", skip the "help" arg
		// so the intended command is selected for calculation.
		if len(args) > 0 && args[0] == "help" {
			args = args[1:]
		}

		appContext, err := app.ParseContext(args)
		switch {
		case appContext == nil:
			slog.WarnContext(context.Background(), "No application context found")
			return

		// Note that ParseContext may return the current selected command that's
		// causing the error. We should continue in those cases when appContext is
		// not nil.
		case err != nil:
			slog.InfoContext(context.Background(), "Error parsing application context", "error", err)
		}

		if appContext.SelectedCommand != nil {
			commands = appContext.SelectedCommand.Model().FlattenedCommands()
		} else {
			commands = app.Model().FlattenedCommands()
		}

		for _, command := range commands {
			if !command.Hidden && len(command.FullCommand) > opt.commandPrintfWidth {
				opt.commandPrintfWidth = len(command.FullCommand)
			}
		}
	}
}
