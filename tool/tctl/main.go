/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"fmt"
	"github.com/gravitational/teleport/lib/utils"
	"os"
)

type CLIConfig struct {
	Debug bool
}

func main() {
	utils.InitLoggerCLI()
	app := utils.InitCmdlineParser("tctl", GlobalHelpString)

	// define global flags:
	var ccf CLIConfig
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)

	var (
		login string
	)

	// commands:
	ver := app.Command("version", "Print the version.")
	app.HelpFlag.Short('h')

	users := app.Command("users", "Manage users logins")
	userAdd := users.Command("add", "Creates a new user")
	userAdd.Alias("Using user add!")
	userAdd.Arg("login", "user login").Required().StringVar(&login)
	userList := users.Command("ls", "Lists all user logins")
	userDelete := users.Command("del", "Delete user login")
	userDelete.Arg("login", "user login to delete").Required().StringVar(&login)

	// parse CLI commands+flags:
	command, err := app.Parse(os.Args[1:])
	if err != nil {
		utils.FatalError(err)
	}

	// execute the selected command:
	switch command {
	case ver.FullCommand():
		onVersion()
	case userAdd.FullCommand():
		onUserAdd(login)
	case userList.FullCommand():
		onUserList()
	case userDelete.FullCommand():
		onUserDelete(login)
	}
}

func onVersion() {
	fmt.Println("TODO: Version command has not been implemented yet")
}

func onUserAdd(login string) {
	fmt.Println("TODO: Adding user: ", login)
}

func onUserList() {
	fmt.Println("TODO: User list is not implemented")
}

func onUserDelete(login string) {
	fmt.Println("TODO: Deleting user: ", login)
}
