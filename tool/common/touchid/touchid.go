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

package touchid

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/touchid"
)

// Command implements the "touchid" hidden/utility commands.
type Command struct {
	Diag *DiagCommand
	Ls   *LsCommand
	Rm   *RmCommand
}

// NewCommand returns touchid subcommands.
// Diag is always available.
// Ls and Rm may not be available depending on binary and platform limitations.
func NewCommand(app *kingpin.Application) *Command {
	tid := app.Command("touchid", "Manage Touch ID credentials").Hidden()
	cmd := &Command{
		Diag: newDiagCommand(tid),
	}
	if touchid.IsAvailable() {
		cmd.Ls = newLsCommand(tid)
		cmd.Rm = newRmCommand(tid)
	}
	return cmd
}

func (c *Command) TryRun(ctx context.Context, selectedCommand string) (match bool, err error) {
	switch {
	case c.Diag.FullCommand() == selectedCommand:
		return true, trace.Wrap(c.Diag.Run())
	case c.Ls.MatchesCommand(selectedCommand):
		return true, trace.Wrap(c.Ls.Run())
	case c.Rm.MatchesCommand(selectedCommand):
		return true, trace.Wrap(c.Rm.Run())
	default:
		return false, nil
	}
}

// DiagCommand implements the "touchid diag" command.
type DiagCommand struct {
	*kingpin.CmdClause
}

func newDiagCommand(app *kingpin.CmdClause) *DiagCommand {
	return &DiagCommand{
		CmdClause: app.Command("diag", "Run Touch ID diagnostics").Hidden(),
	}
}

// Run executes the "touchid diag" command.
func (c *DiagCommand) Run() error {
	res, err := touchid.Diag()
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Has compile support? %v\n", res.HasCompileSupport)
	fmt.Printf("Has signature? %v\n", res.HasSignature)
	fmt.Printf("Has entitlements? %v\n", res.HasEntitlements)
	fmt.Printf("Passed LAPolicy test? %v\n", res.PassedLAPolicyTest)
	fmt.Printf("Passed Secure Enclave test? %v\n", res.PassedSecureEnclaveTest)
	fmt.Printf("Touch ID enabled? %v\n", res.IsAvailable)

	if res.IsClamshellFailure() {
		fmt.Printf("\nTouch ID diagnostics failed, is your MacBook lid closed?\n")
	}

	return nil
}

// LsCommand implements the "touchid ls" command.
type LsCommand struct {
	cmd *kingpin.CmdClause
}

// MatchesCommand returns true if LsCommand matches the given fullCommand, as
// per [kingpin.CmdClause.FullCommand].
// Safe even if LsCommand is nil.
func (c *LsCommand) MatchesCommand(fullCommand string) bool {
	return c != nil && c.cmd != nil && c.cmd.FullCommand() == fullCommand
}

func newLsCommand(app *kingpin.CmdClause) *LsCommand {
	return &LsCommand{
		cmd: app.Command("ls", "Get a list of system Touch ID credentials").Hidden(),
	}
}

// Run executes the "touchid ls" command.
func (c *LsCommand) Run() error {
	if c == nil {
		return errors.New("command not available")
	}

	infos, err := touchid.ListCredentials()
	if err != nil {
		return trace.Wrap(err)
	}

	sort.Slice(infos, func(i, j int) bool {
		i1 := &infos[i]
		i2 := &infos[j]
		if cmp := strings.Compare(i1.RPID, i2.RPID); cmp != 0 {
			return cmp < 0
		}
		if cmp := strings.Compare(i1.User.Name, i2.User.Name); cmp != 0 {
			return cmp < 0
		}
		return i1.CreateTime.Before(i2.CreateTime)
	})

	t := asciitable.MakeTable([]string{"RPID", "User", "Create Time", "Credential ID"})
	for _, info := range infos {
		t.AddRow([]string{
			info.RPID,
			info.User.Name,
			info.CreateTime.Format(time.RFC3339),
			info.CredentialID,
		})
	}
	fmt.Println(t.AsBuffer().String())

	return nil
}

// RmCommand implements the "touchid rm" command.
type RmCommand struct {
	cmd          *kingpin.CmdClause
	credentialID string
}

// MatchesCommand returns true if RmCommand matches the given fullCommand, as
// per [kingpin.CmdClause.FullCommand].
// Safe even if RmCommand is nil.
func (c *RmCommand) MatchesCommand(fullCommand string) bool {
	return c != nil && c.cmd != nil && c.cmd.FullCommand() == fullCommand
}

func newRmCommand(app *kingpin.CmdClause) *RmCommand {
	c := &RmCommand{
		cmd: app.Command("rm", "Remove a Touch ID credential").Hidden(),
	}
	c.cmd.Arg("id", "ID of the Touch ID credential to remove").Required().StringVar(&c.credentialID)
	return c
}

// Run executes the "touchid rm" command.
func (c *RmCommand) Run() error {
	if c == nil {
		return errors.New("command not available")
	}

	if err := touchid.DeleteCredential(c.credentialID); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Touch ID credential %q removed.\n", c.credentialID)
	return nil
}
