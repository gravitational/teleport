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
	"github.com/gravitational/teleport/lib/auth/webauthnwin"
	"github.com/gravitational/trace"
)

type webauthnwinCommand struct {
	support     *webauthnwinSupportCommand
	diagnostics *webauthnwinDiagnosticsCommand
}

// newWebauthnwinCommand returns webauthnwin subcommands.
// support is always available.
func newWebauthnwinCommand(app *kingpin.Application) *webauthnwinCommand {
	wid := app.Command("webauthnwin", "Manage Windows WebAuthn").Hidden()
	cmd := &webauthnwinCommand{
		support:     newWebauthnwinSupportCommand(wid),
		diagnostics: newWebauthnwinDiagnosticsCommand(wid),
	}
	return cmd
}

type webauthnwinSupportCommand struct {
	*kingpin.CmdClause
}

func newWebauthnwinSupportCommand(app *kingpin.CmdClause) *webauthnwinSupportCommand {
	return &webauthnwinSupportCommand{
		CmdClause: app.Command("support", "Check windows webauthn support").Hidden(),
	}
}

func (w *webauthnwinSupportCommand) run(cf *CLIConf) error {
	diag := webauthnwin.CheckSupport()
	fmt.Printf("\nwebauthnwin available: %v\n", diag.HasCompileSupport)
	fmt.Printf("Compiple support: %v\n", diag.IsAvailable)
	fmt.Printf("API version: %v\n", diag.WebAuthnAPIVersion)
	fmt.Printf("Has platform UV: %v\n", diag.HasPlatformUV)

	return nil
}

type webauthnwinDiagnosticsCommand struct {
	*kingpin.CmdClause
}

func newWebauthnwinDiagnosticsCommand(app *kingpin.CmdClause) *webauthnwinDiagnosticsCommand {
	return &webauthnwinDiagnosticsCommand{
		CmdClause: app.Command("diagnostics", "Run Windows webauthn diagnostics").Hidden(),
	}
}

func (w *webauthnwinDiagnosticsCommand) run(cf *CLIConf) error {
	diag, err := webauthnwin.RunDiagnostics(cf.Context, os.Stdout)
	// Abort if we got a nil diagnostic, otherwise print as much as we can.
	if diag == nil {
		return trace.Wrap(err)
	}

	fmt.Printf("\nwebauthnwin available: %v\n", diag.Available)
	fmt.Printf("Register successful? %v\n", diag.RegisterSuccessful)
	fmt.Printf("Login successful? %v\n", diag.LoginSuccessful)
	if err != nil {
		fmt.Println()
	}

	return trace.Wrap(err)
}
