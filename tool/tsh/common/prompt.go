/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/gravitational/teleport/api/types"
	"github.com/sirupsen/logrus"
)

func handlePromptExit() {
	rawModeOff := exec.Command("/bin/stty", "-raw", "echo")
	rawModeOff.Stdin = os.Stdin
	_ = rawModeOff.Run()
	rawModeOff.Wait()
}

func promptStdinFix() {
	rawModeOff := exec.Command("/bin/stty", "-raw", "echo")
	rawModeOff.Stdin = os.Stdin
	_ = rawModeOff.Run()
	rawModeOff.Wait()
}

var promptCommands = []prompt.Suggest{
	{Text: "db", Description: "Database access commands"},
	{Text: "status", Description: "Show status"},
	{Text: "exit", Description: "Exit this program"},
}

var promptCommandsDB = []prompt.Suggest{
	{Text: "ls", Description: "List databases"},
	{Text: "connect", Description: "Connect to a database"},
	{Text: "login", Description: "Logs in a database"},
	{Text: "logout", Description: "Logout databases"},
}

var promptCommandsDBLoginArgs = []prompt.Suggest{
	{Text: "--db-user", Description: "Database user name"},
	{Text: "--db-name", Description: "Database name"},
}

func promptCompleter(cf *CLIConf) prompt.Completer {
	return func(d prompt.Document) []prompt.Suggest {
		if d.TextBeforeCursor() == "" {
			return prompt.FilterHasPrefix(promptCommands, "", true)
		}
		args := strings.Split(d.TextBeforeCursor(), " ")

		if len(args) <= 1 {
			return prompt.FilterHasPrefix(promptCommands, args[0], true)
		}

		switch args[0] {
		case "db":
			if len(args) == 1 {
				return []prompt.Suggest{}
			}
			if len(args) == 2 {
				return prompt.FilterHasPrefix(promptCommandsDB, args[1], true)
			}

			switch args[1] {
			case "logout":
				if len(args) == 3 {
					return prompt.FilterHasPrefix(completeDBLogout(cf), args[2], true)
				}

			case "connect":
				fallthrough
			case "login":
				if len(args) == 3 {
					return prompt.FilterHasPrefix(completeDBLogin(cf), args[2], true)
				}

				wordBefore := d.GetWordBeforeCursor()
				switch {
				case args[len(args)-1] == "--db-user":
					return completeDBUsers(cf, args[2])
				case args[len(args)-2] == "--db-user":
					return prompt.FilterHasPrefix(completeDBUsers(cf, args[2]), args[len(args)-1], true)
				case strings.HasPrefix("--", wordBefore):
					return prompt.FilterHasPrefix(promptCommandsDBLoginArgs, wordBefore, true)
				}
			}
		}
		return []prompt.Suggest{}
	}
}

func completeDBUsers(cf *CLIConf, dbService string) (suggests []prompt.Suggest) {
	if promptCacheAccessChecker == nil {
		return []prompt.Suggest{}
	}

	var database types.Database
	for _, db := range promptCacheDatabases {
		if db.GetName() == dbService {
			database = db
			break
		}
	}
	if database == nil {
		return []prompt.Suggest{}
	}

	dbUsers := getDBUsers(database, promptCacheAccessChecker)
	for _, dbUser := range dbUsers.Allowed {
		suggests = append(suggests, prompt.Suggest{
			Text: dbUser,
		})
	}
	return suggests
}

func completeDBLogin(cf *CLIConf) (suggests []prompt.Suggest) {
	for _, db := range promptCacheDatabases {
		suggests = append(suggests, prompt.Suggest{
			Text:        db.GetName(),
			Description: db.GetDescription(),
		})
	}
	return suggests
}

func completeDBLogout(cf *CLIConf) []prompt.Suggest {
	tc, err := makeClient(cf)
	if err != nil {
		logrus.Debug(err)
		return []prompt.Suggest{}
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		logrus.Debug(err)
		return []prompt.Suggest{}
	}
	activeRoutes, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		logrus.Debug(err)
		return []prompt.Suggest{}
	}

	suggests := make([]prompt.Suggest, 0, len(activeRoutes))
	for _, activeRoute := range activeRoutes {
		suggests = append(suggests, prompt.Suggest{
			Text: activeRoute.ServiceName,
		})
	}
	return suggests
}

func promptRun(args []string) {
	if err := Run(context.Background(), args); err != nil {
		logrus.Debug(err)
	}
}

func promptExecutor(input string) {
	args := strings.Split(input, " ")

	switch len(args) {
	case 0:
	case 1:
		switch args[0] {
		case "status":
			promptRun(args)
		case "exit":
			fmt.Println("Bye!")
			handlePromptExit()
			os.Exit(0)
			return
		default:
			fmt.Println("bad command")
		}

	default:
		switch args[0] {
		case "db":
			switch args[1] {
			case "ls":
				promptRun(args)
			case "":
				promptRun(args)
			case "connect":
				// Note that have to use this custom branch
				// https://github.com/c-bata/go-prompt/pull/263 otherwise the
				// input to psql get hijacked and won't show up in terminal.
				promptRun(args)
			case "logout":
				promptRun(args)
			case "login":
				promptRun(args)
			default:
				fmt.Println("bad command")
			}
		default:
			fmt.Println("bad command")
		}
	}
}

func onPrompt(cf *CLIConf) error {
	p := prompt.New(
		promptExecutor,
		promptCompleter(cf),
		prompt.OptionTitle("tsh prompt: interactive tsh client"),
		prompt.OptionPrefix("[tsh-prompt] >>> "),
		prompt.OptionInputTextColor(prompt.Yellow),
	)
	p.Run()
	handlePromptExit()
	return nil
}
