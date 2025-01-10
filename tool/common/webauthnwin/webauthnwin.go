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

package webauthnwin

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	wanwin "github.com/gravitational/teleport/lib/auth/webauthnwin"
)

// Command implements the "webauthnwin" hidden/utility commands.
type Command struct {
	Diag *DiagCommand
}

// NewCommand creates a new [Command] instance.
func NewCommand(app *kingpin.Application) *Command {
	wid := app.Command("webauthnwin", "Manage Windows WebAuthn").Hidden()
	cmd := &Command{
		Diag: newDiagCommand(wid),
	}
	return cmd
}

// TryRun attempts to execute a "webauthnwin" command. Used by tctl.
func (c *Command) TryRun(ctx context.Context, selectedCommand string) (match bool, err error) {
	if c.Diag.FullCommand() == selectedCommand {
		return true, trace.Wrap(c.Diag.Run(ctx))
	}
	return false, nil
}

// DiagCommand implements the "webauthnwin diag" command.
type DiagCommand struct {
	*kingpin.CmdClause
}

func newDiagCommand(app *kingpin.CmdClause) *DiagCommand {
	return &DiagCommand{
		CmdClause: app.Command("diag", "Run windows webauthn diagnostics").Hidden(),
	}
}

func (w *DiagCommand) Run(ctx context.Context) error {
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

	resp, err := wanwin.Diag(ctx)
	// Abort if we got a nil diagnostic, otherwise print as much as we can.
	if resp == nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Register successful: %v\n", resp.RegisterSuccessful)
	fmt.Printf("Login successful: %v\n", resp.LoginSuccessful)
	return nil
}
