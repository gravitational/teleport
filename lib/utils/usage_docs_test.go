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

//go:build docs

package utils

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/require"
)

func TestUpdateAppUsageTemplate(t *testing.T) {
	tests := []struct {
		name            string
		makeApp         func() *kingpin.Application
		expectSubstring string // The @ character is replaced with a backtick
	}{
		{
			name: "subcommand flags and global flags",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				app.Command("hello", "Hello.")
				create := app.Command("create", "Create.")
				create.Flag("name", "The name of the resource").Default("myresource").String()
				createRocket := create.Command("rocket", "Rocket.")
				createRocket.Flag("launch", "Whether to launch the Rocket").Bool()
				return app
			},
			expectSubstring: `---
title: myapp Reference
description: Provides a comprehensive list of commands, arguments, and flags for myapp.
---

This guide provides a comprehensive list of commands, arguments, and flags for
myapp: This is the main CLI tool.

@@@code
$ myapp [<flags>] <command> [<args> ...]
@@@

Global flags:

|Flag|Default|Description|
|---|---|---|
|@--config@|@config.yaml@|The location of the config file|

## myapp create rocket

Rocket.

Usage:

@@@code
$ myapp create rocket [<flags>]
@@@

Flags:

|Flag|Default|Description|
|---|---|---|
|@--[no-]launch@|@false@|Whether to launch the Rocket|

## myapp hello

Hello.

Usage:

@@@code
$ myapp hello
@@@

## myapp help

Show help.

Usage:

@@@code
$ myapp help [<command>...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|command|none (optional)|Show help on command.|

`,
		},
		{
			name: "multiple main command flags",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				app.Flag("verbosity", "Verbosity level.").Default("3").Int()
				app.Flag("dry-run", "Whether to use dry-run mode").Default("false").Bool()
				return app
			},
			expectSubstring: `This guide provides a comprehensive list of commands, arguments, and flags for
myapp: This is the main CLI tool.

@@@code
$ myapp [<flags>] <command> [<args> ...]
@@@

Global flags:

|Flag|Default|Description|
|---|---|---|
|@--config@|@config.yaml@|The location of the config file|
|@--verbosity@|@3@|Verbosity level.|
|@--[no-]dry-run@|@false@|Whether to use dry-run mode|

`,
		},
		{
			name: "multiple subcommand flags",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				create := app.Command("create", "Create a resource.")
				create.Flag("verbosity", "Verbosity level.").Default("3").Int()
				create.Flag("dry-run", "Whether to use dry-run mode").Default("false").Bool()
				return app
			},
			expectSubstring: `## myapp create

Create a resource.

Usage:

@@@code
$ myapp create [<flags>]
@@@

Flags:

|Flag|Default|Description|
|---|---|---|
|@--verbosity@|@3@|Verbosity level.|
|@--[no-]dry-run@|@false@|Whether to use dry-run mode|

`,
		},
		{
			name: "multiple sub-command args",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				create := app.Command("create", "Create.")
				create.Arg("verbosity", "Verbosity level.").Default("3").Int()
				create.Arg("dry-run", "Whether to use dry-run mode").Default("false").Bool()
				return app
			},
			expectSubstring: `## myapp create

Create.

Usage:

@@@code
$ myapp create [<verbosity>] [<dry-run>]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|verbosity|@3@ (optional)|Verbosity level.|
|dry-run|@false@ (optional)|Whether to use dry-run mode|

`,
		},
		{
			name: "sub-command order",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				app.Command("create", "Create a resource.")
				app.Command("validate", "Validate the config.")
				app.Command("connect", "Connect to a server.")
				return app
			},
			expectSubstring: `## myapp connect

Connect to a server.

Usage:

@@@code
$ myapp connect
@@@

## myapp create

Create a resource.

Usage:

@@@code
$ myapp create
@@@

## myapp help

Show help.

Usage:

@@@code
$ myapp help [<command>...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|command|none (optional)|Show help on command.|

## myapp validate

Validate the config.

Usage:

@@@code
$ myapp validate
@@@

`,
		},
		{
			name: "level-3 command order",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				mfa := app.Command("mfa", "Manage MFA resources.")
				mfa.Command("add", "Add an MFA device.")
				app.Command("create", "Create a resource")
				return app
			},
			expectSubstring: `## myapp create

Create a resource

Usage:

@@@code
$ myapp create
@@@

## myapp help

Show help.

Usage:

@@@code
$ myapp help [<command>...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|command|none (optional)|Show help on command.|

## myapp mfa add

Add an MFA device.

Usage:

@@@code
$ myapp mfa add
@@@

`,
		},
		{
			name: "empty arg",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				app.Command("kubectl", "Proxy kubectl commands.")
				kubectl := app.Command("kubectl", "Proxy kubectl commands.").Interspersed(false)
				// This hack is required in order to accept any args for tsh kubectl.
				kubectl.Arg("", "").StringsVar(new([]string))

				return app
			},
			expectSubstring: `## myapp kubectl

Proxy kubectl commands.

Usage:

@@@code
$ myapp kubectl [args...]
@@@

Arguments:

|Argument|Default|Description|
|---|---|---|
|args|none (optional)|Arbitrary arguments|

`,
		},
		{
			name: "hidden flag",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				app.Command("kubectl", "Proxy kubectl commands.")
				kubectl := app.Command("kubectl", "Proxy kubectl commands.").Interspersed(false)
				kubectl.Flag("diag", "Run diagnostics").Hidden().Bool()
				app.Command("log", "Print logs")

				return app
			},
			expectSubstring: `## myapp kubectl

Proxy kubectl commands.

Usage:

@@@code
$ myapp kubectl
@@@

## myapp log
`,
		},
		{
			name: "main command env vars",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("verbosity", "Verbosity level.").Default("3").Envar("MYAPP_VERBOSITY").Int()
				return app
			},
			expectSubstring: `Global flags:

|Flag|Default|Description|
|---|---|---|
|@--verbosity@|@3@|Verbosity level.|

Global environment variables:

|Variable|Default|Description|
|---|---|---|
|@MYAPP_VERBOSITY@|@3@|Verbosity level.|

`,
		},
		{
			name: "subcommand env vars",
			makeApp: func() *kingpin.Application {
				app := InitCLIParser("myapp", "This is the main CLI tool.")
				app.Flag("config", "The location of the config file").Default("config.yaml").String()
				create := app.Command("create", "Create a resource.")
				create.Flag("name", "The name of the resource").Envar("CREATE_NAME").Default("myresource").String()
				create.Arg("type", "The type of the resource").Envar("CREATE_TYPE").String()

				return app
			},
			expectSubstring: `## myapp create

Create a resource.

Usage:

@@@code
$ myapp create [<flags>] [<type>]
@@@

Environment variables:

|Variable|Default|Description|
|---|---|---|
|@CREATE_TYPE@|none (optional)|The type of the resource|
|@CREATE_NAME@|@myresource@|The name of the resource|

Flags:

|Flag|Default|Description|
|---|---|---|
|@--name@|@myresource@|The name of the resource|

Arguments:

|Argument|Default|Description|
|---|---|---|
|type|none (optional)|The type of the resource|

`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.makeApp()
			var buffer bytes.Buffer
			app.UsageWriter(&buffer)
			args := []string{"help"}
			app.Terminate(func(int) {})

			docsUsageTemplatePath := "docs-usage.md.tmpl"
			f, err := os.Open(docsUsageTemplatePath)
			require.NoError(t, err)
			updateAppUsageTemplate(f, app)

			// kingpin only adds a help command if there is at least
			// one subcommand. Make sure that all test cases
			// introduce a help command.
			app.HelpCommand = app.Command("help", "Print help for the application.")
			// HelpCommand is triggered on PreAction during Parse.
			// See kingpin.Application.init for more details.
			_, err = app.Parse(args)
			require.NoError(t, err)
			expected := strings.ReplaceAll(tt.expectSubstring, "@", "`")
			require.Contains(t, buffer.String(), expected)
		})
	}
}
