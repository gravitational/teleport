/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package testutils

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/alecthomas/kingpin/v2"
)

func checkKingpinHelp(help, where string) (issues []KingpinHelpIssue) {
	// Some hidden flags are empty.
	if help == "" {
		return nil
	}

	// Help should end with `.`, or `.)` in case the sentence is in brackets.
	if !strings.HasSuffix(help, ".") && !strings.HasSuffix(help, ".)") && !strings.Contains(help, ". ($") {
		issues = append(issues, KingpinHelpIssue{
			Where: where,
			Value: help,
			Issue: "help is missing period",
		})
	}

	// Help should start with upper case letter.
	if unicode.IsLower(rune(help[0])) {
		issues = append(issues, KingpinHelpIssue{
			Where: where,
			Value: help,
			Issue: "help starts with lower case letter",
		})
	}
	return
}

func checkKingpinCmdHelps(cmd *kingpin.CmdModel) (issues []KingpinHelpIssue) {
	cmdWhere := fmt.Sprintf("command %q", cmd.FullCommand)
	issues = append(issues, checkKingpinHelp(cmd.Help, cmdWhere)...)

	for _, arg := range cmd.Args {
		where := fmt.Sprintf("%s arg %q", cmdWhere, arg.Name)
		issues = append(issues, checkKingpinHelp(arg.Help, where)...)
	}
	for _, flag := range cmd.Flags {
		where := fmt.Sprintf("%s flag %q", cmdWhere, flag.Name)
		issues = append(issues, checkKingpinHelp(flag.Help, where)...)
	}
	for _, subCmd := range cmd.Commands {
		issues = append(issues, checkKingpinCmdHelps(subCmd)...)
	}
	return issues
}

type KingpinHelpIssue struct {
	Where string
	Value string
	Issue string
}

func (i KingpinHelpIssue) String() string {
	return fmt.Sprintf("%s %s: %s", i.Where, i.Issue, i.Value)
}

// FindKingpinAppHelpIssues checks common app issues like help description
// missing periods.
func FindKingpinAppHelpIssues(app *kingpin.Application) []KingpinHelpIssue {
	appModel := app.Model()
	// convert appModel to CmdModel.
	cmdProxy := &kingpin.CmdModel{
		Name:           appModel.Name,
		Help:           appModel.Help,
		FullCommand:    appModel.Name,
		ArgGroupModel:  appModel.ArgGroupModel,
		FlagGroupModel: appModel.FlagGroupModel,
		CmdGroupModel:  appModel.CmdGroupModel,
	}
	return checkKingpinCmdHelps(cmdProxy)
}
