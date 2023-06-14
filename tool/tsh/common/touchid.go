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

package common

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/touchid"
)

type touchIDCommand struct {
	diag *touchIDDiagCommand
	ls   *touchIDLsCommand
	rm   *touchIDRmCommand
}

// newTouchIDCommand returns touchid subcommands.
// diag is always available.
// ls and rm may not be available depending on binary and platform limitations.
func newTouchIDCommand(app *kingpin.Application) *touchIDCommand {
	tid := app.Command("touchid", "Manage Touch ID credentials").Hidden()
	cmd := &touchIDCommand{
		diag: newTouchIDDiagCommand(tid),
	}
	if touchid.IsAvailable() {
		cmd.ls = newTouchIDLsCommand(tid)
		cmd.rm = newTouchIDRmCommand(tid)
	}
	return cmd
}

type touchIDDiagCommand struct {
	*kingpin.CmdClause
}

func newTouchIDDiagCommand(app *kingpin.CmdClause) *touchIDDiagCommand {
	return &touchIDDiagCommand{
		CmdClause: app.Command("diag", "Run Touch ID diagnostics").Hidden(),
	}
}

func (c *touchIDDiagCommand) run(cf *CLIConf) error {
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
	return nil
}

type touchIDLsCommand struct {
	*kingpin.CmdClause
}

func newTouchIDLsCommand(app *kingpin.CmdClause) *touchIDLsCommand {
	return &touchIDLsCommand{
		CmdClause: app.Command("ls", "Get a list of system Touch ID credentials").Hidden(),
	}
}

func (c *touchIDLsCommand) run(cf *CLIConf) error {
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

type touchIDRmCommand struct {
	*kingpin.CmdClause

	credentialID string
}

func newTouchIDRmCommand(app *kingpin.CmdClause) *touchIDRmCommand {
	c := &touchIDRmCommand{
		CmdClause: app.Command("rm", "Remove a Touch ID credential").Hidden(),
	}
	c.Arg("id", "ID of the Touch ID credential to remove").Required().StringVar(&c.credentialID)
	return c
}

func (c *touchIDRmCommand) FullCommand() string {
	if c.CmdClause == nil {
		return "touchid rm"
	}
	return c.CmdClause.FullCommand()
}

func (c *touchIDRmCommand) run(cf *CLIConf) error {
	if err := touchid.DeleteCredential(c.credentialID); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Touch ID credential %q removed.\n", c.credentialID)
	return nil
}
