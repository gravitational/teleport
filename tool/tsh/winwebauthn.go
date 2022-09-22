// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/lib/auth/winwebauthn"
	"github.com/gravitational/trace"
)

type winwebauthnCommand struct {
	support     *winwebauthnSupportCommand
	diagnostics *winwebauthnDiagnosticsCommand
}

// newWinwebauthnCommand returns winwebauthn subcommands.
// support is always available.
func newWinwebauthnCommand(app *kingpin.Application) *winwebauthnCommand {
	wid := app.Command("winwebauthn", "Manage Windoes webauthn").Hidden()
	cmd := &winwebauthnCommand{
		support:     newWinwebauthnSupportCommand(wid),
		diagnostics: newWinwebauthnDiagnosticsCommand(wid),
	}
	return cmd
}

type winwebauthnSupportCommand struct {
	*kingpin.CmdClause
}

func newWinwebauthnSupportCommand(app *kingpin.CmdClause) *winwebauthnSupportCommand {
	return &winwebauthnSupportCommand{
		CmdClause: app.Command("support", "Check windows webauthn support").Hidden(),
	}
}

func (w *winwebauthnSupportCommand) run(cf *CLIConf) error {
	diag := winwebauthn.CheckSupport()
	fmt.Printf("\nWinwebauthn available: %v\n", diag.HasCompileSupport)
	fmt.Printf("Compiple support: %v\n", diag.IsAvailable)
	fmt.Printf("API version: %v\n", diag.APIVersion)
	fmt.Printf("Has platform UV: %v\n", diag.HasPlatformUV)

	return nil
}

type winwebauthnDiagnosticsCommand struct {
	*kingpin.CmdClause
}

func newWinwebauthnDiagnosticsCommand(app *kingpin.CmdClause) *winwebauthnDiagnosticsCommand {
	return &winwebauthnDiagnosticsCommand{
		CmdClause: app.Command("diagnostics", "Run Windows webauthn diagnostics").Hidden(),
	}
}

func (w *winwebauthnDiagnosticsCommand) run(cf *CLIConf) error {
	diag, err := winwebauthn.RunDiagnostics(cf.Context, os.Stdout)
	// Abort if we got a nil diagnostic, otherwise print as much as we can.
	if diag == nil {
		return trace.Wrap(err)
	}

	fmt.Printf("\nWinwebauthn available: %v\n", diag.Available)
	fmt.Printf("Register successful? %v\n", diag.RegisterSuccessful)
	fmt.Printf("Login successful? %v\n", diag.LoginSuccessful)
	if err != nil {
		fmt.Println()
	}

	return trace.Wrap(err)
}
