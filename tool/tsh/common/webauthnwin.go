/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	wanwin "github.com/gravitational/teleport/lib/auth/webauthnwin"
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
	diag := wanwin.CheckSupport()
	fmt.Printf("\nWebauthnWin available: %v\n", diag.IsAvailable)
	fmt.Printf("Compile support: %v\n", diag.HasCompileSupport)
	fmt.Printf("DLL API version: %v\n", diag.WebAuthnAPIVersion)
	fmt.Printf("Has platform UV: %v\n", diag.HasPlatformUV)

	if !diag.IsAvailable {
		return nil
	}

	promptBefore := wanwin.PromptWriter
	defer func() { wanwin.PromptWriter = promptBefore }()
	wanwin.PromptWriter = os.Stderr

	resp, err := wanwin.Diag(cf.Context)
	// Abort if we got a nil diagnostic, otherwise print as much as we can.
	if resp == nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Register successful: %v\n", resp.RegisterSuccessful)
	fmt.Printf("Login successful: %v\n", resp.LoginSuccessful)
	return nil
}
