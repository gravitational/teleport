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
	"bytes"
	"io"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/require"
)

func TestUpdateAppUsageTemplate(t *testing.T) {
	makeApp := func(usageWriter io.Writer) *kingpin.Application {
		app := InitCLIParser("TestUpdateAppUsageTemplate", "some help message")
		app.UsageWriter(usageWriter)
		app.Terminate(func(int) {})

		app.Command("hello", "Hello.")

		create := app.Command("create", "Create.")
		create.Command("box", "Box.")
		create.Command("rocket", "Rocket.")
		return app
	}

	tests := []struct {
		name           string
		inputArgs      []string
		outputContains string
	}{
		{
			name:      "command width aligned for app help",
			inputArgs: []string{},
			outputContains: `
Commands:
  help          Show help.
  hello         Hello.
  create box    Box.
  create rocket Rocket.
`,
		},
		{
			name:      "command width aligned for command help",
			inputArgs: []string{"create"},
			outputContains: `
Commands:
  create box    Box.
  create rocket Rocket.
`,
		},
		{
			name:      "command width aligned for unknown command error",
			inputArgs: []string{"unknown"},
			outputContains: `
Commands:
  help          Show help.
  hello         Hello.
  create box    Box.
  create rocket Rocket.
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("help flag", func(t *testing.T) {
				var buffer bytes.Buffer
				app := makeApp(&buffer)
				args := append(tt.inputArgs, "--help")
				UpdateAppUsageTemplate(app, args)

				app.Usage(args)
				require.Contains(t, buffer.String(), tt.outputContains)
			})

			t.Run("help command", func(t *testing.T) {
				var buffer bytes.Buffer
				app := makeApp(&buffer)
				args := append([]string{"help"}, tt.inputArgs...)
				UpdateAppUsageTemplate(app, args)

				// HelpCommand is triggered on PreAction during Parse.
				// See kingpin.Application.init for more details.
				_, err := app.Parse(args)
				require.NoError(t, err)
				require.Contains(t, buffer.String(), tt.outputContains)
			})
		})
	}
}
