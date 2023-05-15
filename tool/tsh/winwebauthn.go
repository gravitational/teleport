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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/webauthnwin"
)

type webauthnwinCommand struct {
	diag *webauthnwinDiagCommand
}

// newWebauthnwinCommand returns webauthnwin subcommands.
// `diag` is always available.
func newWebauthnwinCommand(app *kingpin.Application) *webauthnwinCommand {
	wid := app.Command("webauthnwin", "Manage Windows WebAuthn").Hidden()
	cmd := &webauthnwinCommand{
		diag: newWebauthnwinDiagCommand(wid),
	}
	return cmd
}

type webauthnwinDiagCommand struct {
	*kingpin.CmdClause
}

func newWebauthnwinDiagCommand(app *kingpin.CmdClause) *webauthnwinDiagCommand {
	return &webauthnwinDiagCommand{
		CmdClause: app.Command("diag", "Run windows webauthn diagnostics").Hidden(),
	}
}

func (w *webauthnwinDiagCommand) run(cf *CLIConf) error {
	diag := webauthnwin.CheckSupport()
	fmt.Printf("\nWebauthnWin available: %v\n", diag.IsAvailable)
	fmt.Printf("Compile support: %v\n", diag.HasCompileSupport)
	fmt.Printf("DLL API version: %v\n", diag.WebAuthnAPIVersion)
	fmt.Printf("Has platform UV: %v\n", diag.HasPlatformUV)

	if !diag.IsAvailable {
		return nil
	}
	resp, err := webauthnwin.Diag(cf.Context, os.Stdout)
	// Abort if we got a nil diagnostic, otherwise print as much as we can.
	if resp == nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Register successful: %v\n", resp.RegisterSuccessful)
	fmt.Printf("Login successful: %v\n", resp.LoginSuccessful)
	return nil
}
